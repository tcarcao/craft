package view

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
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
            note "x"
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
        finding "y"
    scenario "phone order"
    when Customer phones in an order
        alt "x"
            Billing asks Ordering to seventh
        else "y"
        Ordering asks Billing to eighth
    scenario "batch order"
    when Customer uploads a batch
        note "z"
        par "fan-out"
            Ordering asks Billing to ninth
            Ordering asks Billing to tenth
        loop "for each order"
            par "a"
                Ordering asks Billing to eleventh
            and "b"
                opt "o"
                    Billing asks Ordering to twelfth
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
			if m.Scenario == title && m.Kind != MessageNone && m.Kind != MessageStepNote && m.Kind != MessageFinding {
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

// An opening opt draws the trigger inside its box and, for the path where it
// is skipped, again just before the first step after it. An actor that takes
// the first step itself is a note either way.
func TestSequenceOpeningOptTriggersBothPaths(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Customer places an order
        opt "with a voucher"
            Billing applies the voucher
        Ordering charges the card
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("test.craft", src)}).Doc
	seq := BuildSequence(doc, interaction.Derive(doc))
	var got []string
	for _, m := range seq.Sections[0].Messages {
		got = append(got, fmt.Sprintf("%s->%s %q in %d", m.From, m.To, m.Label, len(m.Branch)))
	}
	want := []string{
		`Customer->shop/Billing "places an order" in 1`,
		`shop/Billing->shop/Billing "applies the voucher" in 1`,
		`Customer->shop/Ordering "places an order" in 0`,
		`shop/Ordering->shop/Ordering "charges the card" in 0`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNotes_AnchorOverThePreviousMessage(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "U" {
    finding "use-case level"
    when Customer places an order
        note "after the trigger"
        Ordering asks Billing to charge
        alt "declined"
            Billing returns the refusal
        note "after the block"
        opt "audit"
            note "only a note"
    when "Order Placed"
        note "nothing drawn yet"
        Ordering records it
        note "after an internal step"
    when Billing listens "Invoice Sent"
        note "after an incoming event"
    when Customer opens the basket
        note "after what the actor does itself"
        Customer asks Ordering for the basket
        note "it says \"empty\""
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("t.craft", src)}).Doc
	seq := BuildSequence(doc, interaction.Derive(doc))
	type got struct {
		kind  MessageKind
		label string
		over  string
	}
	var notes []got
	for _, m := range seq.Sections[0].Messages {
		if m.Kind == MessageNone {
			t.Errorf("unexpected MessageNone: a branch holding only a note is not empty")
		}
		if m.Kind == MessageStepNote || m.Kind == MessageFinding {
			notes = append(notes, got{m.Kind, m.Label, strings.Join(m.Over, ",")})
		}
	}
	want := []got{
		{MessageFinding, "use-case level", ""},
		{MessageStepNote, "after the trigger", "Customer,shop/Ordering"},
		{MessageStepNote, "after the block", "shop/Billing,shop/Ordering"},
		{MessageStepNote, "only a note", "shop/Billing,shop/Ordering"},
		{MessageStepNote, "nothing drawn yet", ""},
		{MessageStepNote, "after an internal step", "shop/Ordering"},
		{MessageStepNote, "after an incoming event", "shop/Billing"},
		{MessageStepNote, "after what the actor does itself", "Customer"},
		{MessageStepNote, `it says "empty"`, "Customer,shop/Ordering"},
	}
	if !slices.Equal(notes, want) {
		t.Errorf("notes:\n%v\nwant:\n%v", notes, want)
	}
}

// A note before an opening opt does not hide the opt: the trigger is still
// drawn inside the opt's box and again for the skipped path.
func TestSequenceOpeningOptAfterALeadingNote(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Customer places an order
        note "vouchers are optional"
        opt "with a voucher"
            Billing applies the voucher
        Ordering charges the card
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("test.craft", src)}).Doc
	seq := BuildSequence(doc, interaction.Derive(doc))
	var got []string
	for _, m := range seq.Sections[0].Messages {
		got = append(got, fmt.Sprintf("%s %s->%s %q in %d", m.Kind, m.From, m.To, m.Label, len(m.Branch)))
	}
	want := []string{
		`step-note -> "vouchers are optional" in 0`,
		`call Customer->shop/Billing "places an order" in 1`,
		`self shop/Billing->shop/Billing "applies the voucher" in 1`,
		`call Customer->shop/Ordering "places an order" in 0`,
		`self shop/Ordering->shop/Ordering "charges the card" in 0`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An opening par draws one trigger before its box, to its first action, and
// one inside each branch that starts at another participant. Split branches
// after the first have a blank label.
func TestSequenceOpeningPar(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
    Shipping
}
use_case "Reads" {
    when Customer opens the page
        par "fan-out"
            Ordering asks Billing for a
            Ordering asks Shipping for b
            Shipping asks Billing for c
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("test.craft", src)}).Doc
	seq := BuildSequence(doc, interaction.Derive(doc))
	var got []string
	for _, m := range seq.Sections[0].Messages {
		var path []string
		for _, r := range m.Branch {
			path = append(path, fmt.Sprintf("%s:%q#%d", r.Kind, r.Label, r.Index))
		}
		got = append(got, fmt.Sprintf("%s->%s %q {%s}", m.From, m.To, m.Label, strings.Join(path, ">")))
	}
	want := []string{
		`Customer->shop/Ordering "opens the page" {}`,
		`shop/Ordering->shop/Billing "for a" {par:"fan-out"#0}`,
		`shop/Ordering->shop/Shipping "for b" {par:""#1}`,
		`Customer->shop/Shipping "opens the page" {par:""#2}`,
		`shop/Shipping->shop/Billing "for c" {par:""#2}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A split par's branches after the first carry a blank label, so writers
// print a bare separator.
func TestBlockOps_SplitPar(t *testing.T) {
	r0 := BranchRef{Block: "b", Kind: lang.BlockPar, Label: "x", Index: 0}
	r1 := BranchRef{Block: "b", Kind: lang.BlockPar, Label: "", Index: 1}
	r2 := BranchRef{Block: "b", Kind: lang.BlockPar, Label: "", Index: 2}
	refs := branchPath([]interaction.BranchRef{
		{Block: "b", Kind: lang.BlockPar, Label: "x", Index: 0, Split: true},
		{Block: "b", Kind: lang.BlockPar, Label: "x", Index: 1, Split: true},
		{Block: "b", Kind: lang.BlockPar, Label: "x", Index: 2, Split: true},
	})
	if !reflect.DeepEqual(refs, []BranchRef{r0, r1, r2}) {
		t.Errorf("branchPath = %+v", refs)
	}
	if got := BlockOps(nil, []BranchRef{r0}); !reflect.DeepEqual(got, []BlockOp{{Kind: BlockOpen, BlockKind: lang.BlockPar, Label: "x"}}) {
		t.Errorf("open = %+v", got)
	}
	if got := BlockOps([]BranchRef{r0}, []BranchRef{r1}); !reflect.DeepEqual(got, []BlockOp{{Kind: BlockElse, BlockKind: lang.BlockPar, Label: ""}}) {
		t.Errorf("else = %+v", got)
	}
	if got := BlockOps([]BranchRef{r2}, nil); !reflect.DeepEqual(got, []BlockOp{{Kind: BlockClose, BlockKind: lang.BlockPar, Label: ""}}) {
		t.Errorf("close = %+v", got)
	}
}
