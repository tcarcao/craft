package visualizer

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
)

// sequenceSource draws src as a sequence diagram in both backends.
func sequenceSource(t *testing.T, src string) (puml, mmd string) {
	t.Helper()
	doc := resolvedDoc(t, src)
	p, _, err := renderSource(doc, ViewSequence, PlantUML)
	if err != nil {
		t.Fatalf("plantuml: %v", err)
	}
	m, _, err := renderSource(doc, ViewSequence, Mermaid)
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	return string(p), string(m)
}

func requireLine(t *testing.T, backend, out, want string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == want {
			return
		}
	}
	t.Errorf("%s output has no line %q:\n%s", backend, want, out)
}

func forbidLine(t *testing.T, backend, out, unwanted string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == unwanted {
			t.Errorf("%s output has line %q:\n%s", backend, unwanted, out)
		}
	}
}

// When the actor that triggers a scenario also takes its first step, the
// trigger is what the actor does, not a call the actor makes to itself: it is
// drawn as a note over the actor.
func TestSequence_TriggerOnTheActorItselfIsANote(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Advisor
domain obido {
    concierge
}
use_case "Advisor reads a seeker file" {
    when Advisor opens a seeker file
        Advisor asks concierge for one seeker file
}
`)
	forbidLine(t, "plantuml", puml, "Advisor -> Advisor : opens a seeker file")
	requireLine(t, "plantuml", puml, "note over Advisor : opens a seeker file")
	requireLine(t, "mermaid", mmd, "Note over Advisor: opens a seeker file")
}

// An action's operation annotation is the wire contract of that step, so the
// sequence diagram shows it after the label.
func TestSequence_OperationAnnotationIsDrawn(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Seeker
domain web {
    frontendplatform
    graphql
}
use_case "Seeker opens an advert" {
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        graphql notifies "AdvertViewed"                [TOPIC web.advert-viewed]
}
`)
	requireLine(t, "plantuml", puml, "frontendplatform -> graphql : for the advert [ad]")
	requireLine(t, "mermaid", mmd, "frontendplatform->>graphql: for the advert [ad]")
	requireLine(t, "plantuml", puml, `graphql ->> graphql : notifies "AdvertViewed" [TOPIC web.advert-viewed]`)
}

// A use case with several scenarios frames each one, numbered and titled by
// its trigger, so alternatives do not read as one long chain. A use case with
// a single scenario is drawn without a frame.
func TestSequence_ScenariosAreFramed(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
    when Customer places an order with an empty basket
        Ordering rejects the order
}
use_case "Single" {
    when Customer cancels an order
        Ordering asks Billing to refund the card
}
`)
	requireLine(t, "plantuml", puml, "group 1: when Customer places an order")
	requireLine(t, "plantuml", puml, "group 2: when Customer places an order with an empty basket")
	if n := strings.Count(puml, "\nend\n"); n != 2 {
		t.Errorf("plantuml: %d group ends, want 2:\n%s", n, puml)
	}
	forbidLine(t, "plantuml", puml, "group 1: when Customer cancels an order")
	if !strings.Contains(mmd, ": 2: when Customer places an order with an empty basket") || strings.Count(mmd, "    rect ") != 2 {
		t.Errorf("mermaid scenarios not framed:\n%s", mmd)
	}
}

// The flow view has no note form: a trigger on the actor's own first step adds
// no edge, since the actor is already connected through the step itself.
func TestFlow_TriggerOnTheActorItselfAddsNoSelfEdge(t *testing.T) {
	doc := resolvedDoc(t, `actor user Advisor
domain obido {
    concierge
}
use_case "Advisor reads a seeker file" {
    when Advisor opens a seeker file
        Advisor asks concierge for one seeker file
}
`)
	out, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "Advisor --> Advisor") {
		t.Errorf("flow has an actor self-edge:\n%s", out)
	}
	requireLine(t, "plantuml", string(out), "Advisor ->> concierge : 1. for one seeker file")
}

const branchesSrc = `actor user Customer
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

