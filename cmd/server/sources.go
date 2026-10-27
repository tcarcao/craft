package main

// Git sources: named models kept in git that clients render by name. Each
// source is one or more repo folders merged into a single model, so a use
// case resolves against contexts and services declared anywhere in it.
// Sources are configured on the server; a client only ever names one, never
// a URL, and the credentials stay in the server's environment.

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/tcarcao/craft/v2/internal/visualizer"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// fetchEvery bounds how stale a mirror can be: a request fetches first when
// the last fetch is older than this.
const fetchEvery = 30 * time.Second

// inlineFile is the name a client's own source gets when it is rendered on
// top of a git source.
const inlineFile = "inline.craft"

// SourcesConfig is the file named by CRAFT_SOURCES.
type SourcesConfig struct {
	Sources []SourceConfig `json:"sources"`
}

type SourceConfig struct {
	Name  string       `json:"name"`
	Repos []RepoConfig `json:"repos"`
}

// RepoConfig is one folder of one repo. URL may reference environment
// variables (${GITLAB_TOKEN}), so tokens never sit in the file.
type RepoConfig struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Root   string `json:"root,omitempty"` // folder holding the .craft files; empty means the whole repo
}

type source struct {
	name  string
	repos []*repo
}

type repo struct {
	RepoConfig
	dir string

	mu      sync.Mutex
	fetched time.Time
}

// loadSources reads the config; no file means no sources.
func loadSources(configPath, cacheDir string) (map[string]*source, error) {
	sources := map[string]*source{}
	if configPath == "" {
		return sources, nil
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg SourcesConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}
	for _, sc := range cfg.Sources {
		if sc.Name == "" || len(sc.Repos) == 0 || sources[sc.Name] != nil {
			return nil, fmt.Errorf("%s: source %q needs a unique name and at least one repo", configPath, sc.Name)
		}
		src := &source{name: sc.Name}
		seen := map[string]bool{}
		for _, rc := range sc.Repos {
			if rc.Name == "" || rc.URL == "" || rc.Branch == "" || seen[rc.Name] {
				return nil, fmt.Errorf("%s: source %q: each repo needs a unique name, a url and a branch", configPath, sc.Name)
			}
			seen[rc.Name] = true
			sum := sha256.Sum256([]byte(rc.URL))
			src.repos = append(src.repos, &repo{
				RepoConfig: rc,
				dir:        filepath.Join(cacheDir, hex.EncodeToString(sum[:8])+".git"),
			})
		}
		sources[sc.Name] = src
	}
	return sources, nil
}

func git(dir string, args ...string) ([]byte, error) {
	sub := args[0]
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", sub, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// sync clones the mirror on first use and fetches when it is stale. A mirror
// clone also carries GitLab's refs/merge-requests/*, so merge requests can be
// rendered before they are merged.
func (r *repo) sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(r.fetched) < fetchEvery {
		return nil
	}
	url := os.ExpandEnv(r.URL)
	var err error
	if _, statErr := os.Stat(r.dir); statErr != nil {
		_, err = git("", "clone", "--mirror", "--quiet", url, r.dir)
	} else {
		_, err = git(r.dir, "remote", "update", "--prune")
	}
	if err != nil {
		// The error text may contain the URL; keep the token out of it.
		return fmt.Errorf("repo %s: %s", r.Name, strings.ReplaceAll(err.Error(), url, r.Name))
	}
	r.fetched = time.Now()
	return nil
}

var mergeRequestRef = regexp.MustCompile(`^!(\d+)$`)

// resolve turns a ref into a commit: empty is the configured branch, !12 is
// GitLab merge request 12, anything else a branch, tag or commit.
func (r *repo) resolve(ref string) (string, error) {
	if ref == "" {
		ref = r.Branch
	}
	if strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("repo %s: invalid ref %q", r.Name, ref)
	}
	candidates := []string{"refs/heads/" + ref, ref}
	if m := mergeRequestRef.FindStringSubmatch(ref); m != nil {
		candidates = []string{"refs/merge-requests/" + m[1] + "/head"}
	}
	for _, c := range candidates {
		if out, err := git(r.dir, "rev-parse", "--verify", "--quiet", c+"^{commit}"); err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", fmt.Errorf("repo %s: ref %q not found", r.Name, ref)
}

