package model_test

import (
	"encoding/json"
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
