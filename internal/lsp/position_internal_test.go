package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// Regression for the semanticTokens/full crash: a green tree wider than the
// source produced token offsets past EOF, and converting one to an LSP position
// panicked with "slice bounds out of range". Every .craft file the repo ships
// must survive the handler and yield in-range positions.
func TestSemanticTokensFull_NonASCIIDocuments(t *testing.T) {
	srcs := map[string]string{
		"accented use-case title": "use_case \"Registo de Utilizadores ção\" {\n  when User creates Account\n    Auth validates email\n}\n",
		"accented identifiers":    "services {\n  Faturação: {\n    contexts: Pagamentos\n  }\n}\n",
		"surrogate pair":          "use_case \"ship it 🚀\" {\n  when User creates Account\n}\n",
		"em dash comment":         "// nota — aqui\nservices {\n  A: {\n    contexts: X\n  }\n}\n",
	}
	for _, dir := range []string{"../../testdata/corpus", "../../examples", "../visualizer/testdata"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".craft") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			srcs[filepath.Join(dir, e.Name())] = string(b)
		}
	}

	for name, src := range srcs {
		t.Run(name, func(t *testing.T) {
			const uri = "file:///test.craft"
			s := &Server{ws: workspace.New(nil)}
			s.ws.Open(uri, src)

			res, err := s.SemanticTokensFull(context.Background(), &protocol.SemanticTokensParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			})
			if err != nil {
				t.Fatalf("SemanticTokensFull: %v", err)
			}

			// Data is 5-tuples of (deltaLine, deltaChar, length, type, mods).
			// Walk them back to absolute positions and check each one exists.
			lines := strings.Split(src, "\n")
			var line, char uint32
			for i := 0; i+4 < len(res.Data); i += 5 {
				if d := res.Data[i]; d > 0 {
					line += d
					char = res.Data[i+1]
				} else {
					char += res.Data[i+1]
				}
				if int(line) >= len(lines) {
					t.Fatalf("token %d on line %d, file has %d lines", i/5, line, len(lines))
				}
				if utf16Len := utf16Length(lines[line]); char > utf16Len {
					t.Errorf("token %d at line %d char %d, line is %d UTF-16 units",
						i/5, line, char, utf16Len)
				}
			}
		})
	}
}

func utf16Length(s string) uint32 {
	var n uint32
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// LSP Position.Character counts UTF-16 code units, not bytes. Treating it as a
// byte column made every incremental edit after a multi-byte character on the
// same line land at the wrong offset, silently corrupting the document the
// server holds — the editor and the server then disagree about the buffer.
func TestApplyIncrementalChange_NonASCII(t *testing.T) {
	tests := []struct {
		name    string
		content string
		r       protocol.Range
		newText string
		want    string
	}{
		{
			name:    "ascii insert is unaffected",
			content: "actor user Alice\n",
			r: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 11},
				End:   protocol.Position{Line: 0, Character: 16},
			},
			newText: "Bob",
			want:    "actor user Bob\n",
		},
		{
			name:    "replace after a 2-byte rune on the same line",
			content: "actor user Ação\n",
			r: protocol.Range{
				// "Ação" is 4 UTF-16 units / 6 bytes; replace it with "Bob".
				Start: protocol.Position{Line: 0, Character: 11},
				End:   protocol.Position{Line: 0, Character: 15},
			},
			newText: "Bob",
			want:    "actor user Bob\n",
		},
		{
			name:    "append at end of a line holding a surrogate pair",
			content: "// ship 🚀\nactor user Alice\n",
			r: protocol.Range{
				// "// ship 🚀" is 10 UTF-16 units / 12 bytes.
				Start: protocol.Position{Line: 0, Character: 10},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			newText: " now",
			want:    "// ship 🚀 now\nactor user Alice\n",
		},
		{
			name:    "delete across lines below a non-ASCII line",
			content: "// café\nactor user Alice\nactor user Bob\n",
			r: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 11},
				End:   protocol.Position{Line: 2, Character: 11},
			},
			newText: "",
			want:    "// café\nactor user Bob\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			li := green.NewLineIndex(tt.content)
			got := applyIncrementalChange(tt.content, li, tt.r, tt.newText)
			if got != tt.want {
				t.Errorf("applyIncrementalChange\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// The cursor column arriving from the editor is a UTF-16 column too, so
// completion must resolve it the same way.
func TestDetectContext_NonASCIICursorColumn(t *testing.T) {
	tests := []struct {
		name string
		src  string
		line int
		col  int // 0-based UTF-16 column
		want completionContext
	}{
		{
			name: "service field after accented context name",
			src:  "services {\n  Foo {\n    contexts: Ação\n    \n  }\n}",
			line: 3, col: 4,
			want: ctxServiceField,
		},
		{
			name: "language field on a line following a surrogate pair",
			src:  "// ship 🚀\nservices {\n  Foo {\n    language: \n  }\n}",
			line: 3, col: 14,
			want: ctxServiceLang,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gn, li, _ := syntax.Parse(tt.src)
			root := syntax.Root(gn)
			if got := detectContext(root, li, tt.line, tt.col); got != tt.want {
				t.Errorf("detectContext: got %v, want %v", got, tt.want)
			}
		})
	}
}

// A non-ASCII line must resolve the cursor to the same context as its ASCII
// twin at the same UTF-16 column. Pinning the ASCII result as the oracle keeps
// the test honest about what "correct" means here. "Ação" and "Name" are both
// 4 UTF-16 columns wide, but "Ação" is 6 bytes.
func TestDetectContext_NonASCIIMatchesASCIITwin(t *testing.T) {
	const asciiSrc = "services {\n  Foo { contexts: Name }\n}"
	const utf8Src = "services {\n  Foo { contexts: Ação }\n}"

	// Walk every UTF-16 column on the line holding the accented name.
	const line = 1
	for col := range 25 {
		gnA, liA, _ := syntax.Parse(asciiSrc)
		want := detectContext(syntax.Root(gnA), liA, line, col)

		gnU, liU, _ := syntax.Parse(utf8Src)
		got := detectContext(syntax.Root(gnU), liU, line, col)

		if got != want {
			t.Errorf("col %d: non-ASCII got %v, ASCII twin got %v", col, got, want)
		}
	}
}
