package craft_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// The public API a consumer such as the hub reads: CraftDoc.Flows,
// Flow.RealisedBy, UseCase.Realises, CraftDoc.FlowByRef and the JSON keys.
func TestFlows_PublicAPI(t *testing.T) {
	files := map[string][]byte{
		"flows.craft": []byte("flow lead-message \"Lead generation (phone and message)\" {\n  priority: P1\n  owner: seeker-engagement\n  metric: \"Leads\"\n}\n"),
		"uc.craft": []byte("use_case \"Seeker sends a message\" {\n  tags {\n    " + craft.TagRealises + ": lead-message\n  }\n" +
			"  when Seeker sends a message\n    chat asks messaging to deliver it\n}\n"),
	}
	doc, diags, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("unexpected error %+v", d)
		}
	}
	var f *craft.Flow = doc.FlowByRef("lead-message")
	if f == nil || f.Priority != "P1" || f.Owner != "seeker-engagement" || f.Metric != "Leads" {
		t.Fatalf("flow = %+v", f)
	}
	if want := []craft.UseCaseRef{{Name: "Seeker sends a message", SourceURI: "uc.craft"}}; !slices.Equal(f.RealisedBy, want) {
		t.Errorf("RealisedBy = %+v", f.RealisedBy)
	}
	if got := doc.UseCases[0].Realises; !slices.Equal(got, []string{"lead-message"}) {
		t.Errorf("Realises = %v", got)
	}
	if got := craft.FlowPriorities(); !slices.Equal(got, []string{"P0", "P1", "P2", "P3"}) {
		t.Errorf("FlowPriorities = %v", got)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"flows":[{"ref":"lead-message"`, `"realisedBy":[{"name":"Seeker sends a message"`, `"realises":["lead-message"]`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON lacks %s:\n%s", key, b)
		}
	}
}

// A model without flows has no flows key and no realises key: its JSON is
// unchanged.
func TestFlows_AbsentKeysOmitted(t *testing.T) {
	doc, _, err := craft.Parse("a.craft", []byte("use_case \"U\" {\n  when S does it\n    c does it\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc)
	if strings.Contains(string(b), `"flows"`) || strings.Contains(string(b), `"realises"`) {
		t.Errorf("JSON = %s", b)
	}
}
