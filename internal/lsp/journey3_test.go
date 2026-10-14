package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tcarcao/craft/v2/internal/lsp"
)

// wsFile is one document opened in a workspace client.
type wsFile struct{ uri, text string }

// newWorkspaceClient starts a server initialised with initOptions (JSON, or
// "" for none) and rootURI (or "" for none), opens every file in order, and
// returns a client whose uri is the first file's.
func newWorkspaceClient(t *testing.T, rootURI, initOptions string, files ...wsFile) *branchClient {
	t.Helper()
	c, _ := startWorkspaceClient(t, rootURI, initOptions, files...)
	return c
}

// initializeWith returns the initialize result for initOptions.
func initializeWith(t *testing.T, initOptions string) json.RawMessage {
	t.Helper()
	_, res := startWorkspaceClient(t, "", initOptions, wsFile{"file:///a.craft", ""})
	return res
}

func startWorkspaceClient(t *testing.T, rootURI, initOptions string, files ...wsFile) (*branchClient, json.RawMessage) {
	t.Helper()
	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()
	ctx, cancel := context.WithTimeout(context.Background(), serverLifetime)
	t.Cleanup(func() { cancel(); testOut.Close() })
	go func() { defer serverOut.Close(); lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck

	c := &branchClient{t: t, out: testOut, br: bufio.NewReader(testIn), uri: files[0].uri}
	init := map[string]any{"processId": nil, "rootUri": nil, "capabilities": map[string]any{}}
	if rootURI != "" {
		init["rootUri"] = rootURI
	}
	if initOptions != "" {
		init["initializationOptions"] = json.RawMessage(initOptions)
	}
	p, _ := json.Marshal(init)
	res := c.call("initialize", p)
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck
	for _, f := range files {
		p, _ := json.Marshal(map[string]any{
			"textDocument": map[string]any{"uri": f.uri, "languageId": "craft", "version": 1, "text": f.text},
		})
		writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	}
	time.Sleep(300 * time.Millisecond)
	return c, res
}

// atURI sends a position request for uri.
func (c *branchClient) atURI(method, uri string, line, char int, extra map[string]any) json.RawMessage {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
	}
	for k, v := range extra {
		params[k] = v
	}
	p, _ := json.Marshal(params)
	return c.call(method, p)
}

type lspLocation struct {
	URI   string `json:"uri"`
	Range struct {
		Start struct{ Line, Character int } `json:"start"`
		End   struct{ Line, Character int } `json:"end"`
	} `json:"range"`
}

const linksRoot = "file:///ws"
const linksURI = "file:///ws/links.craft"

// linksLSPSrc is the links corpus file plus journey tags, a service with a
// repo, and two cited steps.
const linksLSPSrc = `actors {
    user Seller
}

domain seller {
    frontendplatform
    graphql
}

use_case "Seller asks the assistant a question" {
    when Seller asks the assistant a question
        includes "Site session preamble"
        frontendplatform asks graphql to stream the answer  [assistantStream]
}

use_case "Seller promotes an advert" {
    tags {
        journey: seller-promotes-an-advert
    }
    when Seller promotes an advert
        frontendplatform asks graphql for the promotion offers  [promotionOffers]
        //@ re-x src/offers.go:5-9 -- offers come from the catalogue
        continues "Seller pays at the checkout"
}

use_case "Site session preamble" {
    when Seller opens the site
        frontendplatform asks graphql for the session  [session]
        //@ src/session.go:3
}

use_case "Seller pays at the checkout" {
    tags {
        journey: seller-promotes-an-advert
    }
    when Seller opens the checkout
        frontendplatform asks graphql to start the payment  [startPayment]
}

services {
    Frontend {
        contexts: frontendplatform
        repo: re-frontend
    }
}
`

func linksClient(t *testing.T, initOptions string) *branchClient {
	return newWorkspaceClient(t, linksRoot, initOptions, wsFile{linksURI, linksLSPSrc})
}

func TestLinks_Outline(t *testing.T) {
	c := linksClient(t, "")
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []outlineSymKind
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	const module = 2
	found := map[string]int{}
	var walk func([]outlineSymKind)
	walk = func(ss []outlineSymKind) {
		for _, s := range ss {
			found[s.Name] = s.Kind
			walk(s.Children)
		}
	}
	walk(syms)
	for _, want := range []string{"includes: Site session preamble", "continues: Seller pays at the checkout"} {
		if k, ok := found[want]; !ok || k != module {
			t.Errorf("outline %q: kind %d present %v, want Module", want, k, ok)
		}
	}
}