// files returns the .craft files under the root at commit, keyed by
// repo-name/path, plus the root's tree id: it changes only when something
// under the root changes, so it identifies the model, not the commit.
func (r *repo) files(commit string) (map[string]string, string, error) {
	treeSpec := commit + "^{tree}"
	if r.Root != "" {
		treeSpec = commit + ":" + r.Root
	}
	tree, err := git(r.dir, "rev-parse", "--verify", "--quiet", treeSpec)
	if err != nil {
		return nil, "", fmt.Errorf("repo %s: folder %q not found at %.8s", r.Name, r.Root, commit)
	}
	args := []string{"archive", "--format=tar", commit}
	if r.Root != "" {
		args = append(args, "--", r.Root)
	}
	archive, err := git(r.dir, args...)
	if err != nil {
		return nil, "", err
	}
	files := map[string]string{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if h.Typeflag != tar.TypeReg || !strings.HasSuffix(h.Name, ".craft") {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, "", err
		}
		files[path.Join(r.Name, h.Name)] = string(data)
	}
	return files, strings.TrimSpace(string(tree)), nil
}

// snapshot is a source's model at specific commits.
type snapshot struct {
	files     map[string]string
	commits   map[string]string // repo name -> commit
	contentID string
}

// load fetches every repo of the source and reads its files at the requested
// refs (repo name -> ref; a missing entry means the configured branch).
func (s *source) load(refs map[string]string) (*snapshot, error) {
	for name := range refs {
		if !s.has(name) {
			return nil, fmt.Errorf("source %s has no repo %q", s.name, name)
		}
	}
	snap := &snapshot{files: map[string]string{}, commits: map[string]string{}}
	id := sha256.New()
	for _, r := range s.repos {
		if err := r.sync(); err != nil {
			return nil, err
		}
		commit, err := r.resolve(refs[r.Name])
		if err != nil {
			return nil, err
		}
		files, tree, err := r.files(commit)
		if err != nil {
			return nil, err
		}
		for k, v := range files {
			snap.files[k] = v
		}
		snap.commits[r.Name] = commit
		fmt.Fprintf(id, "%s %s\n", r.Name, tree)
	}
	snap.contentID = hex.EncodeToString(id.Sum(nil))[:16]
	return snap, nil
}

func (s *source) has(repoName string) bool {
	for _, r := range s.repos {
		if r.Name == repoName {
			return true
		}
	}
	return false
}

// SourcesResponse lists the configured sources.
type SourcesResponse struct {
	Sources []SourceInfo `json:"sources"`
}

type SourceInfo struct {
	Name  string     `json:"name"`
	Repos []RepoInfo `json:"repos"`
}

type RepoInfo struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

func (s *Server) handleSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := SourcesResponse{Sources: []SourceInfo{}}
		for _, src := range s.sources {
			info := SourceInfo{Name: src.name}
			for _, rp := range src.repos {
				info.Repos = append(info.Repos, RepoInfo{Name: rp.Name, Branch: rp.Branch})
			}
			resp.Sources = append(resp.Sources, info)
		}
		sort.Slice(resp.Sources, func(i, j int) bool { return resp.Sources[i].Name < resp.Sources[j].Name })
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// CatalogRequest names the refs to read; empty means every configured branch.
type CatalogRequest struct {
	Refs map[string]string `json:"refs,omitempty"`
}

// CatalogResponse is what a client can pick from at those refs.
type CatalogResponse struct {
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	ContentID string            `json:"contentId"`
	Commits   map[string]string `json:"commits"`
	UseCases  []CatalogUseCase  `json:"useCases"`
	Journeys  []string          `json:"journeys"`
	Files     []string          `json:"files"`
}

type CatalogUseCase struct {
	Name string `json:"name"`
	File string `json:"file"`
}

func (s *Server) sourceFor(w http.ResponseWriter, r *http.Request) (*source, bool) {
	src := s.sources[mux.Vars(r)["name"]]
	if src == nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("no source named %q", mux.Vars(r)["name"]))
		return nil, false
	}
	return src, true
}

func (s *Server) handleCatalog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		src, ok := s.sourceFor(w, r)
		if !ok {
			return
		}
		var req CatalogRequest
		if r.ContentLength != 0 && !decode(w, r, &req) {
			return
		}
		snap, err := src.load(req.Refs)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		doc, _ := parseFiles(snap.files)
		resp := CatalogResponse{
			Success: true, ContentID: snap.contentID, Commits: snap.commits,
			UseCases: []CatalogUseCase{}, Journeys: craft.JourneySlugs(doc), Files: []string{},
		}
		if resp.Journeys == nil {
			resp.Journeys = []string{}
		}
		for _, uc := range doc.UseCases {
			resp.UseCases = append(resp.UseCases, CatalogUseCase{Name: uc.Name, File: uc.SourceURI})
		}
		for f := range snap.files {
			resp.Files = append(resp.Files, f)
		}
		sort.Strings(resp.Files)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// SourceRenderRequest draws a view of a source. DSL, when set, is merged in
