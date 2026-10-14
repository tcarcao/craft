package view

import (
	"reflect"
	"testing"
)

func TestBlockOps(t *testing.T) {
	alt0 := BranchRef{Block: "b1", Kind: "alt", Label: "accepted", Index: 0}
	alt1 := BranchRef{Block: "b1", Kind: "alt", Label: "declined", Index: 1}
	opt := BranchRef{Block: "b2", Kind: "opt", Label: "retry fails", Index: 0}
	sib := BranchRef{Block: "b3", Kind: "opt", Label: "later", Index: 0}
	cases := []struct {
		name       string
		prev, next []BranchRef
		want       []BlockOp
	}{
		{"none", nil, nil, nil},
		{"same branch", []BranchRef{alt0}, []BranchRef{alt0}, nil},
		{"enter", nil, []BranchRef{alt0},
			[]BlockOp{{Kind: "open", BlockKind: "alt", Label: "accepted"}}},
		{"enter nested", nil, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: "open", BlockKind: "alt", Label: "declined"}, {Kind: "open", BlockKind: "opt", Label: "retry fails"}}},
		{"else switch", []BranchRef{alt0}, []BranchRef{alt1},
			[]BlockOp{{Kind: "else", BlockKind: "alt", Label: "declined"}}},
		{"nested exit", []BranchRef{alt1, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: "close", BlockKind: "opt", Label: "retry fails"}}},
		{"full exit", []BranchRef{alt1, opt}, nil,
			[]BlockOp{{Kind: "close", BlockKind: "opt", Label: "retry fails"}, {Kind: "close", BlockKind: "alt", Label: "declined"}}},
		{"else switch from inside a nested block", []BranchRef{alt0, opt}, []BranchRef{alt1},
			[]BlockOp{{Kind: "close", BlockKind: "opt", Label: "retry fails"}, {Kind: "else", BlockKind: "alt", Label: "declined"}}},
		{"else switch into a nested block", []BranchRef{alt0}, []BranchRef{alt1, opt},
			[]BlockOp{{Kind: "else", BlockKind: "alt", Label: "declined"}, {Kind: "open", BlockKind: "opt", Label: "retry fails"}}},
		{"sibling blocks", []BranchRef{opt}, []BranchRef{sib},
			[]BlockOp{{Kind: "close", BlockKind: "opt", Label: "retry fails"}, {Kind: "open", BlockKind: "opt", Label: "later"}}},
		{"sibling blocks inside a branch", []BranchRef{alt1, opt}, []BranchRef{alt1, sib},
			[]BlockOp{{Kind: "close", BlockKind: "opt", Label: "retry fails"}, {Kind: "open", BlockKind: "opt", Label: "later"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BlockOps(c.prev, c.next); !reflect.DeepEqual(got, c.want) {
				t.Errorf("BlockOps = %+v, want %+v", got, c.want)
			}
		})
	}
}
