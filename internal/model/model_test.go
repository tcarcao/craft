package model_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

func TestScenarioActions_FlattensStepsInSourceOrder(t *testing.T) {
	a := func(id string) model.Step { return model.Step{Action: &model.Action{ID: id}} }
	sc := model.Scenario{Steps: []model.Step{
		a("action_2"),
		{Block: &model.Block{ID: "block_3", Kind: lang.BlockAlt, Branches: []model.Branch{
			{Label: "ok", Steps: []model.Step{a("action_4")}},
			{Label: "404", Steps: []model.Step{
				a("action_5"),
				{Block: &model.Block{ID: "block_6", Kind: lang.BlockOpt, Branches: []model.Branch{
					{Label: "retry fails", Steps: []model.Step{a("action_7")}},
				}}},
			}},
		}}},
		a("action_8"),
	}}
	var got []string
	for _, act := range sc.Actions() {
		got = append(got, act.ID)
	}
	want := "action_2 action_4 action_5 action_7 action_8"
	if j := strings.Join(got, " "); j != want {
		t.Errorf("Actions() = %q, want %q", j, want)
	}
}

func nestedScenario() model.Scenario {
	a := func(id string) model.Step { return model.Step{Action: &model.Action{ID: id}} }
	return model.Scenario{Steps: []model.Step{
		a("a1"),
		{Block: &model.Block{ID: "b", Kind: lang.BlockAlt, Branches: []model.Branch{
			{Label: "x", Steps: []model.Step{a("a2")}},
			{Label: "y", Steps: []model.Step{a("a3")}},
		}}},
	}}
}

func TestScenarioEachAction_MutatesNestedActionInPlace(t *testing.T) {
	sc := nestedScenario()
	sc.EachAction(func(a *model.Action) {
		if a.ID == "a3" {
			a.Description = "changed"
		}
	})
	if got := sc.Steps[1].Block.Branches[1].Steps[0].Action.Description; got != "changed" {
		t.Errorf("tree not updated: %q", got)
	}
	if got := sc.Actions()[2].Description; got != "changed" {
		t.Errorf("Actions() not updated: %q", got)
	}
}

func TestScenarioActions_ReturnsCopies(t *testing.T) {
	sc := nestedScenario()
	acts := sc.Actions()
	acts[0].Description = "mutated"
	if got := sc.Steps[0].Action.Description; got != "" {
		t.Errorf("tree changed through Actions() result: %q", got)
	}
}

func TestScenarioEachAction_SameOrderAsActions(t *testing.T) {
	sc := nestedScenario()
	var visited []string
	sc.EachAction(func(a *model.Action) { visited = append(visited, a.ID) })
	var flat []string
	for _, a := range sc.Actions() {
		flat = append(flat, a.ID)
	}
	if strings.Join(visited, " ") != strings.Join(flat, " ") {
		t.Errorf("EachAction %v != Actions %v", visited, flat)
	}
}

func TestScenario_EmptyShapes(t *testing.T) {
	var sc model.Scenario
	if got := sc.Actions(); len(got) != 0 {
		t.Errorf("nil Steps: got %d actions", len(got))
	}
	sc.EachAction(func(*model.Action) { t.Error("visited an action in an empty scenario") })
	sc = model.Scenario{Steps: []model.Step{{Block: &model.Block{ID: "b", Kind: lang.BlockOpt}}}}
	if got := sc.Actions(); len(got) != 0 {
		t.Errorf("block without branches: got %d actions", len(got))
	}
	sc.EachAction(func(*model.Action) { t.Error("visited an action in a block without branches") })
}

func TestScenario_JSONShape(t *testing.T) {
	sc := model.Scenario{ID: "scenario_1", Steps: []model.Step{
		{Action: &model.Action{ID: "action_2"}},
		{Block: &model.Block{ID: "block_3", Kind: lang.BlockAlt, Branches: []model.Branch{
			{Label: "ok", Steps: []model.Step{}},
		}}},
	}}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["name"]; ok {
		t.Error("name should be omitted when empty")
	}
	steps, ok := m["steps"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatalf("steps = %v", m["steps"])
	}
	s0 := steps[0].(map[string]any)
	if _, ok := s0["action"]; !ok {
		t.Error("step 0 missing action key")
	}
	if _, ok := s0["block"]; ok {
		t.Error("step 0 should not have block key")
	}
	blk, ok := steps[1].(map[string]any)["block"].(map[string]any)
	if !ok {
		t.Fatal("step 1 missing block key")
	}
	br, ok := blk["branches"].([]any)
	if !ok || len(br) != 1 {
		t.Fatalf("branches = %v", blk["branches"])
	}
	if _, ok := br[0].(map[string]any)["label"]; !ok {
		t.Error("branch missing label key")
	}
}

