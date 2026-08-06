package syntax_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestProject_UseCase_ExternalTrigger(t *testing.T) {
	src := `use_case "Money Transfer" {
  when Customer initiates transfer
    PaymentProcessing asks AccountManagement to verify source account
    BalanceTracking notifies "Funds Reserved"

  when PaymentProcessing listens "Funds Reserved"
    PaymentProcessing asks AccountManagement to verify destination account
}`
	g, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	if len(doc.UseCases) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(doc.UseCases))
	}
	uc := doc.UseCases[0]
	if uc.Name != "Money Transfer" {
		t.Errorf("expected name 'Money Transfer', got %q", uc.Name)
	}
	if len(uc.Scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(uc.Scenarios))
	}

	sc1 := uc.Scenarios[0]
	if sc1.ID != "scenario_1" {
		t.Errorf("expected scenario_1, got %q", sc1.ID)
	}
	if sc1.Trigger.Type != "external" {
		t.Errorf("expected external trigger, got %q", sc1.Trigger.Type)
	}
	if sc1.Trigger.Actor != "Customer" {
		t.Errorf("expected actor=Customer, got %q", sc1.Trigger.Actor)
	}
	if len(sc1.Actions) != 2 {
		t.Fatalf("expected 2 actions in sc1, got %d", len(sc1.Actions))
	}
	if sc1.Actions[0].ID != "action_2" {
		t.Errorf("expected action_2, got %q", sc1.Actions[0].ID)
	}
	if sc1.Actions[0].Type != "sync_action" {
		t.Errorf("expected sync_action, got %q", sc1.Actions[0].Type)
	}
	if sc1.Actions[1].Type != "async_action" {
		t.Errorf("expected async_action, got %q", sc1.Actions[1].Type)
	}

	sc2 := uc.Scenarios[1]
	if sc2.ID != "scenario_4" {
		// sc1=1, action_2, action_3 → then scenario_4
		t.Errorf("expected scenario_4, got %q", sc2.ID)
	}
	if sc2.Trigger.Type != "domain_listen" {
		t.Errorf("expected domain_listen, got %q", sc2.Trigger.Type)
	}
	if sc2.Trigger.Context != "PaymentProcessing" {
		t.Errorf("expected domain=PaymentProcessing, got %q", sc2.Trigger.Context)
	}
	if sc2.Trigger.Event != "Funds Reserved" {
		t.Errorf("expected event='Funds Reserved', got %q", sc2.Trigger.Event)
	}

	b, _ := json.MarshalIndent(doc, "", "  ")
	t.Logf("output:\n%s", string(b))
}

func TestProject_ActionOperation(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing asks Ledger to record the entry
}`
	g, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	actions := doc.UseCases[0].Scenarios[0].Actions

	if actions[0].Operation == nil {
		t.Fatal("action 0: expected Operation to be set")
	}
	if got := actions[0].Operation.Verb; got != "POST" {
		t.Errorf("Verb = %q, want POST", got)
	}
	if got := actions[0].Operation.Payload; got != "/v1/charges" {
		t.Errorf("Payload = %q, want /v1/charges", got)
	}
	if got := actions[0].Operation.Text; got != "POST /v1/charges" {
		t.Errorf("Text = %q, want %q", got, "POST /v1/charges")
	}
	if got := actions[0].Phrase; got != "a fresh charge attempt" {
		t.Errorf("Phrase = %q, want %q", got, "a fresh charge attempt")
	}
	if got := actions[0].Description; got != "Subscriptions asks Billing for a fresh charge attempt" {
		t.Errorf("Description = %q, must not contain the annotation", got)
	}

	if actions[1].Operation != nil {
		t.Errorf("action 1: expected nil Operation, got %+v", actions[1].Operation)
	}
}

// An absent annotation must marshal to no `operation` key at all, so existing
// .craftjson goldens stay byte-identical.
func TestProject_ActionOperation_OmittedWhenAbsent(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c\n}"
	g, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	b, err := json.Marshal(doc.UseCases[0].Scenarios[0].Actions[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "operation") {
		t.Errorf("absent annotation must not emit an operation key, got %s", b)
	}
}
