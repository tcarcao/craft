package lsp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/workspace"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// Every position the server publishes is an LSP position, whose Character is a
// count of UTF-16 code units. This test pins that by comparing a non-ASCII
// document against an ASCII twin built to be identical in UTF-16 columns:
//
//	"Ação" -> "Acao"  (4 UTF-16 units; 6 bytes vs 4)
//	"café" -> "cafe"  (4 UTF-16 units; 5 bytes vs 4)
//	"🚀"    -> "xx"    (2 UTF-16 units; 4 bytes vs 2)
//
// Both documents therefore have identical LSP geometry, and every handler must
// return byte-for-byte identical positions for them. Any place that leaks a
// byte column (or a byte length) into an LSP Position breaks this immediately.
// Each pair must be identical in UTF-16 units and differ in bytes.
// TestUTF16Twin_FixtureIsGeometricallyIdentical enforces that.
var twinReplacer = strings.NewReplacer(
	"Faturação", "Faturacao", // 9 units; 11 bytes vs 9
	"Nãoexiste", "Naoexiste", // 9 units; 10 bytes vs 9
	"Cobrança", "Cobranca", // 8 units; 9 bytes vs 8
	"Ação", "Acao", // 4 units; 6 bytes vs 4
	"café", "cafe", // 4 units; 5 bytes vs 4
	"🚀", "xx", // 2 units; 4 bytes vs 2
	"—", "-", // 1 unit;  3 bytes vs 1
)

func asciiTwin(src string) string { return twinReplacer.Replace(src) }

// The oracle is only meaningful if the two documents really do have identical
// UTF-16 geometry and genuinely different byte geometry.
func TestUTF16Twin_FixtureIsGeometricallyIdentical(t *testing.T) {
	utf8Lines := strings.Split(twinDoc, "\n")
	asciiLines := strings.Split(asciiTwin(twinDoc), "\n")

	if len(utf8Lines) != len(asciiLines) {
		t.Fatalf("line count differs: %d vs %d", len(utf8Lines), len(asciiLines))
	}
	sawByteDifference := false
	for i := range utf8Lines {
		if got, want := utf16Length(utf8Lines[i]), utf16Length(asciiLines[i]); got != want {
			t.Errorf("line %d UTF-16 width: %q is %d, twin %q is %d",
				i, utf8Lines[i], got, asciiLines[i], want)
		}
		if len(utf8Lines[i]) != len(asciiLines[i]) {
			sawByteDifference = true
		}
	}
	if !sawByteDifference {
		t.Error("twins are byte-identical — the fixture cannot detect byte/UTF-16 confusion")
	}
}

// Accented names deliberately appear mid-line, with more tokens after them on
// the same line — that is where a byte/UTF-16 mix-up shows up, since the drift
// only affects positions to the right of a multi-byte character.
const twinDoc = `// café — notas 🚀
actors {
  user Ação
  system Bob
  user Ação
}

domain Faturação { Ação Recibos }

services {
  Cobrança {
    contexts: Ação, Recibos
    language: golang
  }
}

use_case "Emitir Recibo café" {
  tags {
    jornada: café
    jornada: Ação
  }

  when Ação submits Pedido
    Ação validates Pedido
    Ação asks Recibos to emit café
    Ação asks Inexistente to fail lookup
    Ação asks Nãoexiste to fail again
    Recibos notifies "Recibo Emitido"
}
`

func openTwin(t *testing.T, src string) (*Server, protocol.TextDocumentIdentifier) {
	t.Helper()
	const uri = "file:///twin.craft"
	s := &Server{ws: workspace.New(nil)}
	s.ws.Open(uri, src)
	return s, protocol.TextDocumentIdentifier{URI: uri}
}

// jsonEq renders both values as JSON so a mismatch names the exact field.
func jsonEq(t *testing.T, label string, got, want any) {
	t.Helper()
	g, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	w, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(g) != string(w) {
		t.Errorf("%s: non-ASCII document disagrees with its ASCII twin\n--- non-ASCII ---\n%s\n--- ASCII twin ---\n%s",
			label, g, w)
	}
}