func TestStepNote_IsNotAnAction(t *testing.T) {
	note := model.Step{Note: &model.Note{ID: "note_3", Kind: model.NoteKindFinding, Text: "no retry"}}
	sc := model.Scenario{Steps: []model.Step{
		note,
		{Action: &model.Action{ID: "action_4"}},
		{Block: &model.Block{ID: "block_5", Kind: model.BlockPar, Split: true, Branches: []model.Branch{
			{Label: "x", Steps: []model.Step{{Note: &model.Note{ID: "note_6"}}}},
			{Label: "x", Steps: []model.Step{{Action: &model.Action{ID: "action_7"}}}},
		}}},
	}}
	var got []string
	for _, a := range sc.Actions() {
		got = append(got, a.ID)
	}
	if j := strings.Join(got, " "); j != "action_4 action_7" {
		t.Errorf("Actions() = %q", j)
	}
}

func TestScenarioOpeningBlock_SkipsLeadingNotes(t *testing.T) {
	blk := &model.Block{ID: "block_3", Kind: model.BlockLoop}
	sc := model.Scenario{Steps: []model.Step{{Note: &model.Note{}}, {Block: blk}}}
	if sc.OpeningBlock() != blk {
		t.Error("OpeningBlock should skip the leading note")
	}
	sc = model.Scenario{Steps: []model.Step{{Note: &model.Note{}}, {Action: &model.Action{}}, {Block: blk}}}
	if sc.OpeningBlock() != nil {
		t.Error("a scenario that opens with an action has no opening block")
	}
}

func TestNewFields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.CraftDoc{UseCases: []model.UseCase{{Name: "U", Scenarios: []model.Scenario{{
		ID: "scenario_1", Trigger: model.Trigger{Type: model.TriggerTypeExternal},
		Steps: []model.Step{{Action: &model.Action{ID: "action_2"}}, {Block: &model.Block{ID: "block_3", Kind: model.BlockAlt}}},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"notes"`, `"note"`, `"split"`, `"store"`, `"surfaces"`, `"operation"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}

func TestEventList_FallsBackToEvent(t *testing.T) {
	if got := (model.Trigger{Event: "a.A"}).EventList(); !slices.Equal(got, []string{"a.A"}) {
		t.Errorf("single = %v", got)
	}
	if got := (model.Trigger{Event: "a.A", Events: []string{"a.A", "b.B"}}).EventList(); !slices.Equal(got, []string{"a.A", "b.B"}) {
		t.Errorf("list = %v", got)
	}
	if got := (model.Trigger{}).EventList(); got != nil {
		t.Errorf("none = %v", got)
	}
	if got := (model.Action{Event: "x.X"}).EventList(); !slices.Equal(got, []string{"x.X"}) {
		t.Errorf("action = %v", got)
	}
}

func TestOperationList(t *testing.T) {
	op := &model.Operation{Text: "a, b"}
	a := model.Action{Operation: op, Operations: []model.Operation{{Text: "a"}, {Text: "b"}}}
	if got := a.OperationList(); len(got) != 2 || got[1].Text != "b" {
		t.Errorf("list = %+v", got)
	}
	if got := (model.Action{Operation: op}).OperationList(); len(got) != 1 || got[0].Text != "a, b" {
		t.Errorf("single = %+v", got)
	}
	if got := (model.Action{}).OperationList(); got != nil {
		t.Errorf("none = %+v", got)
	}
}

func TestOperationTables(t *testing.T) {
	if got := (model.Operation{Verb: "SQL", Payload: "investment flat  city"}).Tables(); !slices.Equal(got, []string{"investment", "flat", "city"}) {
		t.Errorf("SQL = %v", got)
	}
	if got := (model.Operation{Verb: "GET", Payload: "/x"}).Tables(); got != nil {
		t.Errorf("GET = %v", got)
	}
}

func TestV224Fields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.Scenario{
		Trigger: model.Trigger{Type: model.TriggerTypeDomainListen, Event: "a.A"},
		Steps:   []model.Step{{Action: &model.Action{ID: "action_2", Type: model.ActionTypeReturn}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"events"`, `"operations"`, `"stream"`, `"answers"`, `"inferred"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}

func TestKnownActorTypes(t *testing.T) {
	want := []string{"user", "system", "service", "scheduler", "tool", "infra"}
	if got := model.KnownActorTypes(); !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestStepLink_IsNotAnActionAndNotTheFirstStep(t *testing.T) {
	blk := &model.Block{ID: "block_4", Kind: model.BlockAlt}
	sc := model.Scenario{Steps: []model.Step{
		{Link: &model.Link{ID: "link_2", Kind: model.LinkIncludes, Title: "Site session preamble"}},
		{Note: &model.Note{ID: "note_3"}},
		{Block: blk},
	}}
	if sc.OpeningBlock() != blk {
		t.Error("OpeningBlock must skip a leading link")
	}
	if len(sc.Actions()) != 0 {
		t.Error("a link is not an action")
	}
}

func TestV225Fields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.UseCase{Name: "U", Scenarios: []model.Scenario{{
		Trigger: model.Trigger{Type: model.TriggerTypeExternal},
		Steps:   []model.Step{{Action: &model.Action{ID: "action_2"}}, {Note: &model.Note{}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"sources"`, `"link"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}
