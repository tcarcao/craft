package lsp_test

import (
	"encoding/json"
	"testing"
)

const rosterLSPSrc = `actors {
    scheduler Ticker
    robot Bot
}
domain partner {
    cooker
    seshat
}
use_case "Cook" {
    tags {
        sites: otodompl
    }
    when Ticker fires the cook
        cooker notifies sns.cooked
        cooker returns stream the parts
}
use_case "Ledger" {
    when seshat listens sns.cooked, sns.a-changed,
            sns.b-changed, sns.c-changed
        seshat asks cooker to replay  [inferred replayAll, replayOne]
        seshat asks cooker for rows  [SQL ledger rows]
}
`

func TestHover_V224(t *testing.T) {
	c, _ := newBranchClient(t, rosterLSPSrc)
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{1, 6, "scheduler: a clock that starts work (cron, an in-process ticker); drawn as a boundary"},
		{2, 6, "robot: not a recognised kind; drawn as a person"},
		{10, 9, "scope tag: printed in the sequence title; the hub filters and checks its values"},
		{17, 17, "listens to 4 events; publishers: cooker (1 of 4 events)"},
		{17, 10, "listens to 4 events; publishers: cooker (1 of 4 events)"},
		{17, 26, "sns.cooked: published by cooker"},
		{18, 14, "sns.b-changed: no publisher in the model"},
		{14, 23, "streamed reply: drawn in a `loop stream` box; answers no request in the model"},
		{19, 39, "inferred hop: drawn grey"},
		{19, 49, "2 operations: replayAll, replayOne"},
		{20, 39, "direct use of another party's tables: ledger, rows"},
	} {
		if got := hoverText(c, tc.line, tc.char); got != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, got, tc.want)
		}
	}
}

const streamOneOfLSPSrc = `actors {
    user Seller
    system AiAssistant
    tool OperatorTool
}
domain chat {
    graphql
    outbox
}
use_case "Ask" {
    when Seller asks a question
        graphql asks AiAssistant to stream the reply  [POST /chat]
        AiAssistant returns stream the reply
        outbox notifies one of x.Ended, y.Ended
        graphql asks AiAssistant to upload  [ADDR "https://up.example.com/x"]
}
use_case "Replay" {
    when graphql receives a replay  [inferred replay]
        graphql does it
}
`

func TestHover_V224StreamOneOfAddr(t *testing.T) {
	c, _ := newBranchClient(t, streamOneOfLSPSrc)
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{3, 5, "tool: a CLI, script or client a person runs; it, not the person, makes the calls; drawn as a boundary"},
		{1, 5, "user: a person; drawn as a person"},
		{12, 30, "streamed reply: drawn in a `loop stream` box; answers graphql asks AiAssistant to stream the reply"},
		{13, 25, "notifies one of: one event per run, chosen at run time"},
		{13, 28, "notifies one of: one event per run, chosen at run time"},
		{14, 46, "egress to an address with no operation id; the hub does not resolve it"},
		{17, 38, "inferred: an ordinary payload word here; it marks an inferred hop only at the head of an action's bracket"},
	} {
		if got := hoverText(c, tc.line, tc.char); got != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, got, tc.want)
		}
	}
}

