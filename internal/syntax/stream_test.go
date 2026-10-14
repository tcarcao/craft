package syntax_test

import "testing"

func TestReturnsStream(t *testing.T) {
	a := firstAction(t, "AiAssistant returns stream the reply as META, TOKEN and DONE events")
	// `the` is a connector word, outside the phrase, as for any return.
	if !a.IsStream() || a.DescriptionConnector() != "the" || a.PhraseText() != "reply as META, TOKEN and DONE events" || a.TargetName() != "" {
		t.Errorf("stream=%v connector=%q phrase=%q target=%q", a.IsStream(), a.DescriptionConnector(), a.PhraseText(), a.TargetName())
	}
	if got := a.SourceText(); got != "AiAssistant returns stream the reply as META, TOKEN and DONE events" {
		t.Errorf("SourceText = %q", got)
	}
	a = firstAction(t, "graphql returns stream to frontendplatform each event")
	if !a.IsStream() || a.TargetName() != "frontendplatform" || a.PhraseText() != "each event" {
		t.Errorf("to: stream=%v target=%q phrase=%q", a.IsStream(), a.TargetName(), a.PhraseText())
	}
	if got := a.SourceText(); got != "graphql returns stream to frontendplatform each event" {
		t.Errorf("SourceText = %q", got)
	}
	a = firstAction(t, "graphql returns stream with each event  [inferred]")
	if !a.IsStream() || a.DescriptionConnector() != "with" || a.PhraseText() != "each event" || !a.IsInferred() {
		t.Errorf("connector: stream=%v phrase=%q inferred=%v", a.IsStream(), a.PhraseText(), a.IsInferred())
	}
	if got := a.SourceText(); got != "graphql returns stream with each event [inferred]" {
		t.Errorf("SourceText = %q", got)
	}
	if a := firstAction(t, "graphql returns streams of data"); a.IsStream() {
		t.Error("`streams` is a phrase word")
	}
	if a := firstAction(t, "graphql returns to x stream"); a.IsStream() || a.PhraseText() != "stream" {
		t.Error("`stream` after the target is a phrase word")
	}
	if a := firstAction(t, "stream returns the data"); a.IsStream() || a.SubjectName() != "stream" {
		t.Error("`stream` stays a valid subject")
	}
}