// alt and opt blocks are drawn as the backend's native blocks, nested inside
// the scenario frame, and frames are titled by the scenario name.
func TestSequence_BranchesAreNativeBlocks(t *testing.T) {
	puml, mmd := sequenceSource(t, branchesSrc)
	for _, want := range []string{
		"group 1: online order",
		"alt the card is accepted",
		"else the card is declined",
		"opt the bank also declines",
		"group 2: phone order",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	i := strings.Index(puml, "opt the bank also declines")
	if i < 0 {
		t.Fatalf("plantuml: no opt line:\n%s", puml)
	}
	if !strings.Contains(puml[i:], "end\nend\n") {
		t.Errorf("plantuml: opt and alt not both closed after the opt:\n%s", puml)
	}
	for _, want := range []string{"alt the card is accepted", "else the card is declined", "opt the bank also declines"} {
		requireLine(t, "mermaid", mmd, want)
	}
}

// between returns the text of out from start through the first end after it
// (both included), or fails.
func between(t *testing.T, backend, out, start, end string) string {
	t.Helper()
	i := strings.Index(out, start)
	if i < 0 {
		t.Fatalf("%s output has no %q:\n%s", backend, start, out)
	}
	from := i + len(start)
	j := strings.Index(out[from:], end)
	if j < 0 {
		t.Fatalf("%s output has no %q after %q:\n%s", backend, end, start, out)
	}
	return out[i : from+j+len(end)]
}

// The exact text of a scenario frame catches ordering bugs (an end in the
// wrong place, an else before its messages) that line checks cannot.
func TestSequence_BranchesExactPlantUML(t *testing.T) {
	puml, _ := sequenceSource(t, branchesSrc)
	got := between(t, "plantuml", puml, "group 1: online order\n", "group 2: phone order\n")
	want := `group 1: online order
Customer -> Ordering : places an order
Ordering -> Billing : to charge the card
alt the card is accepted
Billing --> Ordering : returns the receipt
else the card is declined
Billing -> Bank : for a second opinion
opt the bank also declines
Bank --> Billing : returns the refusal
end
end
Ordering --> Customer : returns the confirmation
end
group 2: phone order
`
	if got != want {
		t.Errorf("plantuml online order frame:\n--- got\n%s--- want\n%s", got, want)
	}
}

func TestSequence_BranchesExactMermaid(t *testing.T) {
	_, mmd := sequenceSource(t, branchesSrc)
	got := between(t, "mermaid", mmd, "    rect rgb(245, 245, 245)\n    Note over Customer,Bank: 1: online order\n", "    rect ")
	want := `    rect rgb(245, 245, 245)
    Note over Customer,Bank: 1: online order
    Customer->>Ordering: places an order
    Ordering->>Billing: to charge the card
    alt the card is accepted
    Billing-->>Ordering: returns the receipt
    else the card is declined
    Billing->>Bank: for a second opinion
    opt the bank also declines
    Bank-->>Billing: returns the refusal
    end
    end
    Ordering-->>Customer: returns the confirmation
    end
    rect `
	if got != want {
		t.Errorf("mermaid online order frame:\n--- got\n%s--- want\n%s", got, want)
	}
}

// A scenario that ends inside a block closes the block before its frame.
func TestSequence_BlocksCloseBeforeTheFrame(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    scenario "online order"
    when Customer places an order
        Ordering asks Billing to charge the card
        opt "the card is declined"
            Billing returns the refusal
    scenario "phone order"
    when Customer phones in an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
}
`)
	want := "Billing --> Ordering : returns the refusal\nend\nend\ngroup 2: phone order\n"
	if !strings.Contains(puml, want) {
		t.Errorf("plantuml: opt not closed before the frame:\n%s", puml)
	}
	if !strings.HasSuffix(puml, "Billing --> Ordering : returns the receipt\nend\nend\n\n@enduml") {
		t.Errorf("plantuml: alt not closed at the end of the section:\n%s", puml)
	}
	if !strings.HasSuffix(mmd, "    Billing-->>Ordering: returns the receipt\n    end\n    end\n") {
		t.Errorf("mermaid: alt not closed at the end of the section:\n%s", mmd)
	}
}

// A use case with a single scenario keeps no frame even when it is named; in
// a use case with several, an unnamed scenario is titled by its trigger.
func TestSequence_FrameTitles(t *testing.T) {
	puml, _ := sequenceSource(t, `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Single" {
    scenario "only one"
    when Customer cancels an order
        Ordering asks Billing to refund the card
}
use_case "Checkout" {
    scenario "online order"
    when Customer places an order
        Ordering asks Billing to charge the card
    when Customer phones in an order
        Ordering asks Billing to charge the card
}
`)
	forbidLine(t, "plantuml", puml, "group 1: only one")
	requireLine(t, "plantuml", puml, "group 1: online order")
	requireLine(t, "plantuml", puml, "group 2: when Customer phones in an order")
}

// blockLines keeps the block lines of out, trimmed, in order.
func blockLines(out string) []string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		for _, kw := range []string{"alt ", "else ", "opt ", "group ", "rect "} {
			if strings.HasPrefix(l, kw) {
				keep = append(keep, l)
			}
		}
		if l == "end" {
			keep = append(keep, l)
		}
	}
	return keep
}

const emptyBranchHead = `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
`

// A branch that draws nothing is still drawn: the reader sees the outcome
// exists, and an alt starts with its own first label.
func TestSequence_EmptyBranchesKeepTheirHeaders(t *testing.T) {
	cases := []struct {
		name, body string
		want       []string
		wantPUML   string // a fragment of the plantuml output, in order
	}{
		{
			name: "empty first branch",
			body: `        alt "the card is accepted"
        else "the card is declined"
            Billing returns the refusal
