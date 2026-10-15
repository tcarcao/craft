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
//	make test-integration RUN=JourneySemantics   # one group of checks
//	# or:  go test -tags=integration ./internal/visualizer/...
//
// With podman on macOS, use the make target: it disables Ryuk and points
// DOCKER_HOST at the podman machine socket, which testcontainers needs.
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

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/visualizer"
	"github.com/tcarcao/craft/v2/internal/visualizer/plantuml"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
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

// mermaidCLIImage is the mermaid-cli (mmdc) image the Mermaid render checks
// run. Pinned for the same reason as plantumlImage: a parser change upstream
// should fail a deliberate bump, not a random run.
const mermaidCLIImage = "minlag/mermaid-cli:12.0.0"

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
	puml, _, err := renderPUML(doc, visualizer.ViewC4)
	if err != nil {
		t.Fatalf("GenerateC4WithFormat: %v", err)
	}
	return string(puml)
}

// TestC4Render_PerLanguage exercises every language the C4 view recognises.
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
	for _, path := range plantuml.LanguageIncludes() {
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

// TestVAS_AllDiagramsRender renders the full vas.craft fixture through every
// diagram generator and asserts the output renders cleanly on the strict
// PlantUML server. End-to-end smoke test for the diagram-quality fixes.
func TestVAS_AllDiagramsRender(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc
	type variant struct {
		name string
		gen  func() ([]byte, error)
	}
	variants := []variant{
		{"c4", func() ([]byte, error) {
			b, _, err := renderPUML(doc, visualizer.ViewC4)
			return b, err
		}},
		{"domain-detailed", func() ([]byte, error) {
			b, _, err := renderPUML(doc, visualizer.ViewFlow)
			return b, err
		}},
		{"domain-architecture", func() ([]byte, error) {
			b, _, err := renderPUML(doc, visualizer.ViewArchitecture)
			return b, err
		}},
		{"sequence", func() ([]byte, error) {
			b, _, err := renderPUML(doc, visualizer.ViewSequence)
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
// through PlantUML. End-to-end check that Slugify, use-case filtering, and
// Unicode handling cooperate.
func TestVAS_SplitRenders(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	src, err := os.ReadFile("testdata/vas.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc

	out, err := visualizer.New(nil).Render(doc, visualizer.Request{
		View: visualizer.ViewFlow, Backend: visualizer.PlantUML, Format: visualizer.FormatSource,
		UseCases: []string{"one-time-vas-fulfillment-through-provider-apply"},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	puml := out.Data
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
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc

	mermaidSrc, err := renderMermaid(doc, visualizer.ViewSequence)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: mermaidCLIImage,
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
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc

	mermaidSrc, err := renderMermaid(doc, visualizer.ViewFlow)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: mermaidCLIImage,
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
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc

	mermaidSrc, err := renderMermaid(doc, visualizer.ViewArchitecture)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: mermaidCLIImage,
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
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("vas.craft", string(src))}).Doc

	mermaidSrc, err := renderMermaid(doc, visualizer.ViewC4)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "input.mmd")
	if err := os.WriteFile(inPath, []byte(mermaidSrc), 0644); err != nil {
		t.Fatal(err)
	}

	req := testcontainers.ContainerRequest{
		Image: mermaidCLIImage,
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

// renderPUML and renderMermaid draw diagram source through Render. A C4
// diagram uses boundaries mode without databases.
func renderPUML(doc *craft.CraftDoc, v visualizer.View) ([]byte, string, error) {
	return renderSource(doc, v, visualizer.PlantUML)
}

func renderMermaid(doc *craft.CraftDoc, v visualizer.View) (string, error) {
	b, _, err := renderSource(doc, v, visualizer.Mermaid)
	return string(b), err
}

func renderSource(doc *craft.CraftDoc, v visualizer.View, backend visualizer.Backend) ([]byte, string, error) {
	req := visualizer.Request{View: v, Backend: backend, Format: visualizer.FormatSource}
	if v == visualizer.ViewC4 {
		req.C4 = &visualizer.C4Options{Boundaries: true}
	}
	out, err := visualizer.New(nil).Render(doc, req)
	return out.Data, out.ContentType, err
}

// journeySemanticsDoc analyses the journey-semantics fixture: par/and/loop,
// notes and findings, a receives trigger (found and lost messages) and a store
// hand-off with a scheduled listens trigger.
func journeySemanticsDoc(t *testing.T) *craft.CraftDoc {
	t.Helper()
	src, err := os.ReadFile("testdata/journey-semantics.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return analysis.Workspace([]analysis.FileResult{analysis.File("journey-semantics.craft", string(src))}).Doc
}

// TestJourneySemantics_MermaidSequenceRenders proves mermaid-cli accepts the
// new sequence constructs, above all the bare `and` that separates the
// branches of a fan-out `par` (spec §4.5), and the `Unobserved` participant
// that stands in for found and lost messages. Flow, architecture and C4 are
// rendered too, for the store node and the unobserved-caller node.
func TestJourneySemantics_MermaidSequenceRenders(t *testing.T) {
	doc := journeySemanticsDoc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			src, err := renderMermaid(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if v == visualizer.ViewSequence && !strings.Contains(src, "\n    and\n") {
				t.Fatalf("expected a bare `and` in the sequence source:\n%s", src)
			}
			assertMermaidRenders(t, src)
		})
	}
}

// TestJourneySemantics_PlantUMLRenders renders every view of the
// journey-semantics fixture on the strict PlantUML server: par/loop frames,
// coloured finding notes, `[->`/`-->]` found and lost messages, and the
// `database` participant of a store hand-off.
func TestJourneySemantics_PlantUMLRenders(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	doc := journeySemanticsDoc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			puml, _, err := renderPUML(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			assertRendersClean(t, endpoint, string(puml))
		})
	}
}

func journeySemantics2Doc(t *testing.T) *craft.CraftDoc {
	t.Helper()
	src, err := os.ReadFile("testdata/journey-semantics-2.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return analysis.Workspace([]analysis.FileResult{analysis.File("journey-semantics-2.craft", string(src))}).Doc
}

// TestJourneySemantics2_MermaidSequenceRenders proves mermaid-cli accepts the
// v2.24 constructs: `participant X as X (scheduler)` labels, the one-message
// `loop stream` box, scope in a Note title, and the `(inferred)` suffix.
// Flow, architecture and C4 are rendered too.
func TestJourneySemantics2_MermaidSequenceRenders(t *testing.T) {
	doc := journeySemantics2Doc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			src, err := renderMermaid(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if v == visualizer.ViewSequence && !strings.Contains(src, "\n    loop stream\n") {
				t.Fatalf("expected a loop stream box in the sequence source:\n%s", src)
			}
			assertMermaidRenders(t, src)
		})
	}
}

// TestJourneySemantics2_PlantUMLRenders renders every view of the
// journey-semantics-2 fixture on the strict PlantUML server: stereotyped
// boundaries, `-[#999999]>` inferred arrows, `loop stream` boxes, and C4's
// SQL/ADDR technologies and egress relation.
func TestJourneySemantics2_PlantUMLRenders(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	doc := journeySemantics2Doc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			puml, _, err := renderPUML(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			assertRendersClean(t, endpoint, string(puml))
		})
	}
}

func journeySemantics3Doc(t *testing.T) *craft.CraftDoc {
	t.Helper()
	src, err := os.ReadFile("testdata/journey-semantics-3.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return analysis.Workspace([]analysis.FileResult{analysis.File("journey-semantics-3.craft", string(src))}).Doc
}

// TestJourneySemantics3_MermaidSequenceRenders proves mermaid-cli accepts the
// v2.25 fixture: link notes (`Note over A,B: ref: …`) in the sequence view,
// and flow, architecture and C4 views of a model with //@ cites.
func TestJourneySemantics3_MermaidSequenceRenders(t *testing.T) {
	doc := journeySemantics3Doc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			src, err := renderMermaid(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if v == visualizer.ViewSequence && !strings.Contains(src, ": ref: Site session preamble\n") {
				t.Fatalf("expected an includes note in the sequence source:\n%s", src)
			}
			assertMermaidRenders(t, src)
		})
	}
}

