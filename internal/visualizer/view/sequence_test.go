package view

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
)

func TestBlockOps(t *testing.T) {
	alt0 := BranchRef{Block: "b1", Kind: lang.BlockAlt, Label: "accepted", Index: 0}
	alt1 := BranchRef{Block: "b1", Kind: lang.BlockAlt, Label: "declined", Index: 1}
	opt := BranchRef{Block: "b2", Kind: lang.BlockOpt, Label: "retry fails", Index: 0}
	sib := BranchRef{Block: "b3", Kind: lang.BlockOpt, Label: "later", Index: 0}
	in0 := BranchRef{Block: "b4", Kind: lang.BlockAlt, Label: "fast", Index: 0}
	in1 := BranchRef{Block: "b4", Kind: lang.BlockAlt, Label: "slow", Index: 1}
	cases := []struct {
		name       string
		prev, next []BranchRef
		want       []BlockOp
	}{
		{"none", nil, nil, nil},
		{"same branch", []BranchRef{alt0}, []BranchRef{alt0}, nil},
		{"enter", nil, []BranchRef{alt0},
			[]BlockOp{{Kind: BlockOpen, BlockKind: lang.BlockAlt, Label: "accepted"}}},
		{"enter nested", nil, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: BlockOpen, BlockKind: lang.BlockAlt, Label: "declined"}, {Kind: BlockOpen, BlockKind: lang.BlockOpt, Label: "retry fails"}}},
		{"else switch", []BranchRef{alt0}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockElse, BlockKind: lang.BlockAlt, Label: "declined"}}},
		{"nested exit", []BranchRef{alt1, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}}},
		{"full exit", []BranchRef{alt1, opt}, nil,
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}, {Kind: BlockClose, BlockKind: lang.BlockAlt, Label: "declined"}}},
		{"else switch from inside a nested block", []BranchRef{alt0, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}, {Kind: BlockElse, BlockKind: lang.BlockAlt, Label: "declined"}}},
		{"else switch into a nested block", []BranchRef{alt0}, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: BlockElse, BlockKind: lang.BlockAlt, Label: "declined"}, {Kind: BlockOpen, BlockKind: lang.BlockOpt, Label: "retry fails"}}},
		{"else switch at depth 1", []BranchRef{alt1, in0}, []BranchRef{alt1, in1},
			[]BlockOp{{Kind: BlockElse, BlockKind: lang.BlockAlt, Label: "slow"}}},
		{"else switch at depth 1 from deeper", []BranchRef{alt1, in0, opt}, []BranchRef{alt1, in1},
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}, {Kind: BlockElse, BlockKind: lang.BlockAlt, Label: "slow"}}},
		{"prev is a proper prefix of next", []BranchRef{alt1}, []BranchRef{alt1, in0, opt},
			[]BlockOp{{Kind: BlockOpen, BlockKind: lang.BlockAlt, Label: "fast"}, {Kind: BlockOpen, BlockKind: lang.BlockOpt, Label: "retry fails"}}},
		{"sibling blocks", []BranchRef{opt}, []BranchRef{sib},
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}, {Kind: BlockOpen, BlockKind: lang.BlockOpt, Label: "later"}}},
		{"sibling blocks inside a branch", []BranchRef{alt1, opt}, []BranchRef{alt1, sib},
			[]BlockOp{{Kind: BlockClose, BlockKind: lang.BlockOpt, Label: "retry fails"}, {Kind: BlockOpen, BlockKind: lang.BlockOpt, Label: "later"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BlockOps(c.prev, c.next); !reflect.DeepEqual(got, c.want) {
				t.Errorf("BlockOps = %+v, want %+v", got, c.want)
			}
		})
	}
}

// The view numbers actions with its own counter (scenarioLayout.walk) and the
// interaction package with another (walker.walk). They must agree, or a
// message lands in the wrong place. Every interaction with a step must be
// drawn exactly once, in the order and branch the view assigns it.
func TestSequenceStepsMatchInteractionSteps(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    scenario "online order"
    when Customer places an order
        Ordering asks Billing to first
        alt "a"
            alt "a-inner"
                Billing asks Ordering to second
            else "a-inner-empty"
                opt "deep empty"
        else "b"
        Ordering asks Billing to third
        opt "c"
            Billing asks Ordering to fourth
            opt "c-inner"
                Ordering asks Billing to fifth
        Ordering asks Billing to sixth
    scenario "phone order"
    when Customer phones in an order
        alt "x"
            Billing asks Ordering to seventh
        else "y"
        Ordering asks Billing to eighth
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("test.craft", src)}).Doc
	its := interaction.Derive(doc)
	seq := BuildSequence(doc, its)
	if len(seq.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(seq.Sections))
	}
	for si, sc := range doc.UseCases[0].Scenarios {
		title := scenarioTitle(si, sc)
		var want []interaction.Interaction
		lastStep := 0
		for _, it := range its {
			if it.UseCaseIndex != 0 || it.Scenario != sc.ID || it.Step == 0 {
				continue
			}
			if it.Step < lastStep {
				t.Errorf("%s: step %d after %d: interactions not in step order", title, it.Step, lastStep)
			}
			lastStep = it.Step
			want = append(want, it)
		}
		var got []Message
		for _, m := range seq.Sections[0].Messages {
			if m.Scenario == title && m.Kind != MessageNone {
				got = append(got, m)
			}
		}
		// The first message of each scenario is its trigger (step 0).
		if len(got) < 1 {
			t.Fatalf("%s: no messages drawn", title)
		}
		got = got[1:]
		if len(got) != len(want) {
			t.Fatalf("%s: drew %d step messages, interactions have %d", title, len(got), len(want))
		}
		for i, it := range want {
			if got[i].Label != it.Label || !reflect.DeepEqual(got[i].Branch, branchPath(it.Branch)) {
				t.Errorf("%s: message %d = %q in %+v, want %q in %+v (step %d)", title, i, got[i].Label, got[i].Branch, it.Label, branchPath(it.Branch), it.Step)
			}
		}
	}
}
