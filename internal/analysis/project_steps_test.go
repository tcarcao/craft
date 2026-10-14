package analysis_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/model"
)

const nestedSrc = `actor system CRON

domain billing {
    billing
}

use_case "U" {
    when CRON fires it
        billing starts
        alt "a"
            billing one
            opt "b"
                billing two
        else "c"
            billing three
        billing ends
}
`

func stepsOf(t *testing.T, src string) []model.Step {
	t.Helper()
	doc := analysis.File("", src).Doc
	return doc.UseCases[0].Scenarios[0].Steps
}

// zeroPositions clears Line/Column everywhere in the step tree.
func zeroPositions(steps []model.Step) {
	for _, st := range steps {
		if st.Action != nil {
			st.Action.Line, st.Action.Column = 0, 0
		}
		if st.Block != nil {
			st.Block.Line, st.Block.Column = 0, 0
			for i := range st.Block.Branches {
				st.Block.Branches[i].Line = 0
				zeroPositions(st.Block.Branches[i].Steps)
			}
		}
	}
}

// idSequence walks the tree in source order and lists the IDs.
func idSequence(steps []model.Step) []string {
	var ids []string
	for _, st := range steps {
		if st.Action != nil {
			ids = append(ids, st.Action.ID)
		}
		if st.Block != nil {
			ids = append(ids, st.Block.ID)
			for _, br := range st.Block.Branches {
				ids = append(ids, idSequence(br.Steps)...)
			}
		}
	}
	return ids
}

func TestProjectSteps_TabsAndSpacesAreEquivalent(t *testing.T) {
	tabs := stepsOf(t, strings.ReplaceAll(nestedSrc, "    ", "\t"))
	spaces := stepsOf(t, nestedSrc)
	zeroPositions(tabs)
	zeroPositions(spaces)
	if !reflect.DeepEqual(tabs, spaces) {
		t.Errorf("tab-indented steps differ from space-indented steps")
	}
	if len(spaces) != 3 || spaces[1].Block == nil {
		t.Fatalf("unexpected shape: %d top-level steps", len(spaces))
	}
}

func TestProjectSteps_IDsFollowSourceOrder(t *testing.T) {
	got := idSequence(stepsOf(t, nestedSrc))
	// scenario_1 takes 1; then source order across blocks and actions.
	want := []string{"action_2", "block_3", "action_4", "block_5", "action_6", "action_7", "action_8"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v, want %v", got, want)
	}
}

func TestProjectSteps_IncompleteActionInBranchConsumesTwo(t *testing.T) {
	src := strings.Replace(nestedSrc, "billing one\n", "billing\n            billing one\n", 1)
	got := idSequence(stepsOf(t, src))
	// The incomplete `billing` line takes 2 numbers (4, 5); "one" is 6.
	want := []string{"action_2", "block_3", "action_6", "block_7", "action_8", "action_9", "action_10"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IDs = %v, want %v", got, want)
	}
}

// Notes do not count as steps of a fan-out par: each attaches to the branch of
// the action before it, or to the first branch when it comes first.
func TestProject_FanOutParNotesJoinTheBranchOfTheStepBeforeThem(t *testing.T) {
	steps := stepsOf(t, `domain d {
    x
    y
}
use_case "U" {
    when O does it
        par "lookups"
            note "first"
            x asks y for a
            note "about a"
            x asks y for b
            finding "about b"
}
`)
	b := steps[0].Block
	if b == nil || !b.Split || len(b.Branches) != 2 {
		t.Fatalf("block = %+v", b)
	}
	kinds := func(br model.Branch) string {
		var out []string
		for _, st := range br.Steps {
			switch {
			case st.Note != nil:
				out = append(out, st.Note.ID+":"+st.Note.Text)
			case st.Action != nil:
				out = append(out, st.Action.ID)
			}
		}
		return strings.Join(out, " ")
	}
	if got, want := kinds(b.Branches[0]), "note_3:first action_4 note_5:about a"; got != want {
		t.Errorf("branch 0 = %q, want %q", got, want)
	}
	if got, want := kinds(b.Branches[1]), "action_6 note_7:about b"; got != want {
		t.Errorf("branch 1 = %q, want %q", got, want)
	}
	if b.Branches[0].Line != 9 || b.Branches[1].Line != 11 {
		t.Errorf("branch lines = %d, %d; want the lines of their actions (9, 11)", b.Branches[0].Line, b.Branches[1].Line)
	}
}