`,
			want:     []string{"alt the card is accepted", "else the card is declined", "end"},
			wantPUML: "Ordering -> Billing : to charge the card\nalt the card is accepted\nelse the card is declined\nBilling --> Ordering : returns the refusal\nend\n",
		},
		{
			name: "empty middle branch",
			body: `        alt "the card is accepted"
            Billing returns the receipt
        else "the card is pending"
        else "the card is declined"
            Billing returns the refusal
`,
			want:     []string{"alt the card is accepted", "else the card is pending", "else the card is declined", "end"},
			wantPUML: "alt the card is accepted\nBilling --> Ordering : returns the receipt\nelse the card is pending\nelse the card is declined\nBilling --> Ordering : returns the refusal\nend\n",
		},
		{
			name: "empty opt",
			body: `        opt "the card needs a check"
        Ordering returns the confirmation
`,
			want:     []string{"opt the card needs a check", "end"},
			wantPUML: "Ordering -> Billing : to charge the card\nopt the card needs a check\nend\nOrdering --> Customer : returns the confirmation\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			puml, mmd := sequenceSource(t, emptyBranchHead+c.body+"}\n")
			for backend, out := range map[string]string{"plantuml": puml, "mermaid": mmd} {
				if got := blockLines(out); strings.Join(got, "|") != strings.Join(c.want, "|") {
					t.Errorf("%s block lines = %q, want %q:\n%s", backend, got, c.want, out)
				}
			}
			if !strings.Contains(puml, c.wantPUML) {
				t.Errorf("plantuml has no\n%s\nin:\n%s", c.wantPUML, puml)
			}
		})
	}
}

// Mermaid block labels get the same ; sanitizing as message labels.
func TestSequence_MermaidBlockLabelsAreSanitized(t *testing.T) {
	_, mmd := sequenceSource(t, emptyBranchHead+`        opt "the card; is declined"
            Billing returns the refusal
}
`)
	requireLine(t, "mermaid", mmd, "opt the card, is declined")
}

// Two use cases may share a name, even across files whose scenario and step
// numbering collide. They are drawn in one section, each scenario with its own
// messages in source order, and no branch reads as empty because the other use
// case already drew its step.
func TestSequence_DuplicateUseCaseNames(t *testing.T) {
	const decl = `actor user Customer
domain shop {
    Ordering
    Billing
}
`
	first := decl + `use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        Ordering returns the confirmation
}
`
	second := `use_case "Checkout" {
    when Customer phones in an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
        else "the card is declined"
            Billing returns the refusal
}
`
	want := `== Checkout ==

Customer -> Ordering : places an order
Ordering -> Billing : to charge the card
Ordering --> Customer : returns the confirmation
Customer -> Ordering : phones in an order
Ordering -> Billing : to charge the card
alt the card is accepted
Billing --> Ordering : returns the receipt
else the card is declined
Billing --> Ordering : returns the refusal
end

