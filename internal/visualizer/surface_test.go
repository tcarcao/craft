package visualizer

import (
	"strings"
	"testing"
)

const surfaceModel = `actors {
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

const surfaceMarkdown = "# Surfaces\n\n## concierge\n\nCallers: Advisor\nSpec: `obidozoe/docs/openapi.yaml`\n\n" +
	"> every route takes the shared HTTP Basic pair plus a bidder identity header\n\n" +
	"> **Finding:** a read-only credential is refused only at persistence, after the write ran\n\n" +
	"| Operation | Kind | What it offers | Used by |\n|---|---|---|---|\n" +
	"| `getUserList` | query | the seeker list, unpaged, newest first, filtered by a typed status | Advisor works a task |\n" +
	"| `getUserLogs` | query | the seeker audit trail | — |\n" +
	"| `deleteAccount` | command | delete a seeker's account | Seeker removes their concierge account (unobserved caller) |\n"

func TestSurface_Markdown(t *testing.T) {
	doc := resolvedDoc(t, surfaceModel)
	out, err := New(nil).Render(doc, Request{View: ViewSurface, Backend: PlantUML, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out.Data); got != surfaceMarkdown {
		t.Errorf("got:\n%s\nwant:\n%s", got, surfaceMarkdown)
	}
	if out.Ext != "md" {
		t.Errorf("ext = %q", out.Ext)
	}
}

func TestSurface_RejectsOtherFormatsAndFilters(t *testing.T) {
	doc := resolvedDoc(t, surfaceModel)
	for _, req := range []Request{
		{View: ViewSurface, Backend: PlantUML, Format: FormatSource},
		{View: ViewSurface, Backend: PlantUML, Format: FormatPNG},
		{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown, UseCases: []string{"Advisor works a task"}},
		{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown, C4: &C4Options{}},
	} {
		if _, err := New(nil).Render(doc, req); err == nil {
			t.Errorf("expected ErrUnsupported for %+v", req)
		}
	}
}

func TestSurface_MarkdownEscapesPipesAndOrdersBySubject(t *testing.T) {
	src := `domain d {
    zeta
    alpha
    app
}
surface zeta {
    query for a | b  [GET /z|y]
}
surface alpha {
    command reset  [reset]
}
use_case "Two | words" {
    when O does
        app asks zeta for it  [GET /z|y]
        app asks zeta for it again  [GET  /z|y]
}
use_case "A first" {
    when O does
        app asks d/zeta for it  [GET /z|y]
}
`
	out, err := New(nil).Render(resolvedDoc(t, src), Request{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out.Data)
	if !strings.Contains(got, "| `GET /z\\|y` | query | for a \\| b | A first, Two \\| words |\n") {
		t.Errorf("row not escaped or not deduplicated:\n%s", got)
	}
	if strings.Index(got, "## alpha") > strings.Index(got, "## zeta") {
		t.Errorf("tables not ordered by subject:\n%s", got)
	}
	if strings.Contains(got, "Callers:") || strings.Contains(got, "Spec:") {
		t.Errorf("empty header facts written:\n%s", got)
	}
}

// A surface on a service covers requests to the service's contexts that have
// no surface of their own (spec §8.4): the corpus writes the surface on the
// deployable and the asks on its contexts.
func TestSurface_ServiceSurfaceCoversItsContexts(t *testing.T) {
	src := `domain obido {
    advisorapp
    concierge
    ledger
}
services {
    obidozoe {
        contexts: concierge, ledger
    }
}
surface obidozoe {
    query the seeker list  [getUserList]
    query the balances  [getBalances]
}
surface ledger {
    query the balances  [getBalances]
}
use_case "Advisor works a task" {
    when Advisor opens the task list
        advisorapp asks concierge for the seekers  [getUserList]
        advisorapp asks ledger for the balances  [getBalances]
}
`
	out, err := New(nil).Render(resolvedDoc(t, src), Request{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out.Data)
	for _, want := range []string{
		"| `getUserList` | query | the seeker list | Advisor works a task |\n",
		"## obidozoe\n\n| Operation | Kind | What it offers | Used by |\n|---|---|---|---|\n| `getUserList` | query | the seeker list | Advisor works a task |\n| `getBalances` | query | the balances | — |\n",
		"## ledger\n\n| Operation | Kind | What it offers | Used by |\n|---|---|---|---|\n| `getBalances` | query | the balances | Advisor works a task |\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestSurface_OrdersSubjectsCaseInsensitively(t *testing.T) {
	src := "domain d {\n    Zeta\n    alpha\n}\nsurface Zeta {\n    query z  [z]\n}\nsurface alpha {\n    query a  [a]\n}\n"
	out, err := New(nil).Render(resolvedDoc(t, src), Request{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out.Data); strings.Index(got, "## alpha") > strings.Index(got, "## Zeta") {
		t.Errorf("tables not ordered case-insensitively:\n%s", got)
	}
}
