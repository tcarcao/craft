package visualizer

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/mermaid"
	"github.com/tcarcao/craft/v2/internal/visualizer/plantuml"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// goldenModels are the models every view is rendered from: the mixed corpus
// (whole-model examples) plus the VAS fixture.
func goldenModels(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("../../testdata/corpus/99_mixed/*.craft")
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus models found: %v", err)
	}
	files = append(files, "testdata/vas.craft")
	out := map[string]string{}
	for _, f := range files {
		out[strings.TrimSuffix(filepath.Base(f), ".craft")] = f
	}
	return out
}

// goldenRenders lists every view × backend; each renders a resolved model.
var goldenRenders = map[string]func(doc *craft.CraftDoc) string{
	"flow.puml":         func(d *craft.CraftDoc) string { return plantuml.Flow(view.BuildFlow(d, interaction.Derive(d))) },
	"flow.mmd":          func(d *craft.CraftDoc) string { return mermaid.Flow(view.BuildFlow(d, interaction.Derive(d))) },
	"architecture.puml": func(d *craft.CraftDoc) string { return plantuml.Architecture(view.BuildArchitecture(d, interaction.Derive(d))) },
	"architecture.mmd":  func(d *craft.CraftDoc) string { return mermaid.Architecture(view.BuildArchitecture(d, interaction.Derive(d))) },
	"sequence.puml":     func(d *craft.CraftDoc) string { return plantuml.Sequence(view.BuildSequence(d, interaction.Derive(d))) },
	"sequence.mmd":      func(d *craft.CraftDoc) string { return mermaid.Sequence(view.BuildSequence(d, interaction.Derive(d))) },
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
		for kind, render := range goldenRenders {
			t.Run(name+"."+kind, func(t *testing.T) {
				analyze := func() *craft.CraftDoc {
					return analysis.Workspace([]analysis.FileResult{analysis.File(filepath.Base(path), string(src))}).Doc
				}
				got := render(analyze())
				for i := 0; i < 20; i++ {
					if again := render(analyze()); again != got {
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
