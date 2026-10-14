package lsp_test

import (
	"encoding/json"
	"strings"
	"testing"
)

const journeyParSrc = `actor user Seeker
domain ad {
    frontendplatform
    graphql
    atlas
    media
    adcore
    Dapi
}
use_case "Seeker reads an advert" {
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        par "field resolvers"
            graphql asks atlas for the SEO title  [postSeoadvertsbatch]
            graphql asks atlas for the targeting data  [postAdverttargetingbatch]
            graphql asks atlas where the advert came from  [postAdsourcebatch]
        loop "for each image, until all are signed"
            graphql asks media to sign the image URL  [signUrl]
        par "the advert"
            graphql asks adcore for the advert  [GET /ads]
        and "the statistics"
            graphql asks Dapi for the page statistics  [GET /stats]
}
`

const journeyNotesSrc = `actor user Seller
domain payments {
    frontendplatform
    graphql
    paymentgateway
}
use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback; a lost callback leaves the invoice open"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session, not a service token"
        graphql asks paymentgateway to start the payment  [POST /payments]
        finding "no idempotency key: a client retry starts a second payment"
}
`

const journeyReceivesSrc = `domain search {
    graphql
    atlas
}
use_case "Reindex" {
    when graphql receives a reindex sweep  [reindexAdverts]
        graphql asks atlas for every advert  [getAll]
}
`

type outlineSymKind struct {
	Name     string           `json:"name"`
	Kind     int              `json:"kind"`
	Children []outlineSymKind `json:"children"`
}

func outlineWithKinds(t *testing.T, src string) []outlineSymKind {
	t.Helper()
	c, _ := newBranchClient(t, src)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []outlineSymKind
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	return syms
}

func useCaseSym(t *testing.T, syms []outlineSym) outlineSym {
	t.Helper()
	for _, s := range syms {
		if s.Detail == "use_case" {
			return s
		}
	}
	t.Fatalf("no use case symbol in %+v", syms)
	return outlineSym{}
}