func TestLinks_DefinitionAndReferences(t *testing.T) {
	c := linksClient(t, "")
	var locs []lspLocation
	json.Unmarshal(c.at("textDocument/definition", 22, 22), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].URI != linksURI || locs[0].Range.Start.Line != 31 || locs[0].Range.Start.Character != 9 {
		t.Errorf("definition on the continues title = %+v, want links.craft 31:9", locs)
	}
	locs = nil
	json.Unmarshal(c.at("textDocument/definition", 11, 20), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 25 {
		t.Errorf("definition on the includes title = %+v, want line 25", locs)
	}
	locs = nil
	json.Unmarshal(c.atURI("textDocument/references", linksURI, 31, 15, map[string]any{"context": map[string]any{"includeDeclaration": false}}), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 22 || locs[0].Range.Start.Character != 18 {
		t.Errorf("references to the checkout = %+v, want the continues title at 22:18", locs)
	}
	locs = nil
	json.Unmarshal(c.atURI("textDocument/references", linksURI, 25, 12, map[string]any{"context": map[string]any{"includeDeclaration": false}}), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 11 {
		t.Errorf("references to the preamble = %+v, want line 11", locs)
	}
}

func TestLinks_Hover(t *testing.T) {
	c := linksClient(t, "")
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{22, 22, "use case \"Seller pays at the checkout\" in links.craft\nfirst trigger: when Seller opens the checkout\njourney: seller-promotes-an-advert"},
		{11, 20, "use case \"Site session preamble\" in links.craft\nfirst trigger: when Seller opens the site"},
		{21, 12, "cites:\n- re-x src/offers.go:5-9 — offers come from the catalogue\nattached to: frontendplatform asks graphql for the promotion offers"},
		{28, 9, "cites:\n- src/session.go:3\nattached to: frontendplatform asks graphql for the session"},
	} {
		if got := hoverText(c, tc.line, tc.char); got != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, got, tc.want)
		}
	}
	// The verb of a cited step lists its sources after its usual hover.
	if got := hoverText(c, 20, 26); !strings.HasSuffix(got, "sources:\n- re-x src/offers.go:5-9 — offers come from the catalogue") {
		t.Errorf("hover on a cited verb = %q", got)
	}
	// A step with no cite keeps its usual hover.
	if got := hoverText(c, 12, 26); strings.Contains(got, "sources:") {
		t.Errorf("hover on an uncited verb = %q", got)
	}
}

func TestLinks_TitleCompletion(t *testing.T) {
	src := strings.Replace(linksLSPSrc, `        includes "Site session preamble"`, `        includes "Si`, 1)
	c := newWorkspaceClient(t, linksRoot, "", wsFile{linksURI, src})
	var list struct {
		Items []struct {
			Label    string `json:"label"`
			Kind     int    `json:"kind"`
			TextEdit *struct {
				NewText string `json:"newText"`
			} `json:"textEdit"`
		} `json:"items"`
	}
	if err := json.Unmarshal(c.at("textDocument/completion", 11, 20), &list); err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, it := range list.Items {
		labels = append(labels, it.Label)
		if it.Kind != 18 {
			t.Errorf("%q kind = %d, want Reference (18)", it.Label, it.Kind)
		}
	}
	want := []string{"Seller asks the assistant a question", "Seller pays at the checkout", "Seller promotes an advert", "Site session preamble"}
	if !slices.Equal(labels, want) {
		t.Errorf("titles = %v, want %v", labels, want)
	}
}

const formerSrc = `use_case "Pay at the till" {
    tags {
        formerly_titled: Pay
    }
    when S pays
        p does it
}
use_case "Start" {
    when S starts
        q does it
        continues "Pay"
}
`

