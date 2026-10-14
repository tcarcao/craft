package model_test

import (
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
