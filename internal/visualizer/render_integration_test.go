//go:build integration

// Integration tests that render generated PUML against a real PlantUML server
// in a testcontainer. Guards against stdlib drift (e.g. tupadr3 paths that
// disappear or move between PlantUML versions) and generator-side regressions
// that emit unknown sprites.
//
// Run:
//
//	go get github.com/testcontainers/testcontainers-go      # first time only
//	make test-integration
//	# or:  go test -tags=integration ./internal/visualizer/...
//
// Requires a working Docker / Podman daemon at test time.
package visualizer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/internal/visualizer"
	craft "github.com/tcarcao/craft/pkg/craft"
)

// PlantUML renders error messages into the response body (as visible SVG text)
// while still returning HTTP 200 on some paths — detect those markers
// explicitly so silent regressions can't pass.
var renderErrorMarkers = []string{
	"cannot include",
	"Syntax Error",
}

// Pinned image. Bump intentionally — every bump is a chance to catch stdlib
// drift before users do.
const plantumlImage = "plantuml/plantuml-server:jetty"

// Single PlantUML container is shared by every test in this file. Spinning one
// per sub-test is wasteful and flaky under podman.
var (
	plantumlEndpoint string
	plantumlStartErr error
	plantumlOnce     sync.Once
	plantumlTerm     func()
)

func TestMain(m *testing.M) {
	code := m.Run()
	if plantumlTerm != nil {
		plantumlTerm()
	}
	os.Exit(code)
}

// plantUMLEndpoint lazily starts the shared PlantUML container and returns its
// HTTP base URL. Skips the calling test if the container can't be started
// (e.g. no Docker socket available) so this file fails closed only on real
// regressions.
func plantUMLEndpoint(t *testing.T) string {
	t.Helper()
	plantumlOnce.Do(func() {
		ctx := context.Background()
		req := testcontainers.ContainerRequest{
			Image:        plantumlImage,
			ExposedPorts: []string{"8080/tcp"},
			WaitingFor: wait.ForHTTP("/").WithPort("8080/tcp").
				WithStatusCodeMatcher(func(c int) bool { return c == 200 || c == 302 }).
				WithStartupTimeout(90 * time.Second),
		}
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			plantumlStartErr = err
			return
		}
		host, err := c.Host(ctx)
		if err != nil {
			plantumlStartErr = err
			_ = c.Terminate(ctx)
			return
		}
		port, err := c.MappedPort(ctx, "8080")
		if err != nil {
			plantumlStartErr = err
			_ = c.Terminate(ctx)
			return
		}
		plantumlEndpoint = fmt.Sprintf("http://%s:%s", host, port.Port())
		plantumlTerm = func() {
			tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := c.Terminate(tctx); err != nil {
				log.Printf("terminate plantuml container: %v", err)
			}
		}
	})
	if plantumlStartErr != nil {
		t.Skipf("PlantUML container unavailable: %v", plantumlStartErr)
	}
	return plantumlEndpoint
}

func assertRendersClean(t *testing.T, endpoint, puml string) {
	t.Helper()
	resp, err := http.Post(endpoint+"/svg", "text/plain", strings.NewReader(puml))
	if err != nil {
		t.Fatalf("POST /svg: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d from PlantUML\n--- snippet ---\n%s", resp.StatusCode, snippet(body))
	}
	for _, m := range renderErrorMarkers {
		if bytes.Contains(body, []byte(m)) {
			t.Fatalf("response contains error marker %q\n--- snippet ---\n%s", m, snippet(body))
		}
	}
}

func snippet(b []byte) string {
	const n = 800
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// minimalC4Model builds a CraftDoc that produces a valid C4 diagram with one
// service in the requested language. Sufficient to exercise sprite includes.
func minimalC4Model(language string) *craft.CraftDoc {
	return &craft.CraftDoc{
		Services: []craft.Service{{
			Name:     "TestService",
			Contexts: []string{"TestContext"},
			Language: language,
		}},
	}
}

func generateC4PUML(t *testing.T, doc *craft.CraftDoc) string {
	t.Helper()
	viz := visualizer.New()
	puml, _, err := viz.GenerateC4WithFormat(doc, visualizer.C4ModeBoundaries, false, visualizer.FormatPUML)
	if err != nil {
		t.Fatalf("GenerateC4WithFormat: %v", err)
	}
	return string(puml)
}

// TestC4Render_PerLanguage exercises every language getServiceIcon recognises.
// Failure means either the generated PUML emits an include path the strict
// renderer can't resolve, or the sprite referenced via $sprite="..." has no
// matching include.
func TestC4Render_PerLanguage(t *testing.T) {
	endpoint := plantUMLEndpoint(t)

	cases := []string{
		"go", "golang",
		"java",
		"python",
		"nodejs", "node",
		"javascript", "js",
		"typescript", "ts",
		"rust",
		"csharp", "c#", "dotnet",
		"php",
		"ruby",
		"kotlin",
		"swift",
	}
	for _, lang := range cases {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			assertRendersClean(t, endpoint, generateC4PUML(t, minimalC4Model(lang)))
		})
	}
}