@enduml`
	for name, files := range map[string][]analysis.FileResult{
		// Per-file counters: both scenarios are scenario_1.
		"two files": {analysis.File("a.craft", first), analysis.File("b.craft", second)},
		"one file":  {analysis.File("a.craft", first+second)},
	} {
		t.Run(name, func(t *testing.T) {
			doc := analysis.Workspace(files).Doc
			p, _, err := renderSource(doc, ViewSequence, PlantUML)
			if err != nil {
				t.Fatal(err)
			}
			// The last section of a name draws every use case of that name
			// (an earlier one stays empty, as before branches existed).
			puml := string(p)
			if !strings.HasSuffix(puml, want) {
				t.Errorf("plantuml:\n--- got\n%s\n--- want suffix\n%s", puml, want)
			}
		})
	}
}

// Adjacent same-named use cases from two files, both scenario_1, where the
// first scenario draws only step-0 interactions (its listens): each scenario
// keeps its own messages.
func TestSequence_DuplicateUseCaseNamesWithOnlyStepZero(t *testing.T) {
	first := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Billing listens "Card Charged"
}
`
	second := `use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        opt "the card is declined"
            Billing returns the refusal
}
`
	want := `== Checkout ==

[-> Billing : Card Charged
Customer -> Ordering : places an order
Ordering -> Billing : to charge the card
opt the card is declined
Billing --> Ordering : returns the refusal
end

@enduml`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("a.craft", first), analysis.File("b.craft", second)}).Doc
	p, _, err := renderSource(doc, ViewSequence, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(p), want) {
		t.Errorf("plantuml:\n--- got\n%s\n--- want suffix\n%s", p, want)
	}
}

// A use-case filter keeps the exact use case of each interaction: indices
// are those of the filtered document.
func TestSequence_UseCaseFilterKeepsBranches(t *testing.T) {
	doc := resolvedDoc(t, emptyBranchHead+`        Ordering returns the confirmation
}
use_case "Refund" {
    when Customer asks for a refund
        Ordering asks Billing to refund the card
        alt "the card is open"
            Billing returns the refund
        else "the card is closed"
            Billing returns the refusal
}
`)
	out, err := New(nil).Render(doc, Request{View: ViewSequence, Backend: PlantUML, Format: FormatSource, UseCases: []string{"refund"}})
	if err != nil {
		t.Fatal(err)
	}
	want := `== Refund ==

Customer -> Ordering : asks for a refund
Ordering -> Billing : to refund the card
alt the card is open
Billing --> Ordering : returns the refund
else the card is closed
Billing --> Ordering : returns the refusal
end

@enduml`
	if !strings.HasSuffix(string(out.Data), want) {
		t.Errorf("plantuml:\n--- got\n%s\n--- want suffix\n%s", out.Data, want)
	}
}

func TestFlow_EdgesInsideABranchCarryTheBranchLabel(t *testing.T) {
	doc := resolvedDoc(t, branchesSrc)
	for _, backend := range []Backend{PlantUML, Mermaid} {
		out, _, err := renderSource(doc, ViewFlow, backend)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		for _, want := range []string{
			"[the card is declined] for a second opinion",
			"[the bank also declines] returns the refusal",
			"[the card is accepted] returns the receipt",
		} {
			if !strings.Contains(s, want) {
				t.Errorf("%v flow lacks %q:\n%s", backend, want, s)
			}
		}
		if strings.Contains(s, "[the card is declined] returns the refusal") {
			t.Errorf("%v: innermost label must win:\n%s", backend, s)
		}
		for _, line := range strings.Split(s, "\n") {
			if strings.Contains(line, "places an order") && strings.Contains(line, "[") {
				t.Errorf("%v: trigger edge has a branch prefix: %q", backend, line)
			}
		}
		if strings.Contains(s, "] to charge the card") || strings.Contains(s, "] returns the confirmation") {
			t.Errorf("%v: edge outside a block has a prefix:\n%s", backend, s)
		}
	}
}

// A scenario that opens with a block is triggered into every branch: each
// branch's trigger is drawn inside that branch, before its messages. A
// trigger whose actor takes the branch's first step itself is still a note.
const openingBlockSrc = `actor user Seeker
domain web {
    frontendplatform
    mobile
    search
}
use_case "Seeker searches" {
    when Seeker opens the search page
        alt "on the web"
            frontendplatform asks search for listings
        else "in the app"
            mobile asks search for listings
        else "offline"
            Seeker reads the cached results
        search records the query
}
`

func TestSequence_TriggerIsDrawnInsideEachOpeningBranchPlantUML(t *testing.T) {
	puml, _ := sequenceSource(t, openingBlockSrc)
	want := `== Seeker searches ==

alt on the web
Seeker -> frontendplatform : opens the search page
frontendplatform -> search : for listings
else in the app
Seeker -> mobile : opens the search page
mobile -> search : for listings
else offline
note over Seeker : opens the search page
Seeker -> Seeker : reads the cached results
end
search -> search : records the query

@enduml`
	if !strings.HasSuffix(puml, want) {
		t.Errorf("plantuml:\n--- got\n%s\n--- want suffix\n%s", puml, want)
	}
}