func TestLinks_FormerTitleQuickFix(t *testing.T) {
	uri := "file:///ws/former.craft"
	c := newWorkspaceClient(t, linksRoot, "", wsFile{uri, formerSrc})
	diag := map[string]any{
		"range":   map[string]any{"start": map[string]any{"line": 10, "character": 18}, "end": map[string]any{"line": 10, "character": 23}},
		"code":    "craft/lint/link-to-former-title",
		"message": "former title",
	}
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"range":        diag["range"],
		"context":      map[string]any{"diagnostics": []any{diag}},
	})
	var actions []struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		Edit  struct {
			Changes map[string][]struct {
				Range struct {
					Start struct{ Line, Character int } `json:"start"`
					End   struct{ Line, Character int } `json:"end"`
				} `json:"range"`
				NewText string `json:"newText"`
			} `json:"changes"`
		} `json:"edit"`
	}
	if err := json.Unmarshal(c.call("textDocument/codeAction", p), &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Kind != "quickfix" {
		t.Fatalf("actions = %+v, want one quickfix", actions)
	}
	edits := actions[0].Edit.Changes[uri]
	if len(edits) != 1 || edits[0].NewText != `"Pay at the till"` || edits[0].Range.Start.Line != 10 ||
		edits[0].Range.Start.Character != 18 || edits[0].Range.End.Character != 23 {
		t.Errorf("edit = %+v", edits)
	}
	// Another code is not fixed.
	diag["code"] = "craft/lint/self-link"
	p, _ = json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}, "range": diag["range"], "context": map[string]any{"diagnostics": []any{diag}}})
	actions = nil
	json.Unmarshal(c.call("textDocument/codeAction", p), &actions) //nolint:errcheck
	if len(actions) != 0 {
		t.Errorf("unexpected actions %+v", actions)
	}
}

type docLink struct {
	Target string `json:"target"`
	Range  struct {
		Start struct{ Line, Character int } `json:"start"`
		End   struct{ Line, Character int } `json:"end"`
	} `json:"range"`
}

func documentLinks(t *testing.T, c *branchClient) []docLink {
	t.Helper()
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var links []docLink
	if err := json.Unmarshal(c.call("textDocument/documentLink", p), &links); err != nil {
		t.Fatal(err)
	}
	return links
}

func TestSources_DocumentLinks(t *testing.T) {
	c := linksClient(t, `{"sourceRoots": {"*": "/src", "re-frontend": "/work/fe"}}`)
	links := documentLinks(t, c)
	if len(links) != 2 {
		t.Fatalf("links = %+v, want 2", links)
	}
	// The written repo resolves through "*".
	if l := links[0]; l.Target != "file:///src/re-x/src/offers.go#L5" || l.Range.Start.Line != 21 || l.Range.Start.Character != 12 || l.Range.End.Character != 34 {
		t.Errorf("link 0 = %+v", l)
	}
	// No repo: the subject's service declares repo: re-frontend, which has
	// its own root.
	if l := links[1]; l.Target != "file:///work/fe/src/session.go#L3" || l.Range.Start.Line != 28 {
		t.Errorf("link 1 = %+v", l)
	}
	// Without sourceRoots there are no links.
	if got := documentLinks(t, linksClient(t, "")); len(got) != 0 {
		t.Errorf("links without sourceRoots = %+v", got)
	}
}

func TestLinks_SemanticTokens(t *testing.T) {
	const flowKeyword, usecaseString, comment = 8, 30, 34
	toks := semanticTokensAt(t, linksLSPSrc)
	for _, tc := range []struct {
		line, char, want uint32
		what             string
	}{
		{11, 8, flowKeyword, "includes"},
		{11, 17, usecaseString, "includes title"},
		{22, 8, flowKeyword, "continues"},
		{22, 18, usecaseString, "continues title"},
		{21, 8, comment, "//@ cite"},
	} {
		if got, ok := toks[[2]uint32{tc.line, tc.char}]; !ok || got != tc.want {
			t.Errorf("%s at %d:%d = %d (present %v), want %d", tc.what, tc.line, tc.char, got, ok, tc.want)
		}
	}
}

// TestSources_HoverOnEachStepKind uses the sources corpus file: cites on the
// use case, the trigger, an action across a plain comment, and a note.
func TestSources_HoverOnEachStepKind(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "corpus", "04_use_cases", "sources.craft"))
	if err != nil {
		t.Fatal(err)
	}
	c := newWorkspaceClient(t, linksRoot, "", wsFile{"file:///ws/sources.craft", string(raw)})
	for _, tc := range []struct {
		line, char int
		suffix     string
	}{
		{8, 2, "sources:\n- uc/top.go:1"},
		{11, 5, "sources:\n- re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 — one rule, eight detail-types"},
		{13, 26, "sources:\n- eb_email_sender.go:5-40"},
		{19, 14, "sources:\n- notes.md:3"},
		{9, 6, "attached to: use_case \"nMF mails the seller about an add-on event\""},
		{15, 10, "attached to: nmf_monetization picks the mail resolver from the event type alone"},
		{20, 14, "attached to: note \"no retry\""},
	} {
		if got := hoverText(c, tc.line, tc.char); !strings.HasSuffix(got, tc.suffix) {
			t.Errorf("hover %d:%d = %q, want suffix %q", tc.line, tc.char, got, tc.suffix)
		}
	}
}