func TestUTF16Twin_DocumentSymbol(t *testing.T) {
	su, du := openTwin(t, twinDoc)
	sa, da := openTwin(t, asciiTwin(twinDoc))

	got, err := su.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{TextDocument: du})
	if err != nil {
		t.Fatalf("DocumentSymbol (utf8): %v", err)
	}
	want, err := sa.DocumentSymbol(context.Background(), &protocol.DocumentSymbolParams{TextDocument: da})
	if err != nil {
		t.Fatalf("DocumentSymbol (ascii): %v", err)
	}

	gotRanges := symbolRanges(got)
	wantRanges := symbolRanges(want)
	jsonEq(t, "DocumentSymbol ranges", gotRanges, wantRanges)
}

// symbolRanges strips names (which legitimately differ between twins) and
// keeps only the geometry.
func symbolRanges(syms []any) []protocol.Range {
	var out []protocol.Range
	for _, s := range syms {
		switch v := s.(type) {
		case protocol.DocumentSymbol:
			out = append(out, v.Range, v.SelectionRange)
			for _, c := range v.Children {
				out = append(out, c.Range, c.SelectionRange)
			}
		case protocol.SymbolInformation:
			out = append(out, v.Location.Range)
		}
	}
	return out
}

func TestUTF16Twin_Diagnostics(t *testing.T) {
	const uri = "file:///twin.craft"
	su, _ := openTwin(t, twinDoc)
	sa, _ := openTwin(t, asciiTwin(twinDoc))

	// Per-file diagnostics (syntax + sema) and workspace diagnostics
	// (cross-file resolution + lints) travel different code paths, so compare
	// both. The fixture must actually produce some of each — a silent drop to
	// zero would make this test vacuous.
	cases := []struct {
		label string
		got   []craft.Diagnostic
		want  []craft.Diagnostic
	}{
		{"per-file", su.ws.Get(uri).Diagnostics, sa.ws.Get(uri).Diagnostics},
		{"workspace", su.ws.WorkspaceDiagnostics(uri), sa.ws.WorkspaceDiagnostics(uri)},
	}
	for _, c := range cases {
		if len(c.got) == 0 {
			t.Errorf("%s: fixture produced no diagnostics — nothing is being checked", c.label)
			continue
		}
		if len(c.got) != len(c.want) {
			t.Errorf("%s: diagnostic count differs: utf8 %d, ascii %d", c.label, len(c.got), len(c.want))
			continue
		}
		for i := range c.got {
			if c.got[i].Range != c.want[i].Range {
				t.Errorf("%s diagnostic %d (%s) range: utf8 %+v, ascii %+v",
					c.label, i, c.got[i].Code, c.got[i].Range, c.want[i].Range)
			}
		}
	}
}

// Definition is driven by the cursor position, so drive both twins from the
// same LSP position and require the same target.
func TestUTF16Twin_Definition(t *testing.T) {
	// Every UTF-16 position on every line of the document.
	utf8Lines := strings.Split(twinDoc, "\n")
	asciiLines := strings.Split(asciiTwin(twinDoc), "\n")

	su, du := openTwin(t, twinDoc)
	sa, da := openTwin(t, asciiTwin(twinDoc))

	for line := range utf8Lines {
		for char := range utf16Length(asciiLines[line]) + 1 {
			pos := protocol.Position{Line: uint32(line), Character: char}

			got, err := su.Definition(context.Background(), &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: du, Position: pos},
			})
			if err != nil {
				t.Fatalf("Definition (utf8) at %d:%d: %v", line, char, err)
			}
			want, err := sa.Definition(context.Background(), &protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: da, Position: pos},
			})
			if err != nil {
				t.Fatalf("Definition (ascii) at %d:%d: %v", line, char, err)
			}
			if len(got) != len(want) {
				t.Fatalf("Definition at %d:%d: utf8 returned %d locations, ascii %d", line, char, len(got), len(want))
			}
			for i := range got {
				if got[i].Range != want[i].Range {
					t.Errorf("Definition at %d:%d loc %d: utf8 %+v, ascii %+v",
						line, char, i, got[i].Range, want[i].Range)
				}
			}
		}
	}
}