func TestSequence_TriggerIsDrawnInsideEachOpeningBranchMermaid(t *testing.T) {
	_, mmd := sequenceSource(t, openingBlockSrc)
	want := `    alt on the web
    Seeker->>frontendplatform: opens the search page
    frontendplatform->>search: for listings
    else in the app
    Seeker->>mobile: opens the search page
    mobile->>search: for listings
    else offline
    Note over Seeker: opens the search page
    Seeker->>Seeker: reads the cached results
    end
    search->>search: records the query
`
	if !strings.Contains(mmd, want) {
		t.Errorf("mermaid:\n--- got\n%s\n--- want\n%s", mmd, want)
	}
}

// In the flow view, which has no boxes, a trigger into a branch names its
// branch like any other edge inside a block.
func TestFlow_TriggerIntoABranchCarriesTheBranchLabel(t *testing.T) {
	doc := resolvedDoc(t, openingBlockSrc)
	for _, backend := range []Backend{PlantUML, Mermaid} {
		out, _, err := renderSource(doc, ViewFlow, backend)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		for _, want := range []string{"[on the web] opens the search page", "[in the app] opens the search page"} {
			if !strings.Contains(s, want) {
				t.Errorf("%v: no %q:\n%s", backend, want, s)
			}
		}
	}
}

// An event published in another use case arrives from outside the listener's
// section, labelled with its publishers: sorted by display name, at most 3,
// then "+N more". An event nobody in the model publishes keeps its bare label.
const crossUseCaseEventsSrc = `actor user Customer
domain shop {
    Accounts
    Billing
    Ordering
    Payments
    Shipping
}
use_case "Publish" {
    when Customer pays
        Payments notifies "One"
        Ordering notifies "Two"
        Billing notifies "Two"
        Payments notifies "Four"
        Ordering notifies "Four"
        Accounts notifies "Four"
        Billing notifies "Four"
}
use_case "Listen one" {
    when Shipping listens "One"
        Shipping books a courier
}
use_case "Listen two" {
    when Shipping listens "Two"
        Shipping books a courier
}
use_case "Listen four" {
    when Shipping listens "Four"
        Shipping books a courier
}
use_case "Listen none" {
    when Shipping listens "Nobody"
        Shipping books a courier
}
`