// TestC4Render_NoLanguage covers the case where no service declares a
// language — the generator must not emit any language-sprite include.
func TestC4Render_NoLanguage(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	puml := generateC4PUML(t, minimalC4Model(""))
	for path := range languageIncludePathsForTest() {
		if strings.Contains(puml, "<"+path+">") {
			t.Fatalf("unexpected language include %q emitted when no service declares a language\n--- puml ---\n%s", path, puml)
		}
	}
	assertRendersClean(t, endpoint, puml)
}

// TestC4Render_Regression_RustInclude guards the specific bug fixed in this
// change: the generator emitted `tupadr3/devicons2/rust`, a path missing from
// every released PlantUML stdlib. The fix routes rust to `devicons/rust`.
func TestC4Render_Regression_RustInclude(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	puml := generateC4PUML(t, minimalC4Model("rust"))
	if strings.Contains(puml, "<tupadr3/devicons2/rust>") {
		t.Fatalf("regression: generator emitted tupadr3/devicons2/rust (missing from PlantUML stdlib)")
	}
	assertRendersClean(t, endpoint, puml)
}

// languageIncludePathsForTest mirrors the in-package map. Duplicated here to
// keep the test in the `_test` package without exporting internals.
func languageIncludePathsForTest() map[string]string {
	return map[string]string{
		"tupadr3/devicons2/go":         "go",
		"tupadr3/devicons2/java":       "java",
		"tupadr3/devicons2/python":     "python",
		"tupadr3/devicons2/nodejs":     "nodejs",
		"tupadr3/devicons2/javascript": "javascript",
		"tupadr3/devicons/rust":        "rust",
		"tupadr3/devicons2/dot_net":    "dot_net",
		"tupadr3/devicons2/php":        "php",
		"tupadr3/devicons2/ruby":       "ruby",
		"tupadr3/devicons2/kotlin":     "kotlin",
		"tupadr3/devicons2/swift":      "swift",
	}
}

// TestVAS_AllDiagramsRender renders the full vas.craft fixture through every
// diagram generator and asserts the output renders cleanly on the strict
// PlantUML server. End-to-end smoke test for the diagram-quality fixes.
func TestVAS_AllDiagramsRender(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	viz := visualizer.New()
	type variant struct {
		name string
		gen  func() ([]byte, error)
	}
	variants := []variant{
		{"c4", func() ([]byte, error) {
			b, _, err := viz.GenerateC4WithFormat(doc, visualizer.C4ModeBoundaries, false, visualizer.FormatPUML)
			return b, err
		}},
		{"domain-detailed", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(doc, visualizer.DomainModeDetailed, visualizer.FormatPUML)
			return b, err
		}},
		{"domain-architecture", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(doc, visualizer.DomainModeArchitecture, visualizer.FormatPUML)
			return b, err
		}},
		{"sequence", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithTypeAndModeAndFormat(doc, visualizer.DiagramTypeSequence, visualizer.DomainModeDetailed, visualizer.FormatPUML)
			return b, err
		}},
	}
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			puml, err := v.gen()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			assertRendersClean(t, endpoint, string(puml))
		})
	}
}

// TestVAS_SplitRenders proves that a single split-mode file produced by
// filtering vas.craft to one em-dash-bearing use case still renders cleanly
// through PlantUML. End-to-end check that Slugify, FilterUseCases, and
// Unicode handling cooperate.
func TestVAS_SplitRenders(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	filtered, missing := visualizer.FilterUseCases(doc, []string{"one-time-vas-fulfillment-through-provider-apply"})
	if len(missing) != 0 {
		t.Fatalf("expected to match the em-dash use case, missing: %v", missing)
	}
	if len(filtered.UseCases) != 1 {
		t.Fatalf("expected exactly one matched use case, got %d", len(filtered.UseCases))
	}

	puml, _, err := visualizer.New().GenerateDomainDiagramWithModeAndFormat(filtered, visualizer.DomainModeDetailed, visualizer.FormatPUML)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Verify the use case content made it through.
	if !strings.Contains(string(puml), "fulfillment") {
		t.Fatalf("expected em-dash use case content in PUML, got: %s", puml)
	}
	assertRendersClean(t, endpoint, string(puml))
}