// as one more file, so a draft use case renders against the real contexts.
// What is shown: UseCases, else the use cases of Path, else (with DSL) the
// draft's own use cases, else everything. C4 always shows everything.
type SourceRenderRequest struct {
	Refs     map[string]string `json:"refs,omitempty"`
	DSL      string            `json:"dsl,omitempty"`
	View     string            `json:"view"` // c4, c4-flat, architecture, flow, sequence, journey, board
	UseCases []string          `json:"useCases,omitempty"`
	Path     string            `json:"path,omitempty"`
	Journey  string            `json:"journey,omitempty"`
	Format   string            `json:"format,omitempty"` // png, svg, pdf, puml
}

type SourceRenderResponse struct {
	PreviewResponse
	ContentID string            `json:"contentId"`
	Commits   map[string]string `json:"commits"`
}

var sourceViews = map[string]visualizer.View{
	"c4": visualizer.ViewC4, "c4-flat": visualizer.ViewC4,
	"architecture": visualizer.ViewArchitecture, "flow": visualizer.ViewFlow,
	"sequence": visualizer.ViewSequence, "journey": visualizer.ViewSequence,
}

func (s *Server) handleSourceRender() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		src, ok := s.sourceFor(w, r)
		if !ok {
			return
		}
		var req SourceRenderRequest
		if !decode(w, r, &req) {
			return
		}
		if req.View == "board" {
			s.renderSourceBoard(w, src, req)
			return
		}
		view, known := sourceViews[req.View]
		if !known {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("unknown view %q", req.View))
			return
		}
		snap, err := src.load(req.Refs)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.DSL != "" {
			snap.files[inlineFile] = req.DSL
		}
		doc, diags := parseFiles(snap.files)

		vr := visualizer.Request{View: view, Backend: visualizer.PlantUML, Format: plantUMLFormat(req.Format)}
		switch {
		case view == visualizer.ViewC4:
			mode := ""
			if req.View == "c4-flat" {
				mode = "transparent"
			}
			vr.C4 = c4Options(mode, nil, nil)
		case req.View == "journey":
			vr.Journey = req.Journey
		default:
			vr.UseCases = req.UseCases
			if len(vr.UseCases) == 0 {
				vr.UseCases = useCasesIn(doc, req.Path, req.DSL != "")
			}
		}
		out, ok := s.render(w, doc, vr)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(SourceRenderResponse{
			PreviewResponse: PreviewResponse{Success: true, Data: base64.StdEncoding.EncodeToString(out.Data), Diagnostics: diags},
			ContentID:       snap.contentID,
			Commits:         snap.commits,
		})
	}
}

// renderSourceBoard answers a "board" render: one use case of the flow View
// as SVG, base64 like every source render.
func (s *Server) renderSourceBoard(w http.ResponseWriter, src *source, req SourceRenderRequest) {
	if req.Format != "" && req.Format != "svg" {
		respondWithError(w, http.StatusBadRequest, "the board writes svg only")
		return
	}
	if len(req.UseCases) > 1 {
		respondWithError(w, http.StatusBadRequest, "a board draws one use case")
		return
	}
	snap, err := src.load(req.Refs)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DSL != "" {
		snap.files[inlineFile] = req.DSL
	}
	doc, diags := parseFiles(snap.files)
	want := ""
	if len(req.UseCases) == 1 {
		want = req.UseCases[0]
	}
	name, err := boardUseCase(doc, want, useCasesIn(doc, req.Path, req.DSL != ""))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	out, ok := s.render(w, doc, visualizer.Request{
		View: visualizer.ViewFlow, Backend: visualizer.Board, Format: visualizer.FormatSVG, UseCases: []string{name},
	})
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SourceRenderResponse{
		PreviewResponse: PreviewResponse{Success: true, Data: base64.StdEncoding.EncodeToString(out.Data), Diagnostics: diags},
		ContentID:       snap.contentID,
		Commits:         snap.commits,
	})
}

// useCasesIn names the use cases declared in file path, or in the inline
// file when draft is set. nil means no filter.
func useCasesIn(doc *craft.CraftDoc, path string, draft bool) []string {
	if path == "" && draft {
		path = inlineFile
	}
	if path == "" {
		return nil
	}
	names := []string{}
	for _, uc := range doc.UseCases {
		if uc.SourceURI == path {
			names = append(names, uc.Name)
		}
	}
	if len(names) == 0 {
		// A file with no use cases (contexts only) shows the whole model.
		return nil
	}
	return names
}