func names(syms []outlineSym) []string {
	var out []string
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

func TestJourneyOutline_ParAndLoop(t *testing.T) {
	uc := useCaseSym(t, outlineOf(t, journeyParSrc))
	if len(uc.Children) != 1 {
		t.Fatalf("use case children = %v", names(uc.Children))
	}
	got := strings.Join(names(uc.Children[0].Children), " / ")
	want := "par: field resolvers / loop: for each image, until all are signed / par: the advert | the statistics"
	if got != want {
		t.Errorf("scenario children = %q, want %q", got, want)
	}
}

func TestJourneyOutline_Findings(t *testing.T) {
	const symbolKindEvent = 24
	syms := outlineWithKinds(t, journeyNotesSrc)
	var uc *outlineSymKind
	for i := range syms {
		if syms[i].Name == "Seller pays an invoice" {
			uc = &syms[i]
		}
	}
	if uc == nil || len(uc.Children) != 2 {
		t.Fatalf("use case = %+v, want a finding and a scenario", uc)
	}
	first := uc.Children[0]
	if first.Name != "finding: nothing retries a failed callback; a lost callback leaves th" || first.Kind != symbolKindEvent {
		t.Errorf("first child = %q kind %d", first.Name, first.Kind)
	}
	sc := uc.Children[1]
	found := false
	var walk func([]outlineSymKind)
	walk = func(ss []outlineSymKind) {
		for _, s := range ss {
			if strings.HasPrefix(s.Name, "note:") {
				t.Errorf("notes stay out of the outline: %q", s.Name)
			}
			if s.Name == "finding: no idempotency key: a client retry starts a second payment" && s.Kind == symbolKindEvent {
				found = true
			}
			walk(s.Children)
		}
	}
	walk(syms)
	if !found {
		t.Errorf("scenario finding missing: %+v", sc.Children)
	}
}

func TestJourneyFolding_ParAndLoop(t *testing.T) {
	c, _ := newBranchClient(t, journeyParSrc)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var ranges []struct {
		StartLine int `json:"startLine"`
		EndLine   int `json:"endLine"`
	}
	if err := json.Unmarshal(c.call("textDocument/foldingRange", p), &ranges); err != nil {
		t.Fatal(err)
	}
	got := map[[2]int]bool{}
	for _, r := range ranges {
		got[[2]int{r.StartLine, r.EndLine}] = true
	}
	for _, want := range [][2]int{{12, 15}, {16, 17}, {18, 21}} {
		if !got[want] {
			t.Errorf("missing fold %v in %v", want, ranges)
		}
	}
}

func hoverText(c *branchClient, line, char int) string {
	var h struct {
		Contents struct{ Value string } `json:"contents"`
	}
	json.Unmarshal(c.at("textDocument/hover", line, char), &h) //nolint:errcheck
	return h.Contents.Value
}

func TestJourneyHover_Blocks(t *testing.T) {
	c, _ := newBranchClient(t, journeyParSrc)
	for _, tc := range []struct {
		line int
		want string
	}{
		{12, "par block: 3 branches; each step runs concurrently"},
		{16, "loop block: 1 branch"},
		{18, "par block: 2 branches"},
	} {
		if got := hoverText(c, tc.line, 9); got != tc.want {
			t.Errorf("hover line %d = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestJourneyHover_Keywords(t *testing.T) {
	c, _ := newBranchClient(t, journeyNotesSrc)
	if got, want := hoverText(c, 10, 9), "note — drawn over the previous step"; got != want {
		t.Errorf("note hover = %q, want %q", got, want)
	}
	if got, want := hoverText(c, 7, 5), "finding — drawn highlighted and exported to CraftDoc (`Step.Note`)"; got != want {
		t.Errorf("finding hover = %q, want %q", got, want)
	}
	r, _ := newBranchClient(t, journeyReceivesSrc)
	if got, want := hoverText(r, 5, 18), "callee-side trigger: the caller is not observed; drawn as a found message"; got != want {
		t.Errorf("receives hover = %q, want %q", got, want)
	}
}

func completionAt(t *testing.T, src string, line, char int) []completionItem {
	t.Helper()
	c, _ := newBranchClient(t, src)
	var list struct {
		Items []completionItem `json:"items"`
	}
	if err := json.Unmarshal(c.at("textDocument/completion", line, char), &list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func requireSnippet(t *testing.T, items []completionItem, label, text string) {
	t.Helper()
	for _, it := range items {
		if it.Label == label {
			if it.InsertText != text || it.InsertTextFormat != 2 {
				t.Errorf("%s = %q (format %v), want snippet %q", label, it.InsertText, it.InsertTextFormat, text)
			}
			return
		}
	}
	t.Errorf("missing %q in %v", label, labelList(items))
}

func TestJourneyCompletion_StepKeywords(t *testing.T) {
	// A blank line after a step at scenario level: no open par.
	plain := strings.Replace(journeyParSrc, "        par \"field resolvers\"", "        \n        par \"field resolvers\"", 1)
	items := completionAt(t, plain, 12, 8)
	requireSnippet(t, items, "par", `par "${1:label}"`)
	requireSnippet(t, items, "loop", `loop "${1:condition}"`)
	requireSnippet(t, items, "note", `note "${1:text}"`)
	requireSnippet(t, items, "finding", `finding "${1:text}"`)
	requireSnippet(t, items, "alt", `alt "${1:condition}"`)
	if hasLabel(items, "and") {
		t.Errorf("and offered outside an open par: %v", labelList(items))
	}
	// A blank line right after a par body line: `and` continues the par.
	inPar := strings.Replace(journeyParSrc, "        and \"the statistics\"", "        \n        and \"the statistics\"", 1)
	requireSnippet(t, completionAt(t, inPar, 20, 8), "and", `and "${1:label}"`)
}

func TestJourneyCompletion_TriggerVerbAndSchedule(t *testing.T) {
	src := "domain search {\n    graphql\n}\nuse_case \"R\" {\n    when graphql \n        graphql does it\n}\n"
	items := completionAt(t, src, 4, 17)
	for _, l := range []string{"listens", "receives"} {
		if !hasLabel(items, l) {
			t.Errorf("missing %q after `when graphql `: %v", l, labelList(items))
		}
	}
	tail := "domain billing {\n    x\n}\nuse_case \"R\" {\n    when x listens store.a.inserted \n        x does it\n}\n"
	items = completionAt(t, tail, 4, 36)
	requireSnippet(t, items, "cron", `cron "${1:* * * * *}"`)
	requireSnippet(t, items, "every", `every "${1:5m}"`)
}

func semanticTokensAt(t *testing.T, src string) map[[2]uint32]uint32 {
	t.Helper()
	c, _ := newBranchClient(t, src)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(c.call("textDocument/semanticTokens/full", p), &result); err != nil {
		t.Fatal(err)
	}
	out := map[[2]uint32]uint32{}
	var line, char uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		if result.Data[i] != 0 {
			char = 0
		}
		line += result.Data[i]
		char += result.Data[i+1]
		out[[2]uint32{line, char}] = result.Data[i+3]
	}
	return out
}

func TestJourneySemanticTokens(t *testing.T) {
	const flowKeyword, regularString = 8, 32
	check := func(toks map[[2]uint32]uint32, line, char, want uint32, what string) {
		t.Helper()
		if got, ok := toks[[2]uint32{line, char}]; !ok || got != want {
			t.Errorf("%s at %d:%d = %d (present %v), want %d", what, line, char, got, ok, want)
		}
	}
	par := semanticTokensAt(t, journeyParSrc)
	check(par, 12, 8, flowKeyword, "par")
	check(par, 16, 8, flowKeyword, "loop")
	check(par, 20, 8, flowKeyword, "and")
	notes := semanticTokensAt(t, journeyNotesSrc)
	check(notes, 7, 4, flowKeyword, "finding")
	check(notes, 10, 8, flowKeyword, "note")
	check(notes, 10, 13, regularString, "note text")
	rec := semanticTokensAt(t, journeyReceivesSrc)
	check(rec, 5, 17, flowKeyword, "receives")
}

const journeySurfaceSrc = `actors {
    user Advisor
    user Seeker
}
domain obido {
    advisorapp
    concierge
}
surface concierge {
    callers: Advisor
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair plus a bidder identity header"
    finding "a read-only credential is refused only at persistence, after the write ran"
    query the seeker list, unpaged, newest first, filtered by a typed status  [getUserList]
    query the seeker audit trail                                             [getUserLogs]
    command delete a seeker's account                                        [deleteAccount]
}
use_case "Advisor works a task" {
    when Advisor opens the task list
        advisorapp asks concierge for the seekers  [getUserList]
}
use_case "Seeker removes their concierge account" {
    when concierge receives an account deletion  [deleteAccount]
        concierge forgets the seeker
}
`

const journeyStoreSrc = `actors {
    system CRON
    system Siren
}
domain billing {
    invoicing
    smsdispatch
}
services {
    messaging {
        contexts: invoicing, smsdispatch
        data-stores: sms_queue
    }
}
use_case "Invoicing queues a payment reminder" {
    when cron "0 9 * * 1-5"
        invoicing finds the invoices due in three days
        invoicing notifies store.sms_queue.inserted
}
use_case "The queued reminder is sent" {
    when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"
        smsdispatch claims the unsent rows
        smsdispatch asks Siren to send the SMS  [sendSmsNotificationEvent]
}
`

func TestSurfaceOutlineAndFolding(t *testing.T) {
	const kindInterface, kindMethod = 11, 6
	c, _ := newBranchClient(t, journeySurfaceSrc)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []struct {
		Name     string `json:"name"`
		Kind     int    `json:"kind"`
		Detail   string `json:"detail"`
		Children []struct {
			Name   string `json:"name"`
			Kind   int    `json:"kind"`
			Detail string `json:"detail"`
		} `json:"children"`
	}
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range syms {
		if s.Detail != "surface" {
			continue
		}
		found = true
		if s.Name != "surface concierge" || s.Kind != kindInterface {
			t.Errorf("surface symbol = %q kind %d", s.Name, s.Kind)
		}
		var got []string
		for _, ch := range s.Children {
			if ch.Kind != kindMethod {
				t.Errorf("%s kind = %d, want Method", ch.Name, ch.Kind)
			}
			got = append(got, ch.Name+"/"+ch.Detail)
		}
		if g := strings.Join(got, " "); g != "getUserList/query getUserLogs/query deleteAccount/command" {
			t.Errorf("surface children = %s", g)
		}
	}
	if !found {
		t.Fatalf("no surface symbol in %+v", syms)
	}
	var ranges []struct {
		StartLine int `json:"startLine"`
		EndLine   int `json:"endLine"`
	}
	if err := json.Unmarshal(c.call("textDocument/foldingRange", p), &ranges); err != nil {
		t.Fatal(err)
	}
	folded := false
	for _, r := range ranges {
		folded = folded || (r.StartLine == 8 && r.EndLine == 16)
	}
	if !folded {
		t.Errorf("no fold over the surface block (8-16): %v", ranges)
	}
}

func TestSurfaceHoverAndDefinition(t *testing.T) {
	c, _ := newBranchClient(t, journeySurfaceSrc)
	if got, want := hoverText(c, 8, 10), "surface of concierge: 3 operations (2 query, 1 command); callers: Advisor"; got != want {
		t.Errorf("subject hover = %q, want %q", got, want)
	}
	if got, want := hoverText(c, 19, 54), "query: the seeker list, unpaged, newest first, filtered by a typed status"; got != want {
		t.Errorf("bracket hover = %q, want %q", got, want)
	}
	var locs []struct {
		Range struct {
			Start struct{ Line, Character int } `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(c.at("textDocument/definition", 19, 54), &locs); err != nil {
		t.Fatal(err)
	}
	if len(locs) != 1 || locs[0].Range.Start.Line != 13 {
		t.Errorf("definition = %+v, want the getUserList line (13)", locs)
	}
}

func TestSurfaceCompletion(t *testing.T) {
	closed := strings.Replace(journeySurfaceSrc, "[getUserList]\n}", "[getUserList]\n        advisorapp asks concierge for x  []\n}", 1)
	unclosed := strings.Replace(journeySurfaceSrc, "[getUserList]\n}", "[getUserList]\n        advisorapp asks concierge for x  [\n}", 1)
	for name, src := range map[string]string{"closed": closed, "unclosed": unclosed} {
		items := completionAt(t, src, 20, 42)
		for _, op := range []string{"getUserList", "getUserLogs", "deleteAccount"} {
			if !hasLabel(items, op) {
				t.Errorf("%s: missing %q in %v", name, op, labelList(items))
			}
		}
	}

	top := completionAt(t, journeySurfaceSrc, 25, 0)
	requireSnippet(t, top, "surface", "surface ${1:boundary} {\n    callers: ${2:unknown}\n    query ${3:what it answers}  [${4:operation}]\n}")
	// A blank line before a surface is its leading trivia, not its body.
	before := completionAt(t, strings.Replace(journeySurfaceSrc, "}\nsurface", "}\n\nsurface", 1), 8, 0)
	if hasLabel(before, "query") {
		t.Errorf("a blank line before a surface offers its body keywords: %v", labelList(before))
	}

	body := strings.Replace(journeySurfaceSrc, "    callers: Advisor\n", "    callers: Advisor\n    \n", 1)
	items := completionAt(t, body, 10, 4)
	for _, l := range []string{"query", "command", "callers:", "spec:", "note", "finding"} {
		if !hasLabel(items, l) {
			t.Errorf("surface body: missing %q in %v", l, labelList(items))
		}
	}
	if hasLabel(items, "when") {
		t.Errorf("surface body offers use-case keywords: %v", labelList(items))
	}
}

func TestStoreRefHoverDefinitionCompletion(t *testing.T) {
	c, _ := newBranchClient(t, journeyStoreSrc)
	want := "store hand-off: sms_queue, change inserted; written by invoicing; polled by smsdispatch"
	if got := hoverText(c, 17, 30); got != want {
		t.Errorf("notifies hover = %q, want %q", got, want)
	}
	if got := hoverText(c, 20, 32); got != want {
		t.Errorf("listens hover = %q, want %q", got, want)
	}
	var locs []struct {
		Range struct {
			Start struct{ Line, Character int } `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(c.at("textDocument/definition", 17, 30), &locs); err != nil {
		t.Fatal(err)
	}
	if len(locs) != 1 || locs[0].Range.Start.Line != 11 || locs[0].Range.Start.Character != 21 {
		t.Errorf("definition = %+v, want the data-stores entry (11:21)", locs)
	}

	src := strings.Replace(journeyStoreSrc, "        invoicing notifies store.sms_queue.inserted\n",
		"        invoicing notifies store.sms_queue.inserted\n        invoicing notifies store.\n", 1)
	items := completionAt(t, src, 18, 33)
	if !hasLabel(items, "sms_queue") {
		t.Errorf("missing sms_queue after `notifies store.`: %v", labelList(items))
	}
}

func TestSurfaceHover_ServiceSurfaceCoversItsContexts(t *testing.T) {
	src := "domain obido {\n    advisorapp\n    concierge\n}\nservices {\n    obidozoe {\n        contexts: concierge\n    }\n}\n" +
		"surface obidozoe {\n    query the seeker list  [getUserList]\n}\n" +
		"use_case \"U\" {\n    when Advisor opens the list\n        advisorapp asks concierge for the seekers  [getUserList]\n}\n"
	c, _ := newBranchClient(t, src)
	if got, want := hoverText(c, 14, 54), "query: the seeker list"; got != want {
		t.Errorf("bracket hover = %q, want %q", got, want)
	}
}
