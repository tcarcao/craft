package sema_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/internal/model"
	"github.com/tcarcao/craft/internal/sema"
)

// diagRuleName returns the short rule name of a diagnostic code (the part
// after the last '/'), e.g. "craft/sema/malformed-slug" -> "malformed-slug".
// Task 7's brief refers to rule codes by this short name; the actual Code
// field follows the existing craft/{sema,lint,syntax}/<rule> convention.
func diagRuleName(code string) string {
	if i := strings.LastIndexByte(code, '/'); i >= 0 {
		return code[i+1:]
	}
	return code
}

func diagCodes(diags []model.Diagnostic) []string {
	out := make([]string, len(diags))
	for i, d := range diags {
		out[i] = d.Code
	}
	return out
}

func assertHasDiag(t *testing.T, diags []model.Diagnostic, rule string) {
	t.Helper()
	for _, d := range diags {
		if diagRuleName(d.Code) == rule {
			return
		}
	}
	t.Fatalf("expected a %q diagnostic, got %v", rule, diagCodes(diags))
}

func assertNoDiag(t *testing.T, diags []model.Diagnostic, rule string) {
	t.Helper()
	for _, d := range diags {
		if diagRuleName(d.Code) == rule {
			t.Fatalf("unexpected %q diagnostic: %+v (all: %v)", rule, d, diagCodes(diags))
		}
	}
}

func assertDiagCount(t *testing.T, diags []model.Diagnostic, rule string, want int) {
	t.Helper()
	got := 0
	for _, d := range diags {
		if diagRuleName(d.Code) == rule {
			got++
		}
	}
	if got != want {
		t.Fatalf("expected %d %q diagnostics, got %d: %v", want, rule, got, diagCodes(diags))
	}
}

func assertSeverity(t *testing.T, diags []model.Diagnostic, rule string, want model.Severity) {
	t.Helper()
	for _, d := range diags {
		if diagRuleName(d.Code) == rule {
			if d.Severity != want {
				t.Errorf("%s: expected severity %q, got %q", rule, want, d.Severity)
			}
			return
		}
	}
	t.Fatalf("no %q diagnostic found to check severity", rule)
}

