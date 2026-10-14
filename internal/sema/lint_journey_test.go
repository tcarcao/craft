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
