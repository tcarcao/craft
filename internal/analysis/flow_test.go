package analysis

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

const flowDecls = `flow lead-message "Lead generation (phone and message)" {
    priority: P1
    owner: seeker-engagement
    metric: "Leads"
    note "counted once per seeker and advert"
}

flow lead-phone "Lead by phone" {
    priority: P2
    owner: "seeker engagement"
}
`

const flowUseCases = `use_case "Seeker sends a message" {
    tags {
        realises: lead-message
    }
    when Seeker sends a message
        chat asks messaging to deliver the message
}

use_case "Seeker calls the seller" {
    tags {
        realises: lead-phone, lead-message
    }
    when Seeker calls the seller
        chat asks telephony to connect the call
}
`

func TestFlows_Model(t *testing.T) {
	res := Workspace([]FileResult{File("b.craft", flowUseCases), File("a.craft", flowDecls)})
	doc := res.Doc
	if len(doc.Flows) != 2 {
		t.Fatalf("flows = %+v", doc.Flows)
	}
	lm := doc.FlowByRef("lead-message")
	if lm == nil || lm.Name != "Lead generation (phone and message)" || lm.Priority != "P1" || lm.Owner != "seeker-engagement" ||
		lm.Metric != "Leads" || lm.SourceURI != "a.craft" || lm.Line != 1 || len(lm.Notes) != 1 {
		t.Fatalf("lead-message = %+v", lm)
	}
	want := []model.UseCaseRef{{Name: "Seeker sends a message", SourceURI: "b.craft"}, {Name: "Seeker calls the seller", SourceURI: "b.craft"}}
	if !slices.Equal(lm.RealisedBy, want) {
		t.Errorf("lead-message realised by %+v, want %+v", lm.RealisedBy, want)
	}
	if lp := doc.FlowByRef("lead-phone"); lp == nil || lp.Owner != "seeker engagement" || lp.Metric != "" || len(lp.RealisedBy) != 1 {
		t.Errorf("lead-phone = %+v", lp)
	}
	if got := doc.UseCases[1].Realises; !slices.Equal(got, []string{"lead-phone", "lead-message"}) {
		t.Errorf("realises = %v", got)
	}
	if doc.FlowByRef("nope") != nil {
		t.Error("FlowByRef(nope) should be nil")
	}
	for _, d := range res.Diagnostics {
		if d.Severity == model.SeverityError || d.Code == "craft/sema/unresolved-flow" {
			t.Errorf("unexpected diagnostic %+v", d)
		}
	}
}

// The first declaration of a ref in sorted file order is the flow.
func TestFlows_DuplicateKeepsFirst(t *testing.T) {
	second := "flow lead-message \"Second\" {\n    priority: P3\n    owner: other\n}\n"
	doc := Workspace([]FileResult{File("z.craft", second), File("a.craft", flowDecls)}).Doc
	if len(doc.Flows) != 2 || doc.FlowByRef("lead-message").Name != "Lead generation (phone and message)" {
		t.Fatalf("flows = %+v", doc.Flows)
	}
}

func TestFlows_RealisesDiagnostics(t *testing.T) {
	src := flowDecls + "use_case \"U\" {\n    tags {\n        realises: lead-message, unknown-flow, lead-message\n    }\n    when Seeker does it\n        chat does it\n}\n"
	unresolved := modelDiags(t, src, "craft/sema/unresolved-flow")
	if len(unresolved) != 1 || unresolved[0].Severity != model.SeverityWarning {
		t.Fatalf("unresolved-flow = %+v", unresolved)
	}
	// line 14 (0-based 13), `unknown-flow` after `        realises: lead-message, `
	if r := unresolved[0].Range; r.Start.Line != 13 || r.Start.Character != 32 || r.End.Character != 44 {
		t.Errorf("unresolved-flow range = %+v", r)
	}
	dup := modelDiags(t, src, "craft/lint/duplicate-flow-in-list")
	if len(dup) != 1 || dup[0].Severity != model.SeverityWarning || dup[0].Range.Start.Character != 46 {
		t.Fatalf("duplicate-flow-in-list = %+v", dup)
	}
	doc := Workspace([]FileResult{File("m.craft", src)}).Doc
	uc := doc.UseCases[0]
	if !slices.Equal(uc.Realises, []string{"lead-message", "unknown-flow"}) {
		t.Errorf("realises = %v", uc.Realises)
	}
	if rb := doc.FlowByRef("lead-message").RealisedBy; len(rb) != 1 {
		t.Errorf("a repeated ref realises once: %+v", rb)
	}
}

// A use case alone, without the file that declares its flow, validates
// with a warning only (the flow may live in a file that was not loaded).
func TestFlows_UnresolvedIsAWarning(t *testing.T) {
	for _, d := range Workspace([]FileResult{File("b.craft", flowUseCases)}).Diagnostics {
		if d.Code == "craft/sema/unresolved-flow" && d.Severity != model.SeverityWarning {
			t.Errorf("%+v", d)
		}
		if d.Severity == model.SeverityError {
			t.Errorf("unexpected error %+v", d)
		}
	}
}