// TestJourneySemantics3_PlantUMLRenders renders every view of the
// journey-semantics-3 fixture on the strict PlantUML server, above all the
// two-participant `ref over A, B : …` frames of the links.
func TestJourneySemantics3_PlantUMLRenders(t *testing.T) {
	endpoint := plantUMLEndpoint(t)
	doc := journeySemantics3Doc(t)
	for _, v := range []visualizer.View{visualizer.ViewSequence, visualizer.ViewFlow, visualizer.ViewArchitecture, visualizer.ViewC4} {
		t.Run(string(v), func(t *testing.T) {
			puml, _, err := renderPUML(doc, v)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if v == visualizer.ViewSequence && !strings.Contains(string(puml), "ref over Seller, frontendplatform : Site session preamble\n") {
				t.Fatalf("expected a two-participant ref frame:\n%s", puml)
			}
			assertRendersClean(t, endpoint, string(puml))
		})
	}
}

// assertMermaidRenders runs mermaid-cli on src in a container and fails when
// mmdc exits non-zero or writes an empty SVG. It skips when no container
// runtime is available, like plantUMLEndpoint.
func assertMermaidRenders(t *testing.T, src string) {
	t.Helper()
	ctx := context.Background()
	inPath := filepath.Join(t.TempDir(), "input.mmd")
	if err := os.WriteFile(inPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: mermaidCLIImage,
			// /tmp is world-writable so mmdc (non-root) can write output.svg there.
			Cmd: []string{"-i", "/tmp/input.mmd", "-o", "/tmp/output.svg"},
			Files: []testcontainers.ContainerFile{
				{HostFilePath: inPath, ContainerFilePath: "/tmp/input.mmd", FileMode: 0o644},
			},
			WaitingFor: wait.ForExit().WithExitTimeout(120 * time.Second),
		},
		Started: true,
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
		var logBuf []byte
		if logs, _ := c.Logs(ctx); logs != nil {
			logBuf, _ = io.ReadAll(logs)
			_ = logs.Close()
		}
		t.Fatalf("mmdc exited non-zero (%d):\n%s\n--- source ---\n%s", state.ExitCode, snippet(logBuf), src)
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

// TestJourneySpine_Renders proves both renderers accept the v2.25 sequence
// constructs: `ref over` frames (PlantUML), link notes (Mermaid), the
// spine's `title` line and the Mermaid front-matter title.
func TestJourneySpine_Renders(t *testing.T) {
	var rs []analysis.FileResult
	for _, name := range []string{"vas/nmf-books-the-draws.craft", "vas/seller-promotes-an-advert.craft"} {
		src, err := os.ReadFile("testdata/journey-spine/" + name)
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		rs = append(rs, analysis.File(name, string(src)))
	}
	resync, err := os.ReadFile("testdata/journey-spine/resync.craft")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rs = append(rs, analysis.File("img/resync.craft", string(resync)))
	rs = append(rs, analysis.File("links.craft", `use_case "Seller asks the assistant a question" {
    when Seller asks the assistant a question
        includes "Site session preamble"
        frontendplatform asks graphql to stream the answer  [assistantStream]
        alt "the answer is refused"
            continues "Seller pays at the checkout"
}
`))
	doc := analysis.Workspace(rs).Doc
	render := func(t *testing.T, backend visualizer.Backend, journey string) string {
		out, err := visualizer.New(nil).Render(doc, visualizer.Request{View: visualizer.ViewSequence, Backend: backend, Format: visualizer.FormatSource, Journey: journey})
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		return string(out.Data)
	}
	t.Run("mermaid spine", func(t *testing.T) {
		assertMermaidRenders(t, render(t, visualizer.Mermaid, "seller-promotes-an-advert"))
	})
	// v2.25.1: a call to a member's receives, drawn from the caller.
	t.Run("mermaid call spine", func(t *testing.T) {
		assertMermaidRenders(t, render(t, visualizer.Mermaid, "resync"))
	})
	t.Run("mermaid links", func(t *testing.T) {
		assertMermaidRenders(t, render(t, visualizer.Mermaid, ""))
	})
	endpoint := plantUMLEndpoint(t)
	t.Run("plantuml spine", func(t *testing.T) {
		assertRendersClean(t, endpoint, render(t, visualizer.PlantUML, "seller-promotes-an-advert"))
	})
	t.Run("plantuml call spine", func(t *testing.T) {
		assertRendersClean(t, endpoint, render(t, visualizer.PlantUML, "resync"))
	})
	t.Run("plantuml links", func(t *testing.T) {
		puml := render(t, visualizer.PlantUML, "")
		if !strings.Contains(puml, "ref over") {
			t.Fatalf("no ref frame:\n%s", puml)
		}
		assertRendersClean(t, endpoint, puml)
	})
}
