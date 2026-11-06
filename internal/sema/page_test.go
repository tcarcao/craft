package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func TestPage_RequiredProductDomain(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "page ok \"OK\" {\n    product_domain: ad-inventory\n}\npage bare \"Bare\" {\n    url: \"https://x\"\n}\n",
	})
	missing := withCode(diags, "craft/sema/missing-page-property")
	if len(missing) != 1 || missing[0].Severity != model.SeverityError || missing[0].Range.Start.Line != 3 || missing[0].Range.Start.Character != 5 {
		t.Fatalf("missing-page-property = %+v", missing)
	}
}

func TestPage_Platform(t *testing.T) {
	for _, p := range []string{"web", "Frontoffice", "\"app \""} {
		diags := analyzeFiles(t, map[string]string{
			"a.craft": "page p \"P\" {\n    platform: " + p + "\n    product_domain: x\n}\n",
		})
		bad := withCode(diags, "craft/sema/invalid-platform")
		if len(bad) != 1 || bad[0].Severity != model.SeverityError || bad[0].Range.Start.Line != 1 || bad[0].Range.Start.Character != 14 {
			t.Errorf("platform %s: invalid-platform = %+v", p, bad)
		}
	}
	for _, p := range []string{"frontoffice", "backoffice", "app", "\"app\""} {
		diags := analyzeFiles(t, map[string]string{
			"a.craft": "page p \"P\" {\n    platform: " + p + "\n    product_domain: x\n}\n",
		})
		if len(diags) != 0 {
			t.Errorf("platform %s: diags = %+v", p, diags)
		}
	}
}

// `url:` repeats freely (one per site); any other property is set once.
func TestPage_DuplicateProperty(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "page p \"P\" {\n    url: \"a\"\n    url: \"b\"\n    product_domain: x\n    product_domain: y\n}\n",
	})
	dup := withCode(diags, "craft/sema/duplicate-page-property")
	if len(dup) != 1 || dup[0].Severity != model.SeverityWarning || dup[0].Range.Start.Line != 4 {
		t.Errorf("duplicate-page-property = %+v", dup)
	}
}

func TestPage_DuplicateAcrossFiles(t *testing.T) {
	src := "page p \"P\" {\n    product_domain: x\n}\n"
	diags := analyzeFiles(t, map[string]string{"a.craft": src, "b.craft": src})
	dup := withCode(diags, "craft/sema/duplicate-page")
	if len(dup) != 1 || dup[0].SourceURI != "b.craft" || dup[0].Severity != model.SeverityError {
		t.Errorf("duplicate-page = %+v", dup)
	}
}
