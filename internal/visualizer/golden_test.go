package visualizer

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// goldenModels are the models every view is rendered from: the mixed corpus
// (whole-model examples) plus the VAS and journey-semantics fixtures.
func goldenModels(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("../../testdata/corpus/99_mixed/*.craft")
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus models found: %v", err)
	}
	files = append(files, "testdata/vas.craft", "testdata/journey-semantics.craft")
	out := map[string]string{}
	for _, f := range files {
		out[strings.TrimSuffix(filepath.Base(f), ".craft")] = f
	}
	return out
}

// goldenRenders lists every view × backend variant, as Render requests.
var goldenRenders = map[string]Request{
	"flow.puml":              {View: ViewFlow, Backend: PlantUML},
	"flow.mmd":               {View: ViewFlow, Backend: Mermaid},
	"architecture.puml":      {View: ViewArchitecture, Backend: PlantUML},
	"architecture.mmd":       {View: ViewArchitecture, Backend: Mermaid},
	"sequence.puml":          {View: ViewSequence, Backend: PlantUML},
	"sequence.mmd":           {View: ViewSequence, Backend: Mermaid},
	"c4-boundaries.puml":     {View: ViewC4, Backend: PlantUML, C4: &C4Options{Boundaries: true}},
	"c4-boundaries.mmd":      {View: ViewC4, Backend: Mermaid, C4: &C4Options{Boundaries: true}},
	"c4-transparent-db.puml": {View: ViewC4, Backend: PlantUML, C4: &C4Options{ShowDatabases: true}},
	"c4-transparent-db.mmd":  {View: ViewC4, Backend: Mermaid, C4: &C4Options{ShowDatabases: true}},
	"c4-focus.puml":          {View: ViewC4, Backend: PlantUML, C4: &C4Options{Boundaries: true, ShowDatabases: true}},
	"c4-focus.mmd":           {View: ViewC4, Backend: Mermaid, C4: &C4Options{Boundaries: true, ShowDatabases: true}},
}

// render draws one golden; the focus variants focus on the model's first
// service.
func render(t *testing.T, doc *craft.CraftDoc, kind string, req Request) string {
	t.Helper()
	req.Format = FormatSource
	if strings.HasPrefix(kind, "c4-focus") && len(doc.Services) > 0 {
		c4 := *req.C4
		c4.FocusServices = []string{doc.Services[0].Name}
		req.C4 = &c4
	}
	out, err := New(nil).Render(doc, req)
	if err != nil {
		t.Fatalf("render %s: %v", kind, err)
	}
	return string(out.Data)
}

// TestGolden renders every model through every view and backend, and checks
// the output is identical across repeated runs and matches the golden file.
// Run with -update to rewrite the goldens after an intended change, then
// review the diff.
func TestGolden(t *testing.T) {
	for name, path := range goldenModels(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for kind, req := range goldenRenders {
			t.Run(name+"."+kind, func(t *testing.T) {
				analyze := func() *craft.CraftDoc {
					return analysis.Workspace([]analysis.FileResult{analysis.File(filepath.Base(path), string(src))}).Doc
				}
				got := render(t, analyze(), kind, req)
				for i := 0; i < 20; i++ {
					if again := render(t, analyze(), kind, req); again != got {
						t.Fatalf("output differs between runs (run %d)", i)
					}
				}
				golden := filepath.Join("testdata", "golden", name+"."+kind)
				if *updateGolden {
					if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("missing golden (run with -update): %v", err)
				}
				if got != string(want) {
					t.Errorf("output differs from %s (run with -update and review the diff)\n--- got ---\n%s", golden, got)
				}
			})
		}
	}
}

// TestGolden_SurfaceMarkdown renders the surface fixture (surfaceModel, the
// spec §8.1 example) as the surface view and compares it with
// testdata/golden/surface.md. Run with -update to rewrite it.
func TestGolden_SurfaceMarkdown(t *testing.T) {
	out, err := New(nil).Render(resolvedDoc(t, surfaceModel), Request{View: ViewSurface, Backend: PlantUML, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "surface.md")
	if *updateGolden {
		if err := os.WriteFile(golden, out.Data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	if string(out.Data) != string(want) {
		t.Errorf("output differs from %s (run with -update and review the diff)\n--- got ---\n%s", golden, out.Data)
	}
}
