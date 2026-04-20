package linter_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/linter"
	"github.com/tcarcao/craft/internal/parser"
)

// parseModel is a helper that parses a DSL string and fails the test on error.
func parseModel(t *testing.T, dsl string) *parser.DSLModel {
	t.Helper()
	p := parser.NewParser()
	model, err := p.ParseString(dsl)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return model
}

func resultsOf(severity linter.Severity, results []linter.LintResult) []linter.LintResult {
	var filtered []linter.LintResult
	for _, r := range results {
		if r.Severity == severity {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func hasMessage(results []linter.LintResult, substr string) bool {
	for _, r := range results {
		if contains(r.Message, substr) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

// ── dead-event ───────────────────────────────────────────────────────────────

func TestDeadEvent_PublishedButNotConsumed(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does thing
    PaymentService notifies "Order Created"
}
`)
	results := linter.Lint(nil, model)
	warnings := resultsOf(linter.SeverityWarning, results)
	if !hasMessage(warnings, `"Order Created"`) {
		t.Error("expected dead-event warning for 'Order Created'")
	}
}

func TestDeadEvent_PublishedAndConsumed(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does thing
    PaymentService notifies "Order Created"

  when PaymentService listens "Order Created"
    InventoryService updates stock
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"Order Created"`) && contains(r.Message, "published but never consumed") {
			t.Error("unexpected dead-event warning for consumed event 'Order Created'")
		}
	}
}

func TestDeadEvent_HasLineNumber(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does thing
    PaymentService notifies "Dead Event"
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"Dead Event"`) {
			if r.Line == 0 {
				t.Error("expected non-zero line number for dead-event result")
			}
			return
		}
	}
	t.Error("expected dead-event warning not found")
}

// ── unused-actor ─────────────────────────────────────────────────────────────

func TestUnusedActor_DefinedButNotUsed(t *testing.T) {
	model := parseModel(t, `
actors {
  user Admin
}
use_case "Test" {
  when Customer does something
    ServiceA does action
}
`)
	results := linter.Lint(nil, model)
	warnings := resultsOf(linter.SeverityWarning, results)
	if !hasMessage(warnings, `"Admin"`) {
		t.Error("expected unused-actor warning for 'Admin'")
	}
}

func TestUnusedActor_UsedAsTrigger(t *testing.T) {
	model := parseModel(t, `
actors {
  user Admin
}
use_case "Test" {
  when Admin does something
    ServiceA does action
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"Admin"`) && contains(r.Message, "never used") {
			t.Error("unexpected unused-actor warning for used actor 'Admin'")
		}
	}
}

func TestUnusedActor_HasLineNumber(t *testing.T) {
	model := parseModel(t, `
actors {
  user Ghost
}
use_case "Test" {
  when Customer does something
    ServiceA does action
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"Ghost"`) {
			if r.Line == 0 {
				t.Error("expected non-zero line number for unused-actor result")
			}
			return
		}
	}
	t.Error("expected unused-actor warning not found")
}

// ── undefined-context ────────────────────────────────────────────────────────

func TestUndefinedContext_Referenced(t *testing.T) {
	model := parseModel(t, `
domain Payments {
  ProcessingContext
}
use_case "Test" {
  when Customer does something
    GhostContext does action
}
`)
	results := linter.Lint(nil, model)
	errors := resultsOf(linter.SeverityError, results)
	if !hasMessage(errors, `"GhostContext"`) {
		t.Error("expected undefined-context error for 'GhostContext'")
	}
}

func TestUndefinedContext_Defined(t *testing.T) {
	model := parseModel(t, `
domain Payments {
  ProcessingContext
}
use_case "Test" {
  when Customer does something
    ProcessingContext does action
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"ProcessingContext"`) && contains(r.Message, "not defined") {
			t.Error("unexpected undefined-context error for defined context")
		}
	}
}

func TestUndefinedContext_NoDomainBlockSkipsRule(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does something
    AnyContext does action
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if r.Severity == linter.SeverityError && contains(r.Message, "not defined in any domain") {
			t.Error("undefined-context rule should not fire when no domains are defined")
		}
	}
}

// ── undefined-service-context ─────────────────────────────────────────────────

func TestUndefinedServiceContext_Missing(t *testing.T) {
	model := parseModel(t, `
domain Payments {
  ProcessingContext
}
services {
  PaymentService {
    contexts: ProcessingContext, GhostContext
  }
}
`)
	results := linter.Lint(nil, model)
	errors := resultsOf(linter.SeverityError, results)
	if !hasMessage(errors, `"GhostContext"`) {
		t.Error("expected undefined-service-context error for 'GhostContext'")
	}
}

func TestUndefinedServiceContext_AllDefined(t *testing.T) {
	model := parseModel(t, `
domain Payments {
  ProcessingContext
}
services {
  PaymentService {
    contexts: ProcessingContext
  }
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if r.Severity == linter.SeverityError && contains(r.Message, "ProcessingContext") {
			t.Error("unexpected undefined-service-context error for defined context")
		}
	}
}

func TestUndefinedServiceContext_HasLineNumber(t *testing.T) {
	model := parseModel(t, `
domain Payments {
  ProcessingContext
}
services {
  PaymentService {
    contexts: GhostContext
  }
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, `"GhostContext"`) {
			if r.Line == 0 {
				t.Error("expected non-zero line number for undefined-service-context result")
			}
			return
		}
	}
	t.Error("expected undefined-service-context error not found")
}

// ── event-past-tense ─────────────────────────────────────────────────────────

func TestEventPastTense_NonPastTense(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does thing
    ServiceA notifies "Order Processing"
}
`)
	results := linter.Lint(nil, model)
	warnings := resultsOf(linter.SeverityWarning, results)
	if !hasMessage(warnings, `"Order Processing"`) {
		t.Error("expected event-past-tense warning for 'Order Processing'")
	}
}

func TestEventPastTense_PastTense(t *testing.T) {
	model := parseModel(t, `
use_case "Test" {
  when Customer does thing
    ServiceA notifies "Order Created"
}
`)
	results := linter.Lint(nil, model)
	for _, r := range results {
		if contains(r.Message, "past tense") && contains(r.Message, `"Order Created"`) {
			t.Error("unexpected past-tense warning for past-tense event 'Order Created'")
		}
	}
}

// ── syntax errors with line numbers ──────────────────────────────────────────

func TestSyntaxErrors_HaveLineNumbers(t *testing.T) {
	p := parser.NewParser()
	_, err := p.ParseString(`
services {
  BadService {
    contexts: !!!invalid
  }
}
`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	errs := p.SyntaxErrors()
	if len(errs) == 0 {
		t.Fatal("expected at least one structured syntax error")
	}
	for _, se := range errs {
		if se.Line == 0 {
			t.Errorf("syntax error has zero line: %+v", se)
		}
	}
}