func TestDefinition_ListensItemGoesToItsPublishers(t *testing.T) {
	c, _ := newBranchClient(t, rosterLSPSrc)
	var locs []struct {
		Range struct{ Start struct{ Line int } } `json:"range"`
	}
	json.Unmarshal(c.at("textDocument/definition", 17, 26), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 13 {
		t.Errorf("definition = %+v (want the `cooker notifies sns.cooked` line)", locs)
	}
}

const surfaceListLSPSrc = `actors {
    user Advisor
}
domain obido {
    advisorapp
    concierge
}
surface concierge {
    query the seeker list  [getUserList]
    query the seeker audit trail  [getUserLogs]
}
use_case "Advisor works a task" {
    when Advisor opens the task list
        advisorapp asks concierge for the seekers and their trail  [getUserList, getUserLogs]
}
`

func TestBracketItems_SurfaceHoverAndDefinitionPerItem(t *testing.T) {
	c, _ := newBranchClient(t, surfaceListLSPSrc)
	want := "2 operations: getUserList, getUserLogs\ngetUserList: query: the seeker list\ngetUserLogs: query: the seeker audit trail"
	if got := hoverText(c, 13, 70); got != want {
		t.Errorf("bracket hover = %q, want %q", got, want)
	}
	var locs []struct {
		Range struct{ Start struct{ Line int } } `json:"range"`
	}
	json.Unmarshal(c.at("textDocument/definition", 13, 84), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 9 {
		t.Errorf("definition of the second item = %+v, want the getUserLogs line (9)", locs)
	}
	locs = nil
	json.Unmarshal(c.at("textDocument/definition", 13, 70), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 8 {
		t.Errorf("definition of the first item = %+v, want the getUserList line (8)", locs)
	}
}

func TestSemanticTokens_V224(t *testing.T) {
	const flowKeyword, comma = 8, 36
	toks := semanticTokensAt(t, rosterLSPSrc)
	for _, tc := range []struct{ line, char, want uint32 }{
		{14, 23, flowKeyword}, // stream
		{19, 39, flowKeyword}, // inferred
		{17, 34, comma},
	} {
		if got := toks[[2]uint32{tc.line, tc.char}]; got != tc.want {
			t.Errorf("%d:%d = %d, want %d", tc.line, tc.char, got, tc.want)
		}
	}
	oneOf := semanticTokensAt(t, streamOneOfLSPSrc)
	for _, pos := range [][2]uint32{{13, 24}, {13, 28}} { // one, of
		if got := oneOf[pos]; got != flowKeyword {
			t.Errorf("%v = %d, want %d", pos, got, flowKeyword)
		}
	}
}

func TestCompletion_V224(t *testing.T) {
	items := completionAt(t, "actors {\n    \n}\n", 1, 4)
	for _, k := range []string{"user", "system", "service", "scheduler", "tool", "infra"} {
		if !hasLabel(items, k) {
			t.Errorf("actors block lacks %q: %v", k, labelList(items))
		}
	}
	items = completionAt(t, "actor \n", 0, 6)
	if !hasLabel(items, "scheduler") {
		t.Errorf("after `actor ` lacks scheduler: %v", labelList(items))
	}
	items = completionAt(t, rosterLSPSrc+"use_case \"N\" {\n    when seshat listens sns.cooked, \n        seshat does it\n}\n", 23, 36)
	if !hasLabel(items, "sns.cooked") {
		t.Errorf("listens list lacks published events: %v", labelList(items))
	}
	// A continuation line of a list offers the events too.
	items = completionAt(t, rosterLSPSrc+"use_case \"N\" {\n    when seshat listens sns.a-changed,\n            \n        seshat does it\n}\n", 24, 12)
	if !hasLabel(items, "sns.cooked") {
		t.Errorf("listens continuation lacks published events: %v", labelList(items))
	}
	items = completionAt(t, "use_case \"N\" {\n    when O does\n        x returns \n}\n", 2, 18)
	if !hasLabel(items, "stream") {
		t.Errorf("after returns: %v", labelList(items))
	}
	requireSnippet(t, completionAt(t, "use_case \"N\" {\n    when O does\n        x notifies \n}\n", 2, 19), "one of", "one of ${1:event}, ${2:event}")
	items = completionAt(t, "use_case \"N\" {\n    when O does\n        x asks y for it  []\n}\n", 2, 26)
	for _, l := range []string{"inferred", "ADDR", "SQL", "POST"} {
		if !hasLabel(items, l) {
			t.Errorf("bracket head lacks %q: %v", l, labelList(items))
		}
	}
	tags := "use_case \"A\" {\n    tags {\n        sites: imovirtualpt2, imovirtualpt\n    }\n    when O does\n        x does it\n}\nuse_case \"B\" {\n    tags {\n        sites: imovirtualpt2\n        \n    }\n    when O does\n        x does it\n}\n"
	items = completionAt(t, tags, 10, 8)
	requireSnippet(t, items, "sites", "sites: ${1:site}")
	requireSnippet(t, items, "envs", "envs: ${1:env}")
	withC := tags + "use_case \"C\" {\n    tags {\n        sites: \n    }\n    when O does\n        x does it\n}\n"
	items = completionAt(t, withC, 17, 15)
	if len(items) != 2 || items[0].Label != "imovirtualpt2" || items[1].Label != "imovirtualpt" {
		t.Errorf("site values, most frequent first: %v", labelList(items))
	}
}

func TestCompletion_BracketItemOffersTheRemainingSurfaceOperations(t *testing.T) {
	for name, line := range map[string]string{
		"closed":   "        advisorapp asks concierge for x  [getUserList, ]\n",
		"unclosed": "        advisorapp asks concierge for x  [getUserList, \n",
	} {
		src := surfaceListLSPSrc[:len(surfaceListLSPSrc)-2] + line + "}\n"
		items := completionAt(t, src, 14, 55)
		if !hasLabel(items, "getUserLogs") || hasLabel(items, "getUserList") {
			t.Errorf("%s: want getUserLogs only, got %v", name, labelList(items))
		}
	}
}
