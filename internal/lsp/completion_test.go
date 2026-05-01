package lsp

import (
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name string
		src  string
		line int
		col  int
		want completionContext
	}{
		{
			name: "top level blank line",
			src:  "\n",
			line: 0, col: 0,
			want: ctxTopLevel,
		},
		{
			name: "inside service block - blank line",
			src:  "service Foo {\n  \n}",
			line: 1, col: 2,
			want: ctxServiceField,
		},
		{
			name: "inside services block - blank line",
			src:  "services {\n  Foo {\n    \n  }\n}",
			line: 2, col: 4,
			want: ctxServiceField,
		},
		{
			name: "after language: in service",
			src:  "service Foo {\n  language: \n}",
			line: 1, col: 11,
			want: ctxServiceLang,
		},
		{
			name: "after contexts: in service",
			src:  "service Foo {\n  contexts: \n}",
			line: 1, col: 11,
			want: ctxServiceContexts,
		},
		{
			name: "inside domain block",
			src:  "domain Commerce {\n  \n}",
			line: 1, col: 2,
			want: ctxDomainBody,
		},
		{
			name: "inside actors block",
			src:  "actors {\n  \n}",
			line: 1, col: 2,
			want: ctxActorsBlock,
		},
		{
			name: "inside use_case - blank line",
			src:  "use_case \"X\" {\n  \n}",
			line: 1, col: 2,
			want: ctxUseCaseTop,
		},
		{
			name: "inside use_case - after when keyword",
			src:  "use_case \"X\" {\n  when \n}",
			line: 1, col: 7,
			want: ctxUseCaseTop,
		},
		{
			name: "inside use_case - action line (identifier before cursor)",
			src:  "use_case \"X\" {\n  when Actor initiates x\n    Auth \n}",
			line: 2, col: 9,
			want: ctxUseCaseAction,
		},
		{
			name: "inside expose block - blank line",
			src:  "exposure MyAPI {\n  \n}",
			line: 1, col: 2,
			want: ctxExposeField,
		},
		{
			name: "after to: in expose",
			src:  "exposure MyAPI {\n  to: \n}",
			line: 1, col: 5,
			want: ctxExposeTo,
		},
		{
			name: "after through: in expose",
			src:  "exposure MyAPI {\n  through: \n}",
			line: 1, col: 10,
			want: ctxExposeThrough,
		},
		{
			name: "after contexts: in expose",
			src:  "exposure MyAPI {\n  contexts: \n}",
			line: 1, col: 11,
			want: ctxExposeContexts,
		},
		{
			name: "inside arch block - returns unknown",
			src:  "arch MyArch {\n  \n}",
			line: 1, col: 2,
			want: ctxUnknown,
		},
		{
			name: "nested services block - field line",
			src:  "services {\n  Foo {\n    language: \n  }\n}",
			line: 2, col: 14,
			want: ctxServiceLang,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := splitLines(tc.src)
			gn, li, _ := syntax.Parse(tc.src)
			root := syntax.Root(gn)
			got := detectContext(root, li, lines, tc.line, tc.col)
			if got != tc.want {
				t.Errorf("detectContext: got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDetectContext_BlankLineInsideBlock explicitly verifies that whitespace
// tokens in the green tree (emitted for losslessness) are parented by the
// enclosing service declaration, so NodeAt returns a non-nil node and
// treeEnclosingBlock correctly identifies the surrounding block.
func TestDetectContext_BlankLineInsideBlock(t *testing.T) {
	src := "service Foo {\n  \n}"
	// Cursor at the blank line (line 1, col 2) — inside the service body.
	lines := splitLines(src)
	gn, li, _ := syntax.Parse(src)
	root := syntax.Root(gn)
	got := detectContext(root, li, lines, 1, 2)
	if got != ctxServiceField {
		t.Errorf("blank line inside service: got %v, want ctxServiceField", got)
	}
}

func TestDetectContext_CommentBraceFix(t *testing.T) {
	// findEnclosingKeyword counts `}` in a comment as a real `}` and
	// gets confused. Tree-based lookup should return ctxDomainBody.
	src := `domain Foo {
  // this has } in a comment

}`
	// Cursor at the blank line (line 2, col 0) — inside the domain body.
	lines := splitLines(src)
	gn, li, _ := syntax.Parse(src)
	root := syntax.Root(gn)
	got := detectContext(root, li, lines, 2, 0)
	if got != ctxDomainBody {
		t.Errorf("detectContext with comment brace: got %v, want ctxDomainBody", got)
	}
}
