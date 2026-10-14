package sema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
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
	notesOnly := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      note \"nothing runs here yet\"\n    x asks y for a\n}"
	if n := lintCodes(t, notesOnly, "craft/lint/single-branch-par"); n != 1 {
		t.Errorf("notes only: %d warnings, want 1", n)
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

func TestLint_OperationNotInSurface(t *testing.T) {
	src := "domain d {\n  app\n  concierge\n}\nsurface concierge {\n  query the list  [getUserList]\n}\nuse_case \"U\" {\n  when O does\n    app asks concierge for the list  [getUserList]\n    app asks concierge to book  [addMortgagesMeeting]\n    app asks app to think  [anything]\n}"
	if n := lintCodes(t, src, "craft/lint/operation-not-in-surface"); n != 1 {
		t.Errorf("%d warnings, want 1 (addMortgagesMeeting)", n)
	}
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code != "craft/lint/operation-not-in-surface" {
			continue
		}
		if r := d.Range; r.Start.Line != 10 || r.Start.Character != 32 || r.End.Character != 53 {
			t.Errorf("range = %+v", r)
		}
		if !strings.Contains(d.Message, "`[addMortgagesMeeting]`") || !strings.Contains(d.Message, "`concierge` surface") {
			t.Errorf("message = %q", d.Message)
		}
	}
}

func TestLint_StoreWithoutName(t *testing.T) {
	src := "domain d {\n  a\n  b\n}\nuse_case \"U\" {\n  when O does\n    a notifies store.inserted\n    a notifies store.leads.inserted\n  when b listens store.deleted\n    b does it\n    b notifies store.\n}"
	var got []string
	for _, d := range analyzeSingleFileForTest(t, src) {
		switch d.Code {
		case "craft/lint/store-without-name":
			r := d.Range
			got = append(got, fmt.Sprintf("%d:%d-%d", r.Start.Line, r.Start.Character, r.End.Character))
			if d.Severity != "warning" || !strings.Contains(d.Message, "names no store") {
				t.Errorf("diag = %+v", d)
			}
		case "craft/lint/store-without-change":
			t.Errorf("unexpected %+v", d)
		}
	}
	if s := strings.Join(got, " "); s != "6:15-29 8:17-30 10:15-21" {
		t.Errorf("ranges = %s, want 6:15-29 8:17-30 10:15-21", s)
	}
}

func TestLint_RepeatedRequest(t *testing.T) {
	src := `domain web {
  frontendplatform
  mobile
  graphql
}
use_case "Autocomplete" {
  when Seeker types a query
    frontendplatform asks graphql for suggestions  [autocomplete]
    graphql returns the suggestions
    mobile asks graphql for suggestions  [autocomplete]
  when Seeker types again
    frontendplatform asks graphql for suggestions  [autocomplete]
    frontendplatform asks graphql for more  [autocomplete]
    mobile asks graphql for suggestions  [suggest]
    alt "on the web"
      frontendplatform asks graphql for x  [search]
    else "in the app"
      mobile asks graphql for x  [search]
}`
	if n := lintCodes(t, src, "craft/lint/repeated-request"); n != 1 {
		t.Errorf("%d warnings, want 1 (only the mobile line of the first scenario)", n)
	}
}

// A call chain is not two clients: when the second subject was asked earlier
// in the same list, it is relaying the request, so nothing is reported. And
// each (target, operation, subject) pair warns once: A, B, B gives one warning.
func TestLint_RepeatedRequest_SkipsCallChainsAndRepeats(t *testing.T) {
	src := `domain web {
  frontendplatform
  mobile
  graphql
  locationmapper
  sphere
}
use_case "Chain" {
  when Seeker opens a location
    graphql asks sphere for the locations  [getLocations]
    graphql asks locationmapper to map the location
    locationmapper asks sphere for the locations  [getLocations]
  when Seeker types a query
    frontendplatform asks graphql for suggestions  [autocomplete]
    mobile asks graphql for suggestions  [autocomplete]
    mobile asks graphql for suggestions  [autocomplete]
}`
	var lines []int
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/repeated-request" {
			lines = append(lines, d.Range.Start.Line)
		}
	}
	if len(lines) != 1 || lines[0] != 14 {
		t.Errorf("warnings on lines %v, want only [14] (the first mobile line)", lines)
	}
}

func TestLint_OperationNotInSurface_ServiceSurfaceCoversItsContexts(t *testing.T) {
	src := "domain d {\n  app\n  concierge\n  ledger\n}\nservices {\n  obidozoe {\n    contexts: concierge, ledger\n  }\n}\n" +
		"surface obidozoe {\n  query the list  [getUserList]\n}\nsurface ledger {\n  query the balances  [getBalances]\n}\n" +
		"use_case \"U\" {\n  when O does\n    app asks concierge for the list  [getUserList]\n    app asks concierge to book  [addMortgagesMeeting]\n    app asks ledger for the list  [getUserList]\n    app asks obidozoe for the list  [getUserList]\n}"
	var got []string
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/operation-not-in-surface" {
			got = append(got, fmt.Sprintf("%d %s", d.Range.Start.Line, d.Message))
		}
	}
	// concierge has no surface, so obidozoe's covers it; ledger has its own.
	if len(got) != 2 || !strings.Contains(got[0], "[addMortgagesMeeting]") || !strings.Contains(got[0], "`obidozoe` surface") ||
		!strings.Contains(got[1], "[getUserList]") || !strings.Contains(got[1], "`ledger` surface") {
		t.Errorf("warnings = %q", got)
	}
}

