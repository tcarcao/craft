package sema_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

// `alias:` repeats freely (v2.38): no duplicate-page-property or
// duplicate-flow-property, and distinct aliases raise nothing.
func TestAlias_RepeatableWithoutDiagnostics(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "page p \"P\" {\n    product_domain: x\n    alias: a\n    alias: \"b c\"\n}\n" +
			"flow f \"F\" {\n    priority: P1\n    owner: o\n    alias: a\n    alias: \"b c\"\n}\n",
	})
	if len(diags) != 0 {
		t.Errorf("diags = %+v", diags)
	}
}

func TestAlias_EmptyAndDuplicate(t *testing.T) {
	for _, tc := range []struct{ kind, src string }{
		{"page", "page p \"P\" {\n    product_domain: x\n    alias: a\n    alias: \"  \"\n    alias: \"a \"\n}\n"},
		{"flow", "flow p \"P\" {\n    priority: P1\n    owner: o\n    alias: a\n    alias: \"  \"\n    alias: \"a \"\n}\n"},
	} {
		diags := analyzeFiles(t, map[string]string{"a.craft": tc.src})
		empty := withCode(diags, "craft/sema/empty-alias")
		dup := withCode(diags, "craft/sema/duplicate-alias")
		base := strings.Count(tc.src[:strings.Index(tc.src, "alias:")], "\n")
		if len(empty) != 1 || empty[0].Severity != model.SeverityWarning || empty[0].Range.Start.Line != base+1 ||
			!strings.Contains(empty[0].Message, tc.kind+" \"p\"") {
			t.Errorf("%s: empty-alias = %+v", tc.kind, empty)
		}
		if len(dup) != 1 || dup[0].Severity != model.SeverityWarning || dup[0].Range.Start.Line != base+2 ||
			!strings.Contains(dup[0].Message, `alias "a"`) {
			t.Errorf("%s: duplicate-alias = %+v", tc.kind, dup)
		}
		if len(diags) != 2 {
			t.Errorf("%s: diags = %+v", tc.kind, diags)
		}
	}
}
