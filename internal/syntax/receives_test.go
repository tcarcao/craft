package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestReceives_Trigger(t *testing.T) {
	for _, src := range []string{
		"use_case \"U\" {\n    when graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
		"use_case \"U\" {\n    when ad/graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
	} {
		g, _, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Fatalf("unexpected diags: %v", diags)
		}
		tr := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
		if tr.Kind() != lang.TriggerReceive {
			t.Fatalf("Kind = %s", tr.Kind())
		}
		if tr.Connector() != "a" || tr.PhraseText() != "reindex sweep" || tr.OpText() != "reindexAdverts" {
			t.Errorf("connector=%q phrase=%q op=%q", tr.Connector(), tr.PhraseText(), tr.OpText())
		}
		if tr.ContextName() == "" || tr.ActorName() != "" {
			t.Errorf("context=%q actor=%q", tr.ContextName(), tr.ActorName())
		}
		if got, want := tr.SourceText(), tr.ContextName()+" receives a reindex sweep [reindexAdverts]"; got != want {
			t.Errorf("SourceText = %q, want %q", got, want)
		}
	}
}

// A receives trigger without an operation bracket still parses; the phrase
// runs to the end of the line.
func TestReceives_TriggerWithoutOperation(t *testing.T) {
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when graphql receives the sweep\n        graphql returns the ids\n}")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tr := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
	if tr.Kind() != lang.TriggerReceive || tr.Connector() != "the" || tr.PhraseText() != "sweep" || tr.OpText() != "" {
		t.Errorf("kind=%s connector=%q phrase=%q op=%q", tr.Kind(), tr.Connector(), tr.PhraseText(), tr.OpText())
	}
}

func TestListens_ScheduleTail(t *testing.T) {
	src := "use_case \"U\" {\n    when smsdispatch listens store.sms_queue.inserted cron \"* 10 * * 1-5\"\n        smsdispatch claims the rows\n    when leadimporter listens \"LeadRequested\" every \"5m\"\n        leadimporter imports it\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	scs := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()
	k, e, ok := scs[0].Trigger().ListenSchedule()
	if !ok || k != lang.ScheduleCron || e != "* 10 * * 1-5" || scs[0].Trigger().EventRef() != "store.sms_queue.inserted" {
		t.Errorf("first: %v %q %v ref=%q", k, e, ok, scs[0].Trigger().EventRef())
	}
	k, e, ok = scs[1].Trigger().ListenSchedule()
	if !ok || k != lang.ScheduleEvery || e != "5m" || scs[1].Trigger().EventValue() != "LeadRequested" {
		t.Errorf("second: %v %q %v", k, e, ok)
	}
	if _, _, ok := scs[0].Trigger().Schedule(); ok {
		t.Error("Schedule() is the subject-slot form only; a listens tail must not make the trigger run as CRON")
	}
}