func TestLint_UnusedActorTreatsNewKindsLikeSystem(t *testing.T) {
	for _, kind := range []string{"scheduler", "tool", "infra"} {
		unused := "actors {\n  " + kind + " X\n}\n"
		used := unused + "use_case \"U\" {\n  when X starts it\n    a does b\n}\n"
		if n := lintCodes(t, unused, "craft/lint/unused-actor"); n != 1 {
			t.Errorf("%s unused: %d", kind, n)
		}
		if n := lintCodes(t, used, "craft/lint/unused-actor"); n != 0 {
			t.Errorf("%s used: %d", kind, n)
		}
	}
}

func TestLint_DuplicateEventInList(t *testing.T) {
	dup := "use_case \"U\" {\n  when s listens a.A, b.B, a.A\n    s does it\n}"
	if n := lintCodes(t, dup, "craft/lint/duplicate-event-in-list"); n != 1 {
		t.Errorf("dup: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when s listens a.A, b.B\n    s does it\n}", "craft/lint/duplicate-event-in-list"); n != 0 {
		t.Errorf("clean: %d", n)
	}
}

func TestLint_PerItemEventLints(t *testing.T) {
	src := "use_case \"P\" {\n  when O does\n    p notifies a.Charged\n}\nuse_case \"U\" {\n  when s listens a.Charged,\n      a.Refund\n    s does it\n}"
	diags := analyzeSingleFileForTest(t, src)
	var past []model.Diagnostic
	for _, d := range diags {
		if d.Code == "craft/lint/event-not-past-tense" {
			past = append(past, d)
		}
		if d.Code == "craft/lint/dead-event" {
			t.Errorf("a.Charged is consumed by the list: %v", d)
		}
	}
	if len(past) != 1 || past[0].Range.Start.Line != 6 || past[0].Range.Start.Character != 6 {
		t.Errorf("past-tense = %v (want one, on the continuation line)", past)
	}
}

func TestLint_OperationNotInSurfaceChecksEachItem(t *testing.T) {
	src := "domain ad {\n  atlas\n  partner\n}\nsurface atlas {\n  callers: partner\n  command save the advert  [postAdvert]\n  command delete it  [deleteAdvert]\n}\nuse_case \"U\" {\n  when partner listens x.Y\n    partner asks atlas to save or delete the advert  [postAdvert, putAdvert, deleteAdvert]\n}"
	var got []model.Diagnostic
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/operation-not-in-surface" {
			got = append(got, d)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0].Message, "[putAdvert]") {
		t.Fatalf("got %v", got)
	}
	// "    partner asks atlas to save or delete the advert  [" is 54 columns; putAdvert starts after "postAdvert, ".
	if got[0].Range.Start.Character != 66 || got[0].Range.End.Character != 75 {
		t.Errorf("range = %+v", got[0].Range)
	}
}

func TestLint_OneOfSingleEvent(t *testing.T) {
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended\n}", "craft/lint/one-of-single-event"); n != 1 {
		t.Errorf("single: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended, b.Ended\n}", "craft/lint/one-of-single-event"); n != 0 {
		t.Errorf("two: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended, b.Ended, a.Ended\n}", "craft/lint/duplicate-event-in-list"); n != 1 {
		t.Errorf("duplicate: %d", n)
	}
}

func TestLint_OneOfItemsAreEachPublished(t *testing.T) {
	src := "use_case \"P\" {\n  when O does\n    x notifies one of a.Ended, b.Ended\n}\nuse_case \"L\" {\n  when y listens b.Ended\n    y does it\n}"
	var dead []string
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/dead-event" {
			dead = append(dead, d.Message)
		}
	}
	if len(dead) != 1 || !strings.Contains(dead[0], "a.Ended") {
		t.Errorf("dead = %v", dead)
	}
}

func TestLint_EgressOnInternalStep(t *testing.T) {
	bad := "use_case \"U\" {\n  when O does\n    o reads the tables  [SQL investment]\n    o returns the rows  [ADDR api.example.com]\n}"
	good := "use_case \"U\" {\n  when O does\n    o asks front for the tables  [SQL investment]\n}"
	if n := lintCodes(t, bad, "craft/lint/egress-on-internal-step"); n != 2 {
		t.Errorf("bad: %d", n)
	}
	if n := lintCodes(t, good, "craft/lint/egress-on-internal-step"); n != 0 {
		t.Errorf("good: %d", n)
	}
}

func TestLint_UnquotedAddress(t *testing.T) {
	cut := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR //cdn.example.com/x]\n}"
	whole := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR https://cdn.example.com/x]\n}"
	quoted := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR \"//cdn.example.com/x\"]\n}"
	if n := lintCodes(t, cut, "craft/lint/unquoted-address"); n != 1 {
		t.Errorf("cut: %d", n)
	}
	for _, src := range []string{whole, quoted} {
		if n := lintCodes(t, src, "craft/lint/unquoted-address"); n != 0 {
			t.Errorf("%d:\n%s", n, src)
		}
	}
}
