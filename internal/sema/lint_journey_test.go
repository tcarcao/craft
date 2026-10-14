package sema

import "testing"

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
