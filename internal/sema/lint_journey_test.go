package sema

import (
	"fmt"
	"strings"
	"testing"
)

func lintCodes(t *testing.T, src, code string) int {
	t.Helper()
	n := 0
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestLint_SingleBranchPar(t *testing.T) {
	one := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      x asks y for a\n}"
	two := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      x asks y for a\n      x asks y for b\n}"
	explicit := "use_case \"U\" {\n  when O does\n    par \"a\"\n      x asks y for a\n    and \"b\"\n      x asks y for b\n}"
	nested := "use_case \"U\" {\n  when O does\n    loop \"each\"\n      par \"lookups\"\n        x asks y for a\n}"
	if n := lintCodes(t, one, "craft/lint/single-branch-par"); n != 1 {
		t.Errorf("one step: %d warnings, want 1", n)
	}
	if n := lintCodes(t, nested, "craft/lint/single-branch-par"); n != 1 {
		t.Errorf("nested one step: %d warnings, want 1", n)
	}
	for _, src := range []string{two, explicit} {
		if n := lintCodes(t, src, "craft/lint/single-branch-par"); n != 0 {
			t.Errorf("%d warnings, want 0:\n%s", n, src)
		}
	}
}

func TestLint_ReceivesOnActor(t *testing.T) {
	onActor := "actor user Seller\ndomain d {\n  graphql\n}\nuse_case \"U\" {\n  when Seller receives a sweep  [x]\n    graphql does it\n}"
	onContext := "domain d {\n  graphql\n}\nuse_case \"U\" {\n  when graphql receives a sweep  [x]\n    graphql does it\n}"
	if n := lintCodes(t, onActor, "craft/lint/receives-on-actor"); n != 1 {
		t.Errorf("on actor: %d, want 1", n)
	}
	if n := lintCodes(t, onContext, "craft/lint/receives-on-actor"); n != 0 {
		t.Errorf("on context: %d, want 0", n)
	}
}

func TestLint_ReceivesTriggersDifferingByOperationAreDistinguishable(t *testing.T) {
	src := "domain d {\n  graphql\n}\nuse_case \"U\" {\n  when graphql receives a call  [getA]\n    graphql does a\n  when graphql receives a call  [getB]\n    graphql does b\n}"
	if n := lintCodes(t, src, "craft/lint/indistinguishable-scenarios"); n != 0 {
		t.Errorf("%d indistinguishable-scenarios warnings, want 0", n)
	}
}

func TestLint_SingleBranchParIgnoresNotes(t *testing.T) {
	oneWithNote := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      x asks y for a\n      note \"cached\"\n}"
	if n := lintCodes(t, oneWithNote, "craft/lint/single-branch-par"); n != 1 {
		t.Errorf("one step and a note: %d warnings, want 1", n)
	}
}

func TestLint_StoreWithoutChange(t *testing.T) {
	src := "domain d {\n  a\n  b\n}\nuse_case \"U\" {\n  when O does\n    a notifies store.redis.leads.pending\n    a notifies store.leads.inserted\n  when b listens store.redis.leads.pending\n    b does it\n}"
	if n := lintCodes(t, src, "craft/lint/store-without-change"); n != 2 {
		t.Errorf("%d warnings, want 2 (the notifies and the listens of the change-less ref)", n)
	}
	var starts []string
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/store-without-change" {
			r := d.Range
			starts = append(starts, fmt.Sprintf("%d:%d-%d", r.Start.Line, r.Start.Character, r.End.Character))
		}
	}
	if got := strings.Join(starts, " "); got != "6:15-40 8:17-42" {
		t.Errorf("ranges = %s, want 6:15-40 8:17-42 (the ref itself)", got)
	}
}
