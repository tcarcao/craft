package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const resolveFixture = `actors {
    user Customer
    system Billing
}

domain re {
    Billing
    Ledger
    Subscriptions
}

domain ops {
    Ledger
    Monitoring
}

services {
    BillingService {
        contexts: Billing, Subscriptions
        data-stores: billing_db
    }
    OpsService {
        contexts: Monitoring
    }
}
`

func TestResolveParticipant(t *testing.T) {
	_, ws, _ := buildLintWorkspace(t, resolveFixture)
	bc := func(id, svc string) model.Participant {
		return model.Participant{Kind: model.ParticipantBoundedContext, ID: id, Service: svc}
	}
	cases := []struct {
		name, scope, ref string
		want             model.Participant
		ambiguous        bool
	}{
		{"qualified bounded context", "", "ops/Ledger", bc("ops/Ledger", ""), false},
		{"qualified, with an owning service", "", "re/Subscriptions", bc("re/Subscriptions", "BillingService"), false},
		{"qualified, not declared in that domain", "", "ops/Billing", model.Participant{}, false},
		{"bare, declared once", "", "Monitoring", bc("ops/Monitoring", "OpsService"), false},
		{"bare, declared twice, is ambiguous", "", "Ledger", model.Participant{}, true},
		{"bare, scope picks the domain", "re", "Ledger", bc("re/Ledger", ""), false},
		{"bounded context outranks a same-name actor", "", "Billing", bc("re/Billing", "BillingService"), false},
		{"domain", "", "ops", model.Participant{Kind: model.ParticipantDomain, ID: "ops"}, false},
		{"service owns itself", "", "OpsService", model.Participant{Kind: model.ParticipantService, ID: "OpsService", Service: "OpsService"}, false},
		{"actor", "", "Customer", model.Participant{Kind: model.ParticipantActor, ID: "Customer"}, false},
		{"data store carries its owner", "", "billing_db", model.Participant{Kind: model.ParticipantDataStore, ID: "billing_db", Service: "BillingService"}, false},
		{"unknown", "", "Nobody", model.Participant{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ambiguous := ResolveParticipant(ws, tc.scope, tc.ref)
			if got != tc.want || ambiguous != tc.ambiguous {
				t.Errorf("ResolveParticipant(%q, %q) = %+v, ambiguous=%v; want %+v, ambiguous=%v",
					tc.scope, tc.ref, got, ambiguous, tc.want, tc.ambiguous)
			}
		})
	}
}

// A bounded context belongs to one service. Listing it in a second service is
// reported, and the first declaration (in sorted file order) keeps it.
func TestBCInMultipleServices(t *testing.T) {
	files := map[string]string{
		"file:///a.craft": "domain re {\n    Billing\n}\nservices {\n    First {\n        contexts: Billing\n    }\n}\n",
		"file:///b.craft": "services {\n    Second {\n        contexts: Billing\n    }\n}\n",
	}
	perFile := map[string]Symbols{}
	for uri, src := range files {
		g, li, _ := syntax.Parse(src)
		perFile[uri], _ = AnalyzeFile(uri, syntax.Root(g), li)
	}
	ws, diags := MergeWorkspaceSymbols(perFile)

	if got := ws.ContextOwners["re/Billing"]; got != "First" {
		t.Errorf("owner of re/Billing = %q, want First", got)
	}
	var found *model.Diagnostic
	for i, d := range diags {
		if d.Code == "craft/sema/bc-in-multiple-services" {
			found = &diags[i]
		}
	}
	if found == nil {
		t.Fatalf("want bc-in-multiple-services, got %+v", diags)
	}
	if found.SourceURI != "file:///b.craft" || found.Severity != model.SeverityWarning || found.Range.Start.Line != 1 {
		t.Errorf("diagnostic = %+v; want a warning on Second's name in b.craft", *found)
	}
}

// Same-name service blocks within one file are one service, so listing a
// context in both is not a second owner.
func TestBCInMultipleServices_SameServiceSplitAcrossBlocks(t *testing.T) {
	src := "domain re {\n    Billing\n}\nservices {\n    Svc {\n        contexts: Billing\n    }\n}\nservices {\n    Svc {\n        contexts: Billing\n    }\n}\n"
	g, li, _ := syntax.Parse(src)
	syms, _ := AnalyzeFile("file:///a.craft", syntax.Root(g), li)
	_, diags := MergeWorkspaceSymbols(map[string]Symbols{"file:///a.craft": syms})
	for _, d := range diags {
		if d.Code == "craft/sema/bc-in-multiple-services" {
			t.Errorf("one service split across blocks is not two owners: %+v", d)
		}
	}
}