func TestValidate_MalformedSlug(t *testing.T) {
	src := `context_map {
  foo:re/x same_as term:billing/y
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertHasDiag(t, diags, "malformed-slug")
	assertSeverity(t, diags, "malformed-slug", model.SeverityError)
}

func TestValidate_MalformedSlug_BadNamespaceShape(t *testing.T) {
	// domain: namespace is fixed to the literal "re" segment (design §3.1);
	// "billing" is not a valid domain namespace.
	src := `context_map {
  domain:billing/x realized_by service:y
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertHasDiag(t, diags, "malformed-slug")
}

func TestValidate_MalformedSlug_ServiceWithNamespace(t *testing.T) {
	// service: must have NO namespace/slash.
	src := `context_map {
  bc:re/x realized_by service:ns/y
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertHasDiag(t, diags, "malformed-slug")
}

func TestValidate_ValidSlugs_NoMalformedSlug(t *testing.T) {
	src := `context_map {
  bc:re/subscriptions realized_by service:subscriptions-api
  bc:re/vas also_realizes service:vas-application-api
  term:subscriptions/dunning contrasts term:billing/dunning
  term:ordering/order same_as term:offering/order
  term:vas/apply distinct_from term:billing/apply
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertNoDiag(t, diags, "malformed-slug")
	assertNoDiag(t, diags, "edge-endpoint-kind")
}

func TestValidate_EdgeEndpointKind(t *testing.T) {
	src := `context_map {
  bc:re/x same_as service:y
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertHasDiag(t, diags, "edge-endpoint-kind")
	assertSeverity(t, diags, "edge-endpoint-kind", model.SeverityError)
}

func TestValidate_EdgeEndpointKind_RealizedByWrongDirection(t *testing.T) {
	src := `context_map {
  service:x realized_by bc:re/y
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertHasDiag(t, diags, "edge-endpoint-kind")
}

func TestValidate_DeprecatedStringRef(t *testing.T) {
	src := `use_case "x" {
  when A listens "Legacy"
    A notifies "AlsoLegacy"
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertDiagCount(t, diags, "deprecated-string-ref", 2)
	assertSeverity(t, diags, "deprecated-string-ref", model.SeverityWarning)
}

func TestValidate_TypedRefs_NoDeprecatedStringRef(t *testing.T) {
	src := `use_case "x" {
  when A listens vas.VasApplied
    A notifies vas.VasApplied
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertNoDiag(t, diags, "deprecated-string-ref")
}

func TestValidate_DuplicateServiceAnchor_OpsLevel(t *testing.T) {
	src := `services {
  Foo {
    opslevel: foo-api
    opslevel: bar-api
  }
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertDiagCount(t, diags, "duplicate-service-anchor", 1)
	assertSeverity(t, diags, "duplicate-service-anchor", model.SeverityError)
}

func TestValidate_DuplicateServiceAnchor_Repo(t *testing.T) {
	// Carried-forward regression (Task 6 review, ast.go:643): a repo: value
	// with an internal kind: prefix must not corrupt the field-cursor sync
	// used by the flat service-field scanner. Fields() reads structured
	// child nodes (not that flat scan), so this must count correctly.
	src := `services {
  Foo {
    repo: service:org/foo
    repo: another/repo
  }
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertDiagCount(t, diags, "duplicate-service-anchor", 1)
}

// TestValidate_DuplicateServiceAnchor_QuotedServiceName is the Fix-2
// regression lock for validateServiceAnchors: before the fix, svc.Name()
// returned nil for a quoted service name, so svcName stayed "" and the
// diagnostic rendered `service "": ...` instead of the actual (unquoted)
// service name.
func TestValidate_DuplicateServiceAnchor_QuotedServiceName(t *testing.T) {
	src := `service "Foo Service" {
  opslevel: foo-api
  opslevel: bar-api
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertDiagCount(t, diags, "duplicate-service-anchor", 1)
	assertSeverity(t, diags, "duplicate-service-anchor", model.SeverityError)
	var got *model.Diagnostic
	for i := range diags {
		if diagRuleName(diags[i].Code) == "duplicate-service-anchor" {
			got = &diags[i]
		}
	}
	if got == nil {
		t.Fatalf("no duplicate-service-anchor diagnostic found in %v", diagCodes(diags))
	}
	wantMsg := `service "Foo Service": "opslevel" is already declared; only one ` + "`opslevel:`" + ` is allowed per service`
	if got.Message != wantMsg {
		t.Errorf("message = %q, want %q", got.Message, wantMsg)
	}
}

func TestValidate_ServiceAnchors_NoDuplicate(t *testing.T) {
	src := `services {
  Foo {
    opslevel: foo-api
    repo: olxeu/realestate/subscriptions
  }
}`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	assertNoDiag(t, diags, "duplicate-service-anchor")
}

func TestValidate_UnresolvedRefLocal_Warning(t *testing.T) {
	// wsHasSymbols requires at least one declared actor/domain/service in the
	// file-set (matching the existing unresolved-reference guard) — declare
	// an unrelated service so the file-set is non-empty while bc:re/x and
	// service:unknown-svc remain genuinely unresolved.
	src := `services {
  RealSvc {
    opslevel: real-svc
  }
}
context_map {
  bc:re/x realized_by service:unknown-svc
}`
	uri := "file:///a.craft"
	tree := parseTreeFor(src)
	syms, _ := sema.AnalyzeFile(uri, tree)
	perFile := map[string]sema.Symbols{uri: syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	assertHasDiag(t, diags, "unresolved-ref-local")
	assertSeverity(t, diags, "unresolved-ref-local", model.SeverityWarning)
}