func TestUTF16Twin_Hover(t *testing.T) {
	utf8Lines := strings.Split(twinDoc, "\n")
	asciiLines := strings.Split(asciiTwin(twinDoc), "\n")

	su, du := openTwin(t, twinDoc)
	sa, da := openTwin(t, asciiTwin(twinDoc))

	for line := range utf8Lines {
		for char := range utf16Length(asciiLines[line]) + 1 {
			pos := protocol.Position{Line: uint32(line), Character: char}

			got, err := su.Hover(context.Background(), &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: du, Position: pos},
			})
			if err != nil {
				t.Fatalf("Hover (utf8) at %d:%d: %v", line, char, err)
			}
			want, err := sa.Hover(context.Background(), &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: da, Position: pos},
			})
			if err != nil {
				t.Fatalf("Hover (ascii) at %d:%d: %v", line, char, err)
			}
			if (got == nil) != (want == nil) {
				t.Fatalf("Hover at %d:%d: utf8 nil=%v, ascii nil=%v", line, char, got == nil, want == nil)
			}
			if got != nil && got.Range != nil && want.Range != nil && *got.Range != *want.Range {
				t.Errorf("Hover at %d:%d: utf8 %+v, ascii %+v", line, char, *got.Range, *want.Range)
			}
		}
	}
}

// Formatting replaces the whole document, so its edit range ends at
// end-of-file. That end character is a UTF-16 column like every other.
func TestUTF16Twin_Formatting(t *testing.T) {
	// Badly indented so the formatter actually returns an edit, and ending on
	// a line that holds a multi-byte character with no trailing newline.
	const utf8Src = "actors {\n\t\tuser Bob\n}\n// fim café"
	asciiSrc := asciiTwin(utf8Src)

	su, du := openTwin(t, utf8Src)
	sa, da := openTwin(t, asciiSrc)

	got, err := su.Formatting(context.Background(), &protocol.DocumentFormattingParams{TextDocument: du})
	if err != nil {
		t.Fatalf("Formatting (utf8): %v", err)
	}
	want, err := sa.Formatting(context.Background(), &protocol.DocumentFormattingParams{TextDocument: da})
	if err != nil {
		t.Fatalf("Formatting (ascii): %v", err)
	}
	if len(got) == 0 || len(want) == 0 {
		t.Fatalf("fixture produced no formatting edit (utf8 %d, ascii %d) — nothing is being checked",
			len(got), len(want))
	}
	if len(got) != len(want) {
		t.Fatalf("edit count differs: utf8 %d, ascii %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Range != want[i].Range {
			t.Errorf("edit %d range: utf8 %+v, ascii %+v", i, got[i].Range, want[i].Range)
		}
	}
}

func TestUTF16Twin_SemanticTokens(t *testing.T) {
	su, du := openTwin(t, twinDoc)
	sa, da := openTwin(t, asciiTwin(twinDoc))

	got, err := su.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{TextDocument: du})
	if err != nil {
		t.Fatalf("SemanticTokensFull (utf8): %v", err)
	}
	want, err := sa.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{TextDocument: da})
	if err != nil {
		t.Fatalf("SemanticTokensFull (ascii): %v", err)
	}
	if len(got.Data) != len(want.Data) {
		t.Fatalf("token count differs: utf8 %d, ascii %d", len(got.Data)/5, len(want.Data)/5)
	}
	for i := 0; i+4 < len(got.Data); i += 5 {
		// Field 2 is the token length, which is legitimately in UTF-16 units
		// and identical between twins by construction. All five must match.
		for j := range 5 {
			if got.Data[i+j] != want.Data[i+j] {
				t.Errorf("token %d field %d: utf8 %d, ascii %d",
					i/5, j, got.Data[i+j], want.Data[i+j])
				break
			}
		}
	}
}