func TestSequence_InboundEventNamesItsPublishers(t *testing.T) {
	puml, mmd := sequenceSource(t, crossUseCaseEventsSrc)
	for _, want := range []string{
		"[-> Shipping : One (from Payments)",
		"[-> Shipping : Two (from Billing, Ordering)",
		"[-> Shipping : Four (from Accounts, Billing, Ordering, +1 more)",
		"[-> Shipping : Nobody",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{
		"External->>Shipping: One (from Payments)",
		"External->>Shipping: Two (from Billing, Ordering)",
		"External->>Shipping: Four (from Accounts, Billing, Ordering, +1 more)",
		"External->>Shipping: Nobody",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
	if n := strings.Count(puml, "[-> "); n != 4 {
		t.Errorf("plantuml: %d inbound arrows, want one per listening scenario (4):\n%s", n, puml)
	}
}

const notesSrc = `actor user Seller
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

func TestSequence_NotesAndFindings(t *testing.T) {
	puml, mmd := sequenceSource(t, notesSrc)
	for _, want := range []string{
		"note over Seller, paymentgateway #FFE4E1 : Finding: nothing retries a failed callback; a lost callback leaves the invoice open",
		"note over frontendplatform, graphql : the gateway forwards the seller's session, not a service token",
		"note over graphql, paymentgateway #FFE4E1 : Finding: no idempotency key: a client retry starts a second payment",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{
		"Note over Seller,paymentgateway: Finding: nothing retries a failed callback, a lost callback leaves the invoice open",
		"Note over frontendplatform,graphql: the gateway forwards the seller's session, not a service token",
		"Note over graphql,paymentgateway: Finding: no idempotency key: a client retry starts a second payment",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
	// The use-case finding comes before the trigger.
	if strings.Index(puml, "Finding: nothing retries") > strings.Index(puml, "Seller -> frontendplatform") {
		t.Errorf("use-case-level finding must open the section:\n%s", puml)
	}
}

const parRenderSrc = `actor user Seeker
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

func TestSequence_ParAndLoop(t *testing.T) {
	puml, mmd := sequenceSource(t, parRenderSrc)
	for _, want := range []string{"par field resolvers", "else", "loop for each image, until all are signed", "par the advert", "else the statistics"} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{"par field resolvers", "and", "loop for each image, until all are signed", "and the statistics"} {
		requireLine(t, "mermaid", mmd, want)
	}
	forbidLine(t, "mermaid", mmd, "else")
	for name, out := range map[string]string{"plantuml": puml, "mermaid": mmd} {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasSuffix(l, " ") {
				t.Errorf("%s: trailing space in %q", name, l)
			}
		}
	}
}

func TestSequence_EmptyAltLabelHasNoTrailingSpace(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Customer
domain shop {
    Ordering
}
use_case "U" {
    when Customer places an order
        alt ""
            Ordering accepts it
        else ""
            Ordering refuses it
}
`)
	requireLine(t, "plantuml", puml, "alt")
	requireLine(t, "plantuml", puml, "else")
	requireLine(t, "mermaid", mmd, "alt")
	requireLine(t, "mermaid", mmd, "else")
	for name, out := range map[string]string{"plantuml": puml, "mermaid": mmd} {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasSuffix(l, " ") {
				t.Errorf("%s: trailing space in %q", name, l)
			}
		}
	}
}

func TestFlow_ParAndLoopEdgesNameTheirKind(t *testing.T) {
	doc := resolvedDoc(t, parRenderSrc)
	out, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[par: field resolvers] for the SEO title", "[loop: for each image, until all are signed] to sign the image URL", "[par: the statistics] for the page statistics"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("flow lacks %q:\n%s", want, out)
		}
	}
}

const receivesSrc = `domain ad {
    graphql
    atlas
}
use_case "Adverts are swept for reindexing" {
    when graphql receives a reindex sweep  [reindexAdverts]
        graphql asks atlas for every advert that needs reindexing  [getAlladvertstoreindex]
        graphql returns the advert ids
}
`

func TestSequence_ReceivesDrawsFoundAndLostMessages(t *testing.T) {
	puml, mmd := sequenceSource(t, receivesSrc)
	for _, want := range []string{
		"[-> graphql : a reindex sweep [reindexAdverts]",
		"graphql -> atlas : for every advert that needs reindexing [getAlladvertstoreindex]",
		"graphql -->] : returns the advert ids",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	if strings.Contains(puml, "Unobserved") || strings.Contains(puml, "unobserved") {
		t.Errorf("plantuml must not declare a participant for the unobserved caller:\n%s", puml)
	}
	for _, want := range []string{
		"participant Unobserved as unobserved caller",
		"Unobserved->>graphql: a reindex sweep [reindexAdverts]",
		"graphql-->>Unobserved: returns the advert ids",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
	if strings.Count(mmd, "participant Unobserved") != 1 {
		t.Errorf("mermaid declares the unobserved caller more than once:\n%s", mmd)
	}
}

// A receives scenario that opens with a block draws its found message once,
// before the block.
func TestSequence_ReceivesOpeningWithABlockDrawsTheFoundMessageFirst(t *testing.T) {
	puml, _ := sequenceSource(t, `domain ad {
    graphql
    atlas
}
use_case "Sweep" {
    when graphql receives a sweep  [reindex]
        alt "full"
            graphql asks atlas for all
        else "partial"
            graphql asks atlas for some
}
`)
	found := strings.Index(puml, "[-> graphql : a sweep [reindex]")
	alt := strings.Index(puml, "alt full")
	if found < 0 || alt < 0 || found > alt || strings.Count(puml, "[-> graphql") != 1 {
		t.Errorf("found message not drawn once before the block:\n%s", puml)
	}
}

func TestFlow_ReceivesHasAnUnobservedCallerNode(t *testing.T) {
	doc := resolvedDoc(t, receivesSrc)
	out, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `cloud "unobserved caller"`) {
		t.Errorf("flow lacks the unobserved caller node:\n%s", out)
	}
}

func TestArchitectureAndC4_ReceivesDrawNoUnobservedCaller(t *testing.T) {
	doc := resolvedDoc(t, receivesSrc)
	for _, v := range []View{ViewArchitecture, ViewC4} {
		for _, backend := range []Backend{PlantUML, Mermaid} {
			out, _, err := renderSource(doc, v, backend)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.ToLower(string(out)), "unobserved") {
				t.Errorf("%s/%s draws the unobserved caller:\n%s", v, backend, out)
			}
		}
	}
}
