package visualizer

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

type failRasterizer struct{ t *testing.T }

func (f failRasterizer) Rasterize(context.Context, Backend, []byte, Format) ([]byte, error) {
	f.t.Error("the board must not rasterize")
	return nil, nil
}

func analyzeFile(t *testing.T, path string) *craft.CraftDoc {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return analysis.Workspace([]analysis.FileResult{analysis.File(filepath.Base(path), string(src))}).Doc
}

func renderBoard(t *testing.T, doc *craft.CraftDoc, req Request) string {
	t.Helper()
	req.View, req.Backend, req.Format = ViewFlow, Board, FormatSVG
	out, err := New(nil).Render(doc, req)
	if err != nil {
		t.Fatal(err)
	}
	return string(out.Data)
}

func mustParseXML(t *testing.T, svg string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := d.Token(); err != nil {
			if err.Error() != "EOF" {
				t.Fatalf("not well-formed XML: %v", err)
			}
			return
		}
	}
}

func TestValidate_Board(t *testing.T) {
	if err := validate(Request{View: ViewFlow, Backend: Board, Format: FormatSVG}); err != nil {
		t.Fatal(err)
	}
	for _, req := range []Request{
		{View: ViewSequence, Backend: Board, Format: FormatSVG},
		{View: ViewFlow, Backend: Board, Format: FormatPNG},
		{View: ViewFlow, Backend: Board, Format: FormatSource},
		{View: ViewSurface, Backend: Board, Format: FormatMarkdown},
		{View: ViewFlow, Backend: Board, Format: FormatSVG, Journey: "x"},
	} {
		if err := validate(req); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%+v: err = %v, want ErrUnsupported", req, err)
		}
	}
}

func TestRender_BoardNeedsOneUseCase(t *testing.T) {
	doc := analyzeFile(t, "../../testdata/corpus/99_mixed/banking-system.craft")
	_, err := New(nil).Render(doc, Request{View: ViewFlow, Backend: Board, Format: FormatSVG})
	if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "money-transfer") {
		t.Fatalf("err = %v, want ErrUnsupported listing slugs", err)
	}
	out, err := New(nil).Render(doc, Request{View: ViewFlow, Backend: Board, Format: FormatSVG, UseCases: []string{"Money Transfer"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Ext != "svg" || out.ContentType != "image/svg+xml" || !bytes.HasPrefix(out.Data, []byte("<svg")) {
		t.Fatalf("bad output: %s %s %.20s", out.Ext, out.ContentType, out.Data)
	}
	mustParseXML(t, string(out.Data))
}

func TestRender_BoardDefaultTitle(t *testing.T) {
	doc := analyzeFile(t, "testdata/invoicing.craft")
	if got := renderBoard(t, doc, Request{}); !strings.Contains(got, "Scenario: Invoicing process starts for a prepaid Customer") {
		t.Error("default title missing")
	}
	got := renderBoard(t, doc, Request{Title: "X"})
	if !strings.Contains(got, ">X<") || strings.Contains(got, "Scenario:") {
		t.Error("explicit title not used alone")
	}
}

func TestRender_BoardNeverRasterizes(t *testing.T) {
	doc := analyzeFile(t, "testdata/invoicing.craft")
	if _, err := New(failRasterizer{t}).Render(doc, Request{View: ViewFlow, Backend: Board, Format: FormatSVG}); err != nil {
		t.Fatal(err)
	}
}

func TestBoard_EscapesLabels(t *testing.T) {
	doc := resolvedDoc(t, `domain d {
  A
  B
}

use_case "Pay & <check>" {
  when A starts
    A asks B to "fetch" & go  [GET /x]
}
`)
	got := renderBoard(t, doc, Request{})
	mustParseXML(t, got)
	if !strings.Contains(got, "&amp;") {
		t.Error("ampersand not escaped")
	}
}

func TestBoard_UsesDisplayNames(t *testing.T) {
	doc := analyzeFile(t, "testdata/display-names.craft")
	got := renderBoard(t, doc, Request{UseCases: []string{doc.UseCases[0].Name}})
	// Cloud labels wrap, so check each word of a display name.
	for _, name := range []string{"nMF, the money engine", "the hourly clock that rebuilds the prices", "price project #2; EU"} {
		for _, w := range strings.Fields(name) {
			if !strings.Contains(got, w) {
				t.Errorf("display name %q: word %q missing", name, w)
			}
		}
	}
}

func TestBoard_InternalOnlyUseCase(t *testing.T) {
	doc := resolvedDoc(t, `domain d {
  A
}

use_case "Inside" {
  when A starts
    A validates x
}
`)
	got := renderBoard(t, doc, Request{})
	mustParseXML(t, got)
	// The legend draws one cloud too: legend + the single node.
	if n := strings.Count(got, `stroke="#6130b8"`); n != 2 {
		t.Errorf("cloud paths = %d, want 2 (legend + one node)", n)
	}
	if !strings.Contains(got, "validates x") {
		t.Error("note missing")
	}
}

func TestGolden_Board(t *testing.T) {
	for _, path := range []string{"testdata/invoicing.craft", "../../testdata/corpus/99_mixed/banking-system.craft", "../../testdata/corpus/99_mixed/e-comerce.craft"} {
		model := strings.TrimSuffix(filepath.Base(path), ".craft")
		doc := analyzeFile(t, path)
		for _, uc := range doc.UseCases {
			if len(uc.Scenarios) == 0 {
				continue
			}
			t.Run(model+"."+Slugify(uc.Name), func(t *testing.T) {
				req := Request{UseCases: []string{uc.Name}}
				got := renderBoard(t, analyzeFile(t, path), req)
				for i := 0; i < 20; i++ {
					if again := renderBoard(t, analyzeFile(t, path), req); again != got {
						t.Fatalf("output differs between runs (run %d)", i)
					}
				}
				golden := filepath.Join("testdata", "golden", "board", model+"."+Slugify(uc.Name)+".svg")
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
					t.Errorf("output differs from %s (run with -update and review the diff)", golden)
				}
			})
		}
	}
}
