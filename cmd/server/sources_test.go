package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

const contextsFile = `domain Identity {
  Registration
  Profile
}

domain Messaging {
  Notifications
}

services {
  Accounts { contexts: Registration, Profile }
  Mailer { contexts: Notifications }
}
`

const useCaseFile = `use_case "Sign Up" {
  when Visitor submits form
    Registration asks Profile to create profile
    Registration notifies "User Registered"
  when Notifications listens "User Registered"
    Notifications sends welcome email
}
`

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sourceServer serves one source backed by a local repo whose model is split
// over two files, with a merge request ref that adds a use case.
func sourceServer(t *testing.T) http.Handler {
	t.Helper()
	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	run(t, tmp, "init", "--quiet", "-b", "main", work)
	write(t, filepath.Join(work, "model/contexts.craft"), contextsFile)
	write(t, filepath.Join(work, "model/signup.craft"), useCaseFile)
	write(t, filepath.Join(work, "README.md"), "x")
	run(t, work, "add", ".")
	run(t, work, "commit", "--quiet", "-m", "model")
	run(t, work, "checkout", "--quiet", "-b", "draft")
	write(t, filepath.Join(work, "model/delete.craft"), `use_case "Delete Account" {
  when Visitor deletes account
    Profile removes profile
}
`)
	run(t, work, "add", ".")
	run(t, work, "commit", "--quiet", "-m", "draft")
	// What GitLab advertises for merge request 7.
	run(t, work, "update-ref", "refs/merge-requests/7/head", "draft")
	run(t, work, "checkout", "--quiet", "main")

	cfg := filepath.Join(tmp, "sources.json")
	write(t, cfg, `{"sources":[{"name":"accounts","repos":[{"name":"model","url":"${CRAFT_TEST_REPO}","branch":"main","root":"model"}]}]}`)
	t.Setenv("CRAFT_TEST_REPO", work)
	sources, err := loadSources(cfg, filepath.Join(tmp, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	s.sources = sources
	r := mux.NewRouter()
	r.HandleFunc("/sources/{name}/catalog", s.handleCatalog()).Methods("POST")
	r.HandleFunc("/sources/{name}/render", s.handleSourceRender()).Methods("POST")
	return r
}

func post[T any](t *testing.T, h http.Handler, url, body string) (int, T) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, url, strings.NewReader(body)))
	var resp T
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: decode %q: %v", url, rr.Body.String(), err)
	}
	return rr.Code, resp
}

func TestSources_CatalogListsUseCasesAcrossFiles(t *testing.T) {
	h := sourceServer(t)
	_, main := post[CatalogResponse](t, h, "/sources/accounts/catalog", `{}`)
	if !main.Success || len(main.UseCases) != 1 || main.UseCases[0].File != "model/model/signup.craft" {
		t.Fatalf("main catalog = %+v", main)
	}
	_, mr := post[CatalogResponse](t, h, "/sources/accounts/catalog", `{"refs":{"model":"!7"}}`)
	if len(mr.UseCases) != 2 {
		t.Fatalf("merge request catalog = %+v", mr.UseCases)
	}
	if mr.ContentID == main.ContentID || mr.Commits["model"] == main.Commits["model"] {
		t.Errorf("merge request should differ from main: %+v vs %+v", mr, main)
	}
}

// A use case drawn from git resolves the contexts declared in another file.
func TestSources_RenderUseCaseResolvesOtherFiles(t *testing.T) {
	h := sourceServer(t)
	code, resp := post[SourceRenderResponse](t, h, "/sources/accounts/render",
		`{"view":"sequence","useCases":["Sign Up"],"format":"puml"}`)
	if code != http.StatusOK || !resp.Success {
		t.Fatalf("render: %d %+v", code, resp)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("unexpected error: %+v", d)
		}
	}
	puml, _ := base64.StdEncoding.DecodeString(resp.Data)
	if !strings.Contains(string(puml), "Notifications") {
		t.Errorf("diagram misses the Notifications context:\n%s", puml)
	}
}

// A draft use case posted inline renders against the repo's contexts, and
// only the draft is shown.
func TestSources_RenderInlineDraftOnTopOfSource(t *testing.T) {
	h := sourceServer(t)
	body, _ := json.Marshal(SourceRenderRequest{View: "sequence", Format: "puml", DSL: `use_case "Change Email" {
  when Visitor changes email
    Profile asks Registration to verify email
}
`})
	code, resp := post[SourceRenderResponse](t, h, "/sources/accounts/render", string(body))
	if code != http.StatusOK || !resp.Success {
		t.Fatalf("render: %d %+v", code, resp)
	}
	for _, d := range resp.Diagnostics {
		if d.SourceURI == inlineFile && d.Severity == "error" {
			t.Errorf("draft should resolve against the repo: %+v", d)
		}
	}
	puml, _ := base64.StdEncoding.DecodeString(resp.Data)
	if !strings.Contains(string(puml), "verify email") || strings.Contains(string(puml), "welcome email") {
		t.Errorf("expected only the draft use case:\n%s", puml)
	}
}

func TestSources_UnknownRefIsAnError(t *testing.T) {
	h := sourceServer(t)
	code, resp := post[CatalogResponse](t, h, "/sources/accounts/catalog", `{"refs":{"model":"--upload-pack=x"}}`)
	if code != http.StatusBadRequest || resp.Success {
		t.Fatalf("got %d %+v", code, resp)
	}
}

func TestSources_RenderBoardOneUseCase(t *testing.T) {
	code, resp := post[SourceRenderResponse](t, sourceServer(t), "/sources/accounts/render", `{"view":"board","useCases":["Sign Up"],"format":"svg"}`)
	svg, _ := base64.StdEncoding.DecodeString(resp.Data)
	if code != http.StatusOK || !strings.HasPrefix(string(svg), "<svg") {
		t.Fatalf("code=%d data=%.80s", code, svg)
	}
}

func TestSources_RenderBoardRejects(t *testing.T) {
	h := sourceServer(t)
	cases := map[string]struct{ body, want string }{
		"two use cases": {`{"view":"board","useCases":["Sign Up","Other"]}`, "a board draws one use case"},
		"format":        {`{"view":"board","useCases":["Sign Up"],"format":"png"}`, "svg only"},
		"unknown":       {`{"view":"board","useCases":["nope"]}`, "available:"},
	}
	for name, c := range cases {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/sources/accounts/render", strings.NewReader(c.body)))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), c.want) {
			t.Errorf("%s: code=%d body=%s, want 400 containing %q", name, rr.Code, rr.Body.String(), c.want)
		}
	}
}

func TestSources_RenderBoardDraftDefaultsToDraftUseCase(t *testing.T) {
	dsl := "use_case \"Draft Flow\" {\n  when Visitor submits form\n    Registration asks Profile to create profile\n}\n"
	body, _ := json.Marshal(map[string]string{"view": "board", "dsl": dsl})
	code, resp := post[SourceRenderResponse](t, sourceServer(t), "/sources/accounts/render", string(body))
	svg, _ := base64.StdEncoding.DecodeString(resp.Data)
	if code != http.StatusOK || !strings.Contains(string(svg), "Draft Flow") {
		t.Fatalf("code=%d data=%.80s", code, svg)
	}
}
