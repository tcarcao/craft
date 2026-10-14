package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

const cronDoc = "actor system CRON\ndomain ops {\n  Reports\n}\nuse_case \"U\" {\n  when cron \"0 7 * * *\"\n    Reports builds it\n}\n"

// The `cron` keyword is a reference to the CRON actor: definition, hover and
// semantic tokens resolve it like any actor written in the trigger slot.
func TestSchedule_CronKeywordResolvesToCRONActor(t *testing.T) {
	s, doc := openTwin(t, cronDoc)
	pos := protocol.Position{Line: 5, Character: 8} // inside `cron`
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: doc, Position: pos},
	})
	if err != nil || len(locs) != 1 || locs[0].Range.Start.Line != 0 {
		t.Fatalf("definition = %+v, %v; want the CRON declaration on line 0", locs, err)
	}
	h, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: doc, Position: pos},
	})
	if err != nil || h == nil || !strings.Contains(h.Contents.Value, "CRON") {
		t.Fatalf("hover = %+v, %v; want the CRON actor", h, err)
	}
}