// TestVAS_MermaidSequenceRenders proves the Mermaid sequence generator
// produces source the actual mermaid-cli renderer accepts. End-to-end check
// for the new --format mermaid path.
//
// We feed the input via Files (host->container copy) and read the SVG back
// with CopyFileFromContainer instead of a bind mount — bind mounts under
// macOS+podman hit uid/perm issues that have nothing to do with what we're
// actually verifying here.
func TestVAS_MermaidSequenceRenders(t *testing.T) {
	ctx := context.Background()

	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	mermaidSrc, err := visualizer.New().GenerateSequenceDiagramMermaid(doc, visualizer.DomainModeDetailed)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: "minlag/mermaid-cli:latest",
		// /tmp is world-writable so mmdc (non-root) can write output.svg there.
		Cmd: []string{"-i", "/tmp/input.mmd", "-o", "/tmp/output.svg"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: inPath, ContainerFilePath: "/tmp/input.mmd", FileMode: 0o644},
		},
		WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("mermaid-cli container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.ExitCode != 0 {
		logs, _ := c.Logs(ctx)
		var logBuf []byte
		if logs != nil {
			logBuf, _ = io.ReadAll(logs)
			_ = logs.Close()
		}
		t.Fatalf("mmdc exited non-zero (%d):\n%s", state.ExitCode, snippet(logBuf))
	}

	rc, err := c.CopyFileFromContainer(ctx, "/tmp/output.svg")
	if err != nil {
		t.Fatalf("copy output: %v", err)
	}
	defer rc.Close()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("mmdc produced empty SVG")
	}
}

// TestVAS_MermaidDomainDetailedRenders verifies the detailed-domain Mermaid
// output renders cleanly via mmdc. Existing tests only covered sequence,
// which is how the trigger-edge and actor-filtering bugs initially shipped.
func TestVAS_MermaidDomainDetailedRenders(t *testing.T) {
	ctx := context.Background()

	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	mermaidSrc, err := visualizer.New().GenerateDomainDiagramMermaid(doc, visualizer.DomainModeDetailed)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: "minlag/mermaid-cli:latest",
		Cmd:   []string{"-i", "/tmp/input.mmd", "-o", "/tmp/output.svg"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: inPath, ContainerFilePath: "/tmp/input.mmd", FileMode: 0o644},
		},
		WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("mermaid-cli container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.ExitCode != 0 {
		logs, _ := c.Logs(ctx)
		var logBuf []byte
		if logs != nil {
			logBuf, _ = io.ReadAll(logs)
			_ = logs.Close()
		}
		t.Fatalf("mmdc exited non-zero (%d):\n%s", state.ExitCode, snippet(logBuf))
	}

	rc, err := c.CopyFileFromContainer(ctx, "/tmp/output.svg")
	if err != nil {
		t.Fatalf("copy output: %v", err)
	}
	defer rc.Close()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("mmdc produced empty SVG")
	}
}

// TestVAS_MermaidDomainArchitectureRenders covers the architecture-mode
// variant of the domain generator. Same harness; different generator call.
func TestVAS_MermaidDomainArchitectureRenders(t *testing.T) {
	ctx := context.Background()

	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	mermaidSrc, err := visualizer.New().GenerateDomainDiagramMermaid(doc, visualizer.DomainModeArchitecture)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: "minlag/mermaid-cli:latest",
		Cmd:   []string{"-i", "/tmp/input.mmd", "-o", "/tmp/output.svg"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: inPath, ContainerFilePath: "/tmp/input.mmd", FileMode: 0o644},
		},
		WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("mermaid-cli container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.ExitCode != 0 {
		logs, _ := c.Logs(ctx)
		var logBuf []byte
		if logs != nil {
			logBuf, _ = io.ReadAll(logs)
			_ = logs.Close()
		}
		t.Fatalf("mmdc exited non-zero (%d):\n%s", state.ExitCode, snippet(logBuf))
	}

	rc, err := c.CopyFileFromContainer(ctx, "/tmp/output.svg")
	if err != nil {
		t.Fatalf("copy output: %v", err)
	}
	defer rc.Close()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("mmdc produced empty SVG")
	}
}

// TestVAS_MermaidC4Renders covers the C4 generator. Mermaid's c4Diagram
// is experimental; this confirms our output passes the live parser.
func TestVAS_MermaidC4Renders(t *testing.T) {
	ctx := context.Background()

	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	greenRoot, li, _ := syntax.Parse(string(src))
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	mermaidSrc, err := visualizer.New().GenerateC4Mermaid(doc, visualizer.C4ModeBoundaries, false)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: "minlag/mermaid-cli:latest",
		Cmd:   []string{"-i", "/tmp/input.mmd", "-o", "/tmp/output.svg"},
		Files: []testcontainers.ContainerFile{
			{HostFilePath: inPath, ContainerFilePath: "/tmp/input.mmd", FileMode: 0o644},
		},
		WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("mermaid-cli container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.ExitCode != 0 {
		logs, _ := c.Logs(ctx)
		var logBuf []byte
		if logs != nil {
			logBuf, _ = io.ReadAll(logs)
			_ = logs.Close()
		}
		t.Fatalf("mmdc exited non-zero (%d):\n%s", state.ExitCode, snippet(logBuf))
	}

	rc, err := c.CopyFileFromContainer(ctx, "/tmp/output.svg")
	if err != nil {
		t.Fatalf("copy output: %v", err)
	}
	defer rc.Close()
	out, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("mmdc produced empty SVG")
	}
}
