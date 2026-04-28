package lsp

import (
	"testing"
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
			src:  "expose MyAPI {\n  \n}",
			line: 1, col: 2,
			want: ctxExposeField,
		},
		{
			name: "after to: in expose",
			src:  "expose MyAPI {\n  to: \n}",
			line: 1, col: 5,
			want: ctxExposeTo,
		},
		{
			name: "after through: in expose",
			src:  "expose MyAPI {\n  through: \n}",
			line: 1, col: 10,
			want: ctxExposeThrough,
		},
		{
			name: "after contexts: in expose",
			src:  "expose MyAPI {\n  contexts: \n}",
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
			got := detectContext(lines, tc.line, tc.col)
			if got != tc.want {
				t.Errorf("detectContext: got %v, want %v", got, tc.want)
			}
		})
	}
}
