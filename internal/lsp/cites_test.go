package lsp

import "testing"

func TestFileURI(t *testing.T) {
	cases := []struct {
		abs  string
		line int
		want string
	}{
		{"/src/a.go", 5, "file:///src/a.go#L5"},
		{"/src/a.go", 0, "file:///src/a.go"},
		{"/my src/#1/50%.go", 2, "file:///my%20src/%231/50%25.go#L2"},
		{"C:/work/a b.go", 3, "file:///C:/work/a%20b.go#L3"},
	}
	for _, c := range cases {
		if got := fileURI(c.abs, c.line); got != c.want {
			t.Errorf("fileURI(%q, %d) = %q, want %q", c.abs, c.line, got, c.want)
		}
	}
}
