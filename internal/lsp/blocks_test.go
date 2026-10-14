package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tcarcao/craft/v2/internal/lsp"
)

const branchSrc = `actor user Customer
actor system Bank
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    scenario "online order"
    when Customer places an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
        else "the card is declined"
            Billing asks Bank for a second opinion
            opt "the bank also declines"
                Bank returns the refusal
        Ordering returns the confirmation
    scenario "phone order"
    when Customer phones in an order
        Ordering asks Billing to charge the card
}
`

type branchClient struct {
	t   *testing.T
	out io.Writer
	br  *bufio.Reader
	id  int
	uri string
}

func newBranchClient(t *testing.T, src string) (*branchClient, json.RawMessage) {
	t.Helper()
	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()
	ctx, cancel := context.WithTimeout(context.Background(), serverLifetime)
	t.Cleanup(func() { cancel(); testOut.Close() })
	go func() { defer serverOut.Close(); lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck

	c := &branchClient{t: t, out: testOut, br: bufio.NewReader(testIn), uri: "file:///branches.craft"}
	initResp := c.call("initialize", json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`))
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": c.uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)
	return c, initResp
}

func (c *branchClient) call(method string, params json.RawMessage) json.RawMessage {
	c.t.Helper()
	c.id++
	id := c.id
	if err := writeMsg(c.out, lspMsg{JSONRPC: "2.0", ID: &id, Method: method, Params: params}); err != nil {
		c.t.Fatalf("write %s: %v", method, err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(c.br)
		if err != nil {
			c.t.Fatalf("read %s: %v", method, err)
		}
		if msg.ID != nil && *msg.ID == id {
			if msg.Error != nil {
				c.t.Fatalf("%s error: %s", method, msg.Error)
			}
			return msg.Result
		}
	}
	c.t.Fatalf("%s: timed out", method)
	return nil
}

func (c *branchClient) at(method string, line, char int) json.RawMessage {
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": c.uri},
		"position":     map[string]any{"line": line, "character": char},
	})
	return c.call(method, p)
}

type outlineSym struct {
	Name     string       `json:"name"`
	Detail   string       `json:"detail"`
	Range    lineRange    `json:"range"`
	Children []outlineSym `json:"children"`
}

type lineRange struct {
	Start struct{ Line int } `json:"start"`
	End   struct{ Line int } `json:"end"`
}

func (s outlineSym) lines() [2]int { return [2]int{s.Range.Start.Line, s.Range.End.Line} }

func TestBranchOutline(t *testing.T) {
	c, _ := newBranchClient(t, branchSrc)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []outlineSym
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	var uc *outlineSym
	for i := range syms {
		if syms[i].Detail == "use_case" {
			uc = &syms[i]
		}
	}
	if uc == nil {
		t.Fatalf("no use case symbol in %+v", syms)
	}
	if len(uc.Children) != 2 {
		t.Fatalf("use case children = %+v, want 2 scenarios", uc.Children)
	}
	online, phone := uc.Children[0], uc.Children[1]
	if online.Name != "online order" || online.lines() != [2]int{7, 16} {
		t.Errorf("online scenario = %q %v, want \"online order\" [7 16]", online.Name, online.lines())
	}
	if phone.Name != "phone order" || phone.lines() != [2]int{17, 19} {
		t.Errorf("phone scenario = %q %v, want \"phone order\" [17 19]", phone.Name, phone.lines())
	}
	if len(online.Children) != 1 {
		t.Fatalf("online children = %+v, want one alt block", online.Children)
	}
	alt := online.Children[0]
	if alt.Name != "alt: the card is accepted | the card is declined" || alt.lines() != [2]int{10, 15} {
		t.Errorf("alt = %q %v", alt.Name, alt.lines())
	}
	if len(alt.Children) != 1 {
		t.Fatalf("alt children = %+v, want nested opt", alt.Children)
	}
	if opt := alt.Children[0]; opt.Name != "opt: the bank also declines" || opt.lines() != [2]int{14, 15} {
		t.Errorf("opt = %q %v", opt.Name, opt.lines())
	}
}

func TestBranchOutline_UnnamedScenarioUsesTrigger(t *testing.T) {
	src := "use_case \"X\" {\n    when Customer phones in\n        A asks B\n}\n"
	c, _ := newBranchClient(t, src)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []outlineSym
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || len(syms[0].Children) != 1 || syms[0].Children[0].Name != "when Customer phones in" {
		t.Fatalf("got %+v", syms)
	}
}

func outlineOf(t *testing.T, src string) []outlineSym {
	t.Helper()
	c, _ := newBranchClient(t, src)
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": c.uri}})
	var syms []outlineSym
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	return syms
}

func TestBranchOutline_TriggerNameIgnoresComment(t *testing.T) {
	src := "use_case \"X\" {\n    when Customer phones in an order // note\n        A asks B\n}\n"
	syms := outlineOf(t, src)
	if len(syms) != 1 || len(syms[0].Children) != 1 {
		t.Fatalf("got %+v", syms)
	}
	if got := syms[0].Children[0].Name; got != "when Customer phones in an order" {
		t.Errorf("name = %q", got)
	}
}

func TestBranchOutline_ScenarioStartingWithBlock(t *testing.T) {
	src := "use_case \"X\" {\n    when Customer pays\n        alt \"ok\"\n            A asks B\n        else \"no\"\n            A asks C\n}\n"
	syms := outlineOf(t, src)
	if len(syms) != 1 || len(syms[0].Children) != 1 {
		t.Fatalf("got %+v", syms)
	}
	sc := syms[0].Children[0]
	if len(sc.Children) != 1 || sc.Children[0].Name != "alt: ok | no" || sc.Children[0].lines() != [2]int{2, 5} {
		t.Errorf("scenario children = %+v", sc.Children)
	}
}

func TestBranchOutline_OrphanScenarioLineHasNoSymbol(t *testing.T) {
	src := "use_case \"X\" {\n    when Customer pays\n        A asks B\n    scenario \"x\"\n}\n"
	syms := outlineOf(t, src)
	if len(syms) != 1 {
		t.Fatalf("got %+v", syms)
	}
	for _, ch := range syms[0].Children {
		if ch.Name == "x" {
			t.Errorf("ghost scenario symbol present: %+v", syms[0].Children)
		}
	}
	if len(syms[0].Children) != 1 {
		t.Errorf("children = %+v, want only the real scenario", syms[0].Children)
	}
}

func TestBranchFolding(t *testing.T) {
	c, _ := newBranchClient(t, branchSrc)
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
	for _, want := range [][2]int{{6, 20}, {10, 15}, {14, 15}} {
		if !got[want] {
			t.Errorf("missing fold %v in %v", want, ranges)
		}
	}
	if !got[[2]int{2, 5}] {
		t.Errorf("domain fold regressed: %v", ranges)
	}
}

func TestBranchHover(t *testing.T) {
	c, _ := newBranchClient(t, branchSrc)
	hover := func(line, char int) string {
		var h struct {
			Contents struct{ Value string } `json:"contents"`
		}
		json.Unmarshal(c.at("textDocument/hover", line, char), &h) //nolint:errcheck
		return h.Contents.Value
	}
	if v := hover(10, 8); !strings.Contains(v, "alt") || !strings.Contains(v, "2 branches") {
		t.Errorf("alt hover = %q", v)
	}
	if v := hover(14, 12); !strings.Contains(v, "opt") || !strings.Contains(v, "1 branch") || strings.Contains(v, "1 branches") {
		t.Errorf("opt hover = %q", v)
	}
}

func TestBranchCompletion(t *testing.T) {
	src := strings.Replace(branchSrc, "    when Customer phones in an order", "        \n    when Customer phones in an order", 1)
	c, _ := newBranchClient(t, src)
	items := c.at("textDocument/completion", 18, 8)
	var list struct {
		Items []completionItem `json:"items"`
	}
	if err := json.Unmarshal(items, &list); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"alt":      `alt "${1:condition}"`,
		"opt":      `opt "${1:condition}"`,
		"else":     `else "${1:condition}"`,
		"scenario": `scenario "${1:name}"`,
	}
	for label, text := range want {
		found := false
		for _, it := range list.Items {
			if it.Label == label {
				found = true
				if it.InsertText != text {
					t.Errorf("%s insert = %q, want %q", label, it.InsertText, text)
				}
				if it.InsertTextFormat != 2 {
					t.Errorf("%s insertTextFormat = %v, want 2 (snippet)", label, it.InsertTextFormat)
				}
			}
		}
		if !found {
			t.Errorf("missing %q in %v", label, labelList(list.Items))
		}
	}
	if !hasLabel(list.Items, "when") {
		t.Errorf("when no longer offered: %v", labelList(list.Items))
	}
}
