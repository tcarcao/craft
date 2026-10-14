package analysis_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/lang"
)

const scheduleSrc = `actor system CRON
domain ops {
    Reports
}
use_case "Nightly" {
    when cron "0 7 * * 1-5"
        Reports builds the digest
    scenario "poll"
    when every "5m"
        Reports polls the feed
}
`

func TestProject_ScheduledTriggersCarryTheirSchedule(t *testing.T) {
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("a.craft", scheduleSrc)}).Doc
	scs := doc.UseCases[0].Scenarios
	if len(scs) != 2 {
		t.Fatalf("got %d scenarios, want 2", len(scs))
	}
	cases := []struct {
		kind lang.ScheduleKind
		expr string
		desc string
	}{
		{lang.ScheduleCron, "0 7 * * 1-5", `when cron "0 7 * * 1-5"`},
		{lang.ScheduleEvery, "5m", `when every "5m"`},
	}
	for i, c := range cases {
		tr := scs[i].Trigger
		if tr.Type != lang.TriggerExternal || tr.Actor != "CRON" || tr.Verb != "" {
			t.Errorf("[%d] type/actor/verb = %q/%q/%q", i, tr.Type, tr.Actor, tr.Verb)
		}
		if tr.Schedule != c.expr || tr.ScheduleKind != c.kind {
			t.Errorf("[%d] schedule = %q (%q), want %q (%q)", i, tr.Schedule, tr.ScheduleKind, c.expr, c.kind)
		}
		if tr.Description != c.desc {
			t.Errorf("[%d] description = %q, want %q", i, tr.Description, c.desc)
		}
		if tr.Subject == nil || tr.Subject.ID != "CRON" || tr.Subject.Kind != lang.ParticipantActor {
			t.Errorf("[%d] subject = %+v, want the declared CRON actor", i, tr.Subject)
		}
	}
	b, _ := json.Marshal(scs[0].Trigger)
	if !strings.Contains(string(b), `"schedule":"0 7 * * 1-5"`) || !strings.Contains(string(b), `"scheduleKind":"cron"`) {
		t.Errorf("json = %s", b)
	}
}

func TestProject_DeclaredCronActorIsNotUnused(t *testing.T) {
	res := analysis.Workspace([]analysis.FileResult{analysis.File("a.craft", scheduleSrc)})
	for _, d := range res.Diagnostics {
		if d.Code == "craft/lint/unused-actor" {
			t.Errorf("unexpected %s: %s", d.Code, d.Message)
		}
	}
}
