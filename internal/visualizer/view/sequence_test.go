package view

import (
	"reflect"
	"testing"
)

func TestBlockOps(t *testing.T) {
	alt0 := BranchRef{Block: "b1", Kind: BlockAlt, Label: "accepted", Index: 0}
	alt1 := BranchRef{Block: "b1", Kind: BlockAlt, Label: "declined", Index: 1}
	opt := BranchRef{Block: "b2", Kind: BlockOpt, Label: "retry fails", Index: 0}
	sib := BranchRef{Block: "b3", Kind: BlockOpt, Label: "later", Index: 0}
	in0 := BranchRef{Block: "b4", Kind: BlockAlt, Label: "fast", Index: 0}
	in1 := BranchRef{Block: "b4", Kind: BlockAlt, Label: "slow", Index: 1}
	cases := []struct {
		name       string
		prev, next []BranchRef
		want       []BlockOp
	}{
		{"none", nil, nil, nil},
		{"same branch", []BranchRef{alt0}, []BranchRef{alt0}, nil},
		{"enter", nil, []BranchRef{alt0},
			[]BlockOp{{Kind: BlockOpen, BlockKind: BlockAlt, Label: "accepted"}}},
		{"enter nested", nil, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: BlockOpen, BlockKind: BlockAlt, Label: "declined"}, {Kind: BlockOpen, BlockKind: BlockOpt, Label: "retry fails"}}},
		{"else switch", []BranchRef{alt0}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockElse, BlockKind: BlockAlt, Label: "declined"}}},
		{"nested exit", []BranchRef{alt1, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}}},
		{"full exit", []BranchRef{alt1, opt}, nil,
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}, {Kind: BlockClose, BlockKind: BlockAlt, Label: "declined"}}},
		{"else switch from inside a nested block", []BranchRef{alt0, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}, {Kind: BlockElse, BlockKind: BlockAlt, Label: "declined"}}},
		{"else switch into a nested block", []BranchRef{alt0}, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: BlockElse, BlockKind: BlockAlt, Label: "declined"}, {Kind: BlockOpen, BlockKind: BlockOpt, Label: "retry fails"}}},
		{"else switch at depth 1", []BranchRef{alt1, in0}, []BranchRef{alt1, in1},
			[]BlockOp{{Kind: BlockElse, BlockKind: BlockAlt, Label: "slow"}}},
		{"else switch at depth 1 from deeper", []BranchRef{alt1, in0, opt}, []BranchRef{alt1, in1},
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}, {Kind: BlockElse, BlockKind: BlockAlt, Label: "slow"}}},
		{"prev is a proper prefix of next", []BranchRef{alt1}, []BranchRef{alt1, in0, opt},
			[]BlockOp{{Kind: BlockOpen, BlockKind: BlockAlt, Label: "fast"}, {Kind: BlockOpen, BlockKind: BlockOpt, Label: "retry fails"}}},
		{"sibling blocks", []BranchRef{opt}, []BranchRef{sib},
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}, {Kind: BlockOpen, BlockKind: BlockOpt, Label: "later"}}},
		{"sibling blocks inside a branch", []BranchRef{alt1, opt}, []BranchRef{alt1, sib},
			[]BlockOp{{Kind: BlockClose, BlockKind: BlockOpt, Label: "retry fails"}, {Kind: BlockOpen, BlockKind: BlockOpt, Label: "later"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BlockOps(c.prev, c.next); !reflect.DeepEqual(got, c.want) {
				t.Errorf("BlockOps = %+v, want %+v", got, c.want)
			}
		})
	}
}
