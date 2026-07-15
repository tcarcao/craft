package syntax_test

import (
	"encoding/json"
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