const (
	spineBooksURI   = "file:///ws/vas/nmf-books-the-draws.craft"
	spinePromoteURI = "file:///ws/vas/seller-promotes-an-advert.craft"
)

// spineClient opens the two journey-spine fixtures; its uri is the promote
// file.
func spineClient(t *testing.T) *branchClient {
	t.Helper()
	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join("..", "visualizer", "testdata", "journey-spine", "vas", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	return newWorkspaceClient(t, linksRoot, "",
		wsFile{spinePromoteURI, read("seller-promotes-an-advert.craft")},
		wsFile{spineBooksURI, read("nmf-books-the-draws.craft")})
}

func TestSpine_Hover(t *testing.T) {
	c := spineClient(t)
	want := "journey seller-promotes-an-advert: 3 use cases\n" +
		"1. Seller promotes an advert (vas/seller-promotes-an-advert.craft)\n" +
		"2. nMF books the draws of a scheduled VAS (vas/nmf-books-the-draws.craft)\n" +
		"3. Seller pays at the checkout (vas/seller-promotes-an-advert.craft)"
	for _, pos := range [][2]int{{13, 17}, {25, 30}} {
		if got := hoverText(c, pos[0], pos[1]); got != want {
			t.Errorf("hover %v = %q, want %q", pos, got, want)
		}
	}
	if got := hoverText(c, 13, 10); strings.HasPrefix(got, "journey ") {
		t.Errorf("hover on the tag key = %q, want no spine", got)
	}
}

func TestSpine_HoverCycle(t *testing.T) {
	src := "use_case \"A\" {\n    tags {\n        journey: j\n    }\n    when a listens e.B\n        a notifies e.A\n}\n" +
		"use_case \"B\" {\n    tags {\n        journey: j\n    }\n    when b listens e.A\n        b notifies e.B\n}\n"
	c := newWorkspaceClient(t, linksRoot, "", wsFile{"file:///ws/c.craft", src})
	if got := hoverText(c, 2, 18); !strings.HasSuffix(got, "\ncycle") {
		t.Errorf("hover = %q, want a cycle line", got)
	}
}

func TestSpine_Definition(t *testing.T) {
	c := spineClient(t)
	var locs []lspLocation
	json.Unmarshal(c.at("textDocument/definition", 13, 20), &locs) //nolint:errcheck
	if len(locs) != 2 || locs[0].URI != spineBooksURI || locs[0].Range.Start.Line != 0 ||
		locs[1].URI != spinePromoteURI || locs[1].Range.Start.Line != 23 {
		t.Errorf("definition = %+v, want books (0) then checkout (23)", locs)
	}
}

func TestSpine_CodeLens(t *testing.T) {
	c := spineClient(t)
	lenses := func(uri string) []struct {
		Range struct {
			Start struct{ Line int } `json:"start"`
		} `json:"range"`
		Command struct {
			Title string `json:"title"`
		} `json:"command"`
	} {
		p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": uri}})
		var out []struct {
			Range struct {
				Start struct{ Line int } `json:"start"`
			} `json:"range"`
			Command struct {
				Title string `json:"title"`
			} `json:"command"`
		}
		if err := json.Unmarshal(c.call("textDocument/codeLens", p), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	got := lenses(spinePromoteURI)
	if len(got) != 2 || got[0].Range.Start.Line != 13 || got[1].Range.Start.Line != 25 {
		t.Fatalf("lenses = %+v, want lines 13 and 25", got)
	}
	for _, l := range got {
		if l.Command.Title != "3 use cases in this journey" {
			t.Errorf("lens title = %q", l.Command.Title)
		}
	}
	if got := lenses(spineBooksURI); len(got) != 1 || got[0].Range.Start.Line != 2 {
		t.Errorf("books lenses = %+v", got)
	}
}
