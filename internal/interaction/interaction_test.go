package interaction_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

func derive(t *testing.T, files map[string]string) []interaction.Interaction {
	t.Helper()
	var results []analysis.FileResult
	for name, src := range files {
		results = append(results, analysis.File(name, src))
	}
	return interaction.Derive(analysis.Workspace(results).Doc)
}

// summary renders interactions compactly: "kind from->to [label]".
func summary(its []interaction.Interaction) string {
	var lines []string
	for _, it := range its {
		to := it.To.Name
		if it.To.IsZero() {
			to = "·"
		}
		lines = append(lines, fmt.Sprintf("%d %s %s->%s [%s]", it.Step, it.Kind, it.From.Name, to, it.Label))
	}
	return strings.Join(lines, "\n")
}

const structure = `actor user Customer
actor system Bank

domain shop {
    Ordering
    Billing
    Shipping
}

services {
    ShopService {
        contexts: Ordering, Billing
        data-stores: orders_db
    }
}
`

func TestDerive_ScenarioInSourceOrder(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places order
        Ordering validates the basket
        Ordering asks Billing to charge the card
        Billing asks Bank for authorization
        Bank returns the decision
        Billing returns to Ordering the receipt
        Ordering asks orders_db to store the order
        Ordering notifies "Order Placed"
}
`})
	want := `0 trigger Customer->Ordering [places order]
1 internal Ordering->Ordering [validates the basket]
2 request Ordering->Billing [to charge the card]
3 request Billing->Bank [for authorization]
4 response Bank->Billing [returns the decision]
5 response Billing->Ordering [returns the receipt]
6 request Ordering->orders_db [to store the order]
7 publish Ordering->· [Order Placed]`
	if got := summary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if its[6].To.Kind != model.ParticipantDataStore || its[6].To.Service != "ShopService" {
		t.Errorf("data store endpoint = %+v", its[6].To)
	}
}

// A return with no target and no caller left goes to External; the
// triggering actor is the bottom of the call stack.
func TestDerive_UntargetedReturnsUnwindTheCallStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Quote" {
    when Customer asks for a quote
        Ordering asks Billing for a price
        Billing returns the price
        Ordering returns the quote
        Ordering returns nothing more
}
`})
	var got []string
	for _, it := range its {
		if it.Kind == interaction.Response {
			got = append(got, it.To.Name)
		}
	}
	if strings.Join(got, ",") != "Ordering,Customer,External" {
		t.Errorf("return targets = %v, want Ordering,Customer,External", got)
	}
}

// Every publisher pairs with every listener of the event, and an event nobody
// in the model publishes is delivered from External.
func TestDerive_ListenersPairWithEveryPublisher(t *testing.T) {
	its := derive(t, map[string]string{
		"a.craft": structure + `
use_case "Pay" {
    when Customer pays
        Billing notifies "Payment Settled"
}
use_case "Refund" {
    when Customer asks for a refund
        Ordering notifies "Payment Settled"
}
`,
		"b.craft": `
use_case "Ship" {
    when Shipping listens "Payment Settled"
        Shipping books a courier
    when Shipping listens "Address Changed"
        Shipping updates the label
}
`,
	})
	var got []string
	for _, it := range its {
		if it.Kind == interaction.Listen {
			got = append(got, fmt.Sprintf("%s->%s %s", it.From.Name, it.To.Name, it.Event))
		}
	}
	want := "Billing->Shipping Payment Settled|Ordering->Shipping Payment Settled|External->Shipping Address Changed"
	if strings.Join(got, "|") != want {
		t.Errorf("listens = %q, want %q", strings.Join(got, "|"), want)
	}
}

// A qualified participant resolves to its bounded context, and display names
// stay bare unless two drawn bounded contexts share one.
func TestDerive_QualifiedNamesAndDisplayNames(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": `
domain re {
    billing
    ledger
}
domain ops {
    ledger
}
use_case "U" {
    when re/billing starts
        re/billing asks re/ledger to record it
        re/billing asks ops/ledger to mirror it
}
`})
	if len(its) != 2 {
		t.Fatalf("got %d interactions:\n%s", len(its), summary(its))
	}
	if its[0].From.Key != "re/billing" || its[0].From.Name != "billing" {
		t.Errorf("unique name should display bare: %+v", its[0].From)
	}
	if its[0].To.Name != "re/ledger" || its[1].To.Name != "ops/ledger" {
		t.Errorf("a name in two domains should display qualified: %q, %q", its[0].To.Name, its[1].To.Name)
	}
}

// A bounded context in the actor slot is the system starting its own work:
// no trigger edge from it.
func TestDerive_BoundedContextAsTriggerSubjectIsNotAnActor(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Nightly" {
    when Billing starts the nightly run
        Billing closes the day
}
`})
	if its[0].Kind == interaction.Trigger {
		t.Errorf("unexpected trigger from a bounded context: %s", summary(its))
	}
}

// Unresolved names are still drawn, keyed by what was written.
func TestDerive_UnresolvedNamesAreKept(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "U" {
    when Customer does it
        Ordering asks Gateway to forward it
}
`})
	to := its[1].To
	if to.Key != "Gateway" || to.Kind != "" {
		t.Errorf("unresolved endpoint = %+v", to)
	}
}

func TestDerive_Sites(t *testing.T) {
	its := derive(t, map[string]string{"dir/a.craft": structure + `
use_case "U" {
    when Customer does it
        Ordering asks Billing to charge
}
`})
	trig, req := its[0].Site, its[1].Site
	if trig.URI != "dir/a.craft" || trig.Column != 10 || trig.Length != 8 {
		t.Errorf("trigger site = %+v", trig)
	}
	if req.Line != trig.Line+1 || req.Column != 9 || req.Length != 8 {
		t.Errorf("request site = %+v", req)
	}
}

func TestDerive_IsDeterministic(t *testing.T) {
	files := map[string]string{"a.craft": structure + `
use_case "U" {
    when Customer does it
        Ordering notifies "A Happened"
        Billing notifies "A Happened"
    when Shipping listens "A Happened"
        Shipping reacts
}
`}
	first := summary(derive(t, files))
	for i := 0; i < 20; i++ {
		if got := summary(derive(t, files)); got != first {
			t.Fatalf("run %d differs:\n%s\nvs\n%s", i, got, first)
		}
	}
}

// A trigger's connector word is part of what the actor does: "places an
// order" must not be drawn as "places order".
func TestDerive_TriggerLabelKeepsConnector(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering validates the basket
}
`})
	if got, want := its[0].Label, "places an order"; got != want {
		t.Errorf("trigger label = %q, want %q", got, want)
	}
}

// A return answers the most recent call made TO the returner. When the
// returner's own call was never answered explicitly, the return still goes to
// whoever called the returner — never back to the returner itself.
func TestDerive_ReturnGoesToWhoeverCalledTheReturner(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places order
        Ordering asks Billing to charge the card
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering [places order]
1 request Ordering->Billing [to charge the card]
2 response Ordering->Customer [returns the confirmation]`
	if got := summary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func branchSummary(its []interaction.Interaction) string {
	var lines []string
	for _, it := range its {
		var path []string
		for _, b := range it.Branch {
			path = append(path, fmt.Sprintf("%s:%s#%d", b.Kind, b.Label, b.Index))
		}
		to := it.To.Name
		if it.To.IsZero() {
			to = "·"
		}
		lines = append(lines, fmt.Sprintf("%d %s %s->%s {%s}", it.Step, it.Kind, it.From.Name, to, strings.Join(path, " > ")))
	}
	return strings.Join(lines, "\n")
}

func TestDerive_BranchPathsAndPerBranchCallStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
        else "the card is declined"
            Billing asks Bank for a second opinion
            opt "the bank also declines"
                Bank returns the refusal
            Billing returns the refusal
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {}
2 response Billing->Ordering {alt:the card is accepted#0}
3 request Billing->Bank {alt:the card is declined#1}
4 response Bank->Billing {alt:the card is declined#1 > opt:the bank also declines#0}
5 response Billing->Ordering {alt:the card is declined#1}
6 response Ordering->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A scenario that opens with a block is triggered into every branch: one
// trigger per branch, to that branch's first action, carrying its path.
func TestDerive_TriggerReachesEveryBranchOfAnOpeningBlock(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "with a voucher"
            Billing applies the voucher
        else "without"
            Ordering charges the card
        Ordering confirms the order
}
`})
	want := `0 trigger Customer->Billing {alt:with a voucher#0}
1 internal Billing->Billing {alt:with a voucher#0}
0 trigger Customer->Ordering {alt:without#1}
2 internal Ordering->Ordering {alt:without#1}
3 internal Ordering->Ordering {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if len(its) == 5 && its[0].Branch[0].Block != its[2].Branch[0].Block {
		t.Errorf("triggers sit in blocks %q and %q, want one block", its[0].Branch[0].Block, its[2].Branch[0].Block)
	}
	for _, i := range []int{0, 2} {
		if i < len(its) && (its[i].Label != "places an order" || its[i].Site.Line == 0) {
			t.Errorf("trigger %d = %+v, want the scenario's label and site", i, its[i])
		}
	}
}

// A branch that opens with a block recurses: its trigger goes to each inner
// branch. A branch with no action gets no trigger.
func TestDerive_TriggerRecursesIntoNestedOpeningBlocksAndSkipsEmptyBranches(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "nothing to do"
        else "online"
            opt "with a voucher"
                Billing applies the voucher
            Ordering charges the card
        else "by phone"
            Shipping books a courier
}
`})
	want := `0 trigger Customer->Billing {alt:online#1 > opt:with a voucher#0}
1 internal Billing->Billing {alt:online#1 > opt:with a voucher#0}
2 internal Ordering->Ordering {alt:online#1}
0 trigger Customer->Shipping {alt:by phone#2}
3 internal Shipping->Shipping {alt:by phone#2}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Each branch's call stack starts with that branch's own trigger frame. In the
// second branch Ordering's return answers the actor through that frame, past
// the call Shipping left open; with the first branch's frame instead,
// Ordering would be nobody's callee and its return would go to Shipping.
// After the block the stack is restored as before it, so a later return by
// the first branch's subject still answers the actor.
func TestDerive_EachBranchStartsWithItsTriggerFrame(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "online"
            Billing asks Bank for authorization
        else "by phone"
            Ordering validates the order
            Shipping asks Billing for a fee
            Ordering returns the confirmation
        Billing returns the receipt
}
`})
	want := `0 trigger Customer->Billing {alt:online#0}
1 request Billing->Bank {alt:online#0}
0 trigger Customer->Ordering {alt:by phone#1}
2 internal Ordering->Ordering {alt:by phone#1}
3 request Shipping->Billing {alt:by phone#1}
4 response Ordering->Customer {alt:by phone#1}
5 response Billing->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A scenario that opens with an action keeps its single trigger, outside any
// branch, even when a block follows.
func TestDerive_TriggerBeforeABlockHasNoBranch(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering validates the basket
        alt "with a voucher"
            Billing applies the voucher
}
`})
	want := `0 trigger Customer->Ordering {}
1 internal Ordering->Ordering {}
2 internal Billing->Billing {alt:with a voucher#0}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Each alt branch starts from the call stack as it was at the alt, and the
// stack after the block is the one from before it. A call left unanswered in
// the first branch must not capture a return in the second branch (it would
// go to Ordering), and the second branch's return must not leak past the
// block (the last return would go to Customer).
func TestDerive_AltBranchesDoNotShareTheCallStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Shipping asks Billing to reserve the shipping fee
        alt "the card is charged now"
            Ordering asks Billing to charge the card
        else "the card is charged later"
            Billing returns the reservation
        Billing returns the fee
}
`})
	want := `0 trigger Customer->Shipping {}
1 request Shipping->Billing {}
2 request Ordering->Billing {alt:the card is charged now#0}
3 response Billing->Shipping {alt:the card is charged later#1}
4 response Billing->Shipping {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A notifies inside a branch pairs with listeners like any other, and the
// publish carries its branch path.
func TestDerive_PublishInsideABranchPairsWithListeners(t *testing.T) {
	its := derive(t, map[string]string{
		"a.craft": structure + `
use_case "Pay" {
    when Customer pays
        Billing charges the card
        opt "the payment settles"
            Billing notifies "Payment Settled"
}
`,
		"b.craft": `
use_case "Ship" {
    when Shipping listens "Payment Settled"
        Shipping books a courier
}
`,
	})
	var publish, listen *interaction.Interaction
	for i := range its {
		switch its[i].Kind {
		case interaction.Publish:
			publish = &its[i]
		case interaction.Listen:
			listen = &its[i]
		}
	}
	if publish == nil || len(publish.Branch) != 1 || publish.Branch[0].Label != "the payment settles" || publish.Branch[0].Index != 0 || publish.Branch[0].Block == "" {
		t.Errorf("publish = %+v", publish)
	}
	if listen == nil || listen.From.Name != "Billing" || listen.To.Name != "Shipping" || listen.Event != "Payment Settled" || listen.Branch != nil {
		t.Errorf("listen = %+v", listen)
	}
}

// Each interaction owns its Branch slice: changing one never shows in another,
// not even in a sibling from the same branch.
func TestDerive_BranchSlicesAreNotShared(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "the card is accepted"
            Ordering validates the basket
            Ordering asks Billing to charge the card
}
`})
	first, second := &its[1], &its[2]
	if len(first.Branch) != 1 || len(second.Branch) != 1 {
		t.Fatalf("branches = %+v / %+v", first.Branch, second.Branch)
	}
	first.Branch[0].Label = "changed"
	first.Branch = append(first.Branch, interaction.BranchRef{Label: "extra"})
	if got := second.Branch; len(got) != 1 || got[0].Label != "the card is accepted" {
		t.Errorf("second branch = %+v, want it untouched", got)
	}
}

// Two alts in sequence, one with an empty branch: each interaction carries the
// path of its own block, and the stack after the first alt is the one from
// before it. The call to Bank sits in the last branch, so nothing resets the
// stack after it: leaking it would send Bank's return to Billing; restored,
// Bank answers the latest other caller, Ordering.
func TestDerive_SequentialAltsAndAnEmptyBranch(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        alt "the bank is skipped"
        else "the bank is asked"
            Billing asks Bank for authorization
        alt "the bank answers"
            Bank returns the decision
        else "billing answers"
            Billing returns the receipt
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {}
2 request Billing->Bank {alt:the bank is asked#1}
3 response Bank->Ordering {alt:the bank answers#0}
4 response Billing->Ordering {alt:billing answers#1}
5 response Ordering->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if len(its) == 6 && its[2].Branch[0].Block == its[3].Branch[0].Block {
		t.Errorf("both alts have block %q, want distinct IDs", its[2].Branch[0].Block)
	}
	if len(its) == 6 && its[3].Branch[0].Block != its[4].Branch[0].Block {
		t.Errorf("branches of one alt have blocks %q and %q", its[3].Branch[0].Block, its[4].Branch[0].Block)
	}
}

// UseCaseIndex identifies the use case exactly, even when two files hold a use
// case with the same name and their scenario IDs collide.
func TestDerive_UseCaseIndexAcrossFiles(t *testing.T) {
	doc := analysis.Workspace([]analysis.FileResult{
		analysis.File("a.craft", structure+`use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
}
use_case "Refund" {
    when Customer asks for a refund
        Ordering asks Billing to refund the card
}
`),
		analysis.File("b.craft", `use_case "Checkout" {
    when Customer phones in an order
        Ordering asks Billing to charge the card
}
`),
	}).Doc
	if len(doc.UseCases) != 3 {
		t.Fatalf("got %d use cases, want 3", len(doc.UseCases))
	}
	its := interaction.Derive(doc)
	seen := map[int]int{}
	for _, it := range its {
		if it.UseCaseIndex < 0 || it.UseCaseIndex >= len(doc.UseCases) {
			t.Fatalf("UseCaseIndex %d out of range", it.UseCaseIndex)
		}
		uc := doc.UseCases[it.UseCaseIndex]
		if uc.Name != it.UseCase {
			t.Errorf("UseCaseIndex %d is %q, interaction says %q", it.UseCaseIndex, uc.Name, it.UseCase)
		}
		var scenarioHere bool
		for _, sc := range uc.Scenarios {
			scenarioHere = scenarioHere || sc.ID == it.Scenario
		}
		if !scenarioHere {
			t.Errorf("use case %d has no scenario %q", it.UseCaseIndex, it.Scenario)
		}
		seen[it.UseCaseIndex]++
	}
	// Each use case has a trigger and an ask.
	for i := range doc.UseCases {
		if seen[i] != 2 {
			t.Errorf("use case %d (%s, %s): %d interactions, want 2", i, doc.UseCases[i].Name, doc.UseCases[i].SourceURI, seen[i])
		}
	}
	// The trigger of b.craft's Checkout belongs to b.craft's use case.
	for _, it := range its {
		if it.Label == "phones in an order" && doc.UseCases[it.UseCaseIndex].SourceURI != "b.craft" {
			t.Errorf("b.craft trigger has UseCaseIndex %d (%s)", it.UseCaseIndex, doc.UseCases[it.UseCaseIndex].SourceURI)
		}
	}
}

// A scheduled trigger is drawn from the CRON actor, labelled with its schedule.
func TestDerive_ScheduledTriggerIsDrawnFromCRON(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `actor system CRON
use_case "Digest" {
    when cron "0 7 * * 1-5"
        Ordering builds the digest
    scenario "poll"
    when every "5m"
        Billing polls the feed
}
`})
	var got []string
	for _, it := range its {
		if it.Kind == interaction.Trigger {
			got = append(got, it.From.Name+"->"+it.To.Name+" ["+it.Label+"]")
		}
	}
	want := "CRON->Ordering [cron 0 7 * * 1-5]|CRON->Billing [every 5m]"
	if strings.Join(got, "|") != want {
		t.Errorf("triggers = %q, want %q", strings.Join(got, "|"), want)
	}
}

// A branch that opens with a block whose own branches are all empty still gets
// a trigger: it goes to the branch's first action in source order.
func TestDerive_TriggerFallsBackToTheBranchsFirstAction(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "online"
            opt "nothing"
            Ordering charges the card
        else "by phone"
            Shipping books a courier
}
`})
	want := `0 trigger Customer->Ordering {alt:online#0}
1 internal Ordering->Ordering {alt:online#0}
0 trigger Customer->Shipping {alt:by phone#1}
2 internal Shipping->Shipping {alt:by phone#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// The trigger is drawn into any branch that has a target, even when the
// scenario's first action in source order has no context to call.
func TestDerive_TriggerIgnoresABranchWithoutAContext(t *testing.T) {
	act := func(ctx string) model.Step {
		return model.Step{Action: &model.Action{Type: model.ActionTypeInternal, Context: ctx, Verb: "does"}}
	}
	doc := &model.CraftDoc{UseCases: []model.UseCase{{Name: "U", Scenarios: []model.Scenario{{
		ID:      "s1",
		Trigger: model.Trigger{Type: model.TriggerTypeExternal, Actor: "Customer"},
		Steps: []model.Step{{Block: &model.Block{ID: "b1", Kind: model.BlockAlt, Branches: []model.Branch{
			{Label: "first", Steps: []model.Step{act("")}},
			{Label: "second", Steps: []model.Step{act("Ordering")}},
		}}}},
	}}}}}
	want := `0 trigger Customer->Ordering {alt:second#1}
2 internal Ordering->Ordering {alt:second#1}`
	if got := branchSummary(interaction.Derive(doc)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// An opening opt may be skipped: besides the trigger inside its body, the
// scenario is triggered, outside any branch, into the first action after it.
func TestDerive_OpeningOptAlsoTriggersTheStepAfterIt(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        opt "with a voucher"
            Billing applies the voucher
        Ordering charges the card
}
`})
	want := `0 trigger Customer->Billing {opt:with a voucher#0}
1 internal Billing->Billing {opt:with a voucher#0}
0 trigger Customer->Ordering {}
2 internal Ordering->Ordering {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// With nothing after the opt there is no skipped path to trigger; an alt never
// gets one, since one of its branches always runs.
func TestDerive_OpeningOptWithoutAStepAfterItAndAltGetNoSkipTrigger(t *testing.T) {
	for _, src := range []string{`
        opt "with a voucher"
            Billing applies the voucher`, `
        alt "a"
            Billing applies the voucher
        else "b"
            Shipping books a courier
        Ordering charges the card`} {
		its := derive(t, map[string]string{"a.craft": structure + "\nuse_case \"Checkout\" {\n    when Customer places an order" + src + "\n}\n"})
		for _, it := range its {
			if it.Kind == interaction.Trigger && it.Branch == nil {
				t.Errorf("unexpected unbranched trigger in:%s\n%s", src, branchSummary(its))
			}
		}
	}
}

func TestDerive_ParBranchesStartFromTheParStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        par "side effects"
            Billing returns the receipt
            Billing asks Bank to log it
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {}
2 response Billing->Ordering {par:side effects#0}
3 request Billing->Bank {par:side effects#1}
4 response Ordering->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, it := range its {
		if len(it.Branch) > 0 && !it.Branch[0].Split {
			t.Errorf("fan-out branch ref must carry Split: %+v", it.Branch[0])
		}
	}
}

func TestDerive_OpeningParTriggersOnceWhenEveryBranchStartsAtTheSameClient(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens the page
        par "fan-out"
            Ordering asks Billing for a
            Ordering asks Shipping for b
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {par:fan-out#0}
2 request Ordering->Shipping {par:fan-out#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_OpeningParTriggersABranchThatStartsElsewhere(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens the page
        par "fan-out"
            Ordering asks Billing for a
            Shipping asks Billing for b
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {par:fan-out#0}
0 trigger Customer->Shipping {par:fan-out#1}
2 request Shipping->Billing {par:fan-out#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_OpeningLoopIsTriggeredOnce(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Batch" {
    when Customer uploads a batch
        loop "for each order"
            Ordering asks Billing to charge
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {loop:for each order#0}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_LeadingNoteDoesNotHideTheOpeningBlock(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        note "the client retries on timeout"
        alt "with a voucher"
            Billing applies the voucher
        else "without"
            Ordering charges the card
}
`})
	want := `0 trigger Customer->Billing {alt:with a voucher#0}
1 internal Billing->Billing {alt:with a voucher#0}
0 trigger Customer->Ordering {alt:without#1}
2 internal Ordering->Ordering {alt:without#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_LeadingNoteDoesNotHideTheOpeningOpt(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        note "vouchers are optional"
        opt "with a voucher"
            Billing applies the voucher
        Ordering charges the card
}
`})
	want := `0 trigger Customer->Billing {opt:with a voucher#0}
1 internal Billing->Billing {opt:with a voucher#0}
0 trigger Customer->Ordering {}
2 internal Ordering->Ordering {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_ReceivesIsAFoundMessageAndItsReplyIsLost(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Sweep" {
    when Ordering receives a reindex sweep  [reindex]
        Ordering asks Billing for the totals  [getTotals]
        Billing returns the totals
        Ordering returns the ids
}
`})
	want := `0 trigger unobserved caller->Ordering [a reindex sweep]
1 request Ordering->Billing [for the totals]
2 response Billing->Ordering [returns the totals]
3 response Ordering->unobserved caller [returns the ids]`
	if got := summary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if its[0].From != interaction.Unobserved || its[0].Operation == nil || its[0].Operation.Text != "reindex" {
		t.Errorf("trigger = %+v", its[0])
	}
	if its[0].Site.Line == 0 || its[0].Site.Length != len("Ordering") {
		t.Errorf("trigger site = %+v", its[0].Site)
	}
}

func TestDerive_ReceivesOpeningWithABlockStillTriggersOnceUpFront(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Sweep" {
    when Ordering receives a sweep  [reindex]
        alt "full"
            Ordering asks Billing for all
        else "partial"
            Ordering asks Billing for some
}
`})
	if its[0].Kind != interaction.Trigger || its[0].Branch != nil || its[0].To.Name != "Ordering" {
		t.Errorf("trigger = %+v", its[0])
	}
	n := 0
	for _, it := range its {
		if it.Kind == interaction.Trigger {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d triggers, want 1", n)
	}
}

// A branch of an opening alt that itself opens with a par or a loop is
// triggered once, at the branch, before the box, as a scenario that opens with
// a par or loop is. Only a par branch that starts at another participant gets
// a trigger of its own.
func TestDerive_ParOrLoopOpeningAnAltBranchIsTriggeredOnceBeforeTheBox(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens an advert
        alt "fresh"
            par "fan"
                Ordering asks Billing for a
                Ordering asks Shipping for b
        else "stale"
            loop "each"
                Ordering asks Billing to refresh
        else "mixed"
            par "split"
                Ordering asks Billing for c
                Shipping asks Billing for d
}
`})
	want := `0 trigger Customer->Ordering {alt:fresh#0}
1 request Ordering->Billing {alt:fresh#0 > par:fan#0}
2 request Ordering->Shipping {alt:fresh#0 > par:fan#1}
0 trigger Customer->Ordering {alt:stale#1}
3 request Ordering->Billing {alt:stale#1 > loop:each#0}
0 trigger Customer->Ordering {alt:mixed#2}
4 request Ordering->Billing {alt:mixed#2 > par:split#0}
0 trigger Customer->Shipping {alt:mixed#2 > par:split#1}
5 request Shipping->Billing {alt:mixed#2 > par:split#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// In a branch of an opening par that starts elsewhere and itself opens with a
// par, the inner par's branches compare against that branch's trigger.
func TestDerive_NestedOpeningParsTriggerEachNewSubjectOnce(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens the page
        par "outer"
            Ordering asks Billing for a
        and "other"
            par "inner"
                Shipping asks Billing for b
                Shipping asks Ordering for c
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {par:outer#0}
0 trigger Customer->Shipping {par:other#1}
2 request Shipping->Billing {par:other#1 > par:inner#0}
3 request Shipping->Ordering {par:other#1 > par:inner#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A listens trigger with a trailing schedule carries its cadence on each Listen.
func TestDerive_ListenCarriesItsSchedule(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Ship" {
    when Shipping listens "Payment Settled" every "5m"
        Shipping books a courier
}
`})
	var listens []interaction.Interaction
	for _, it := range its {
		if it.Kind == interaction.Listen {
			listens = append(listens, it)
		}
	}
	if len(listens) != 1 || listens[0].Schedule != "every 5m" {
		t.Errorf("listens = %+v, want one with Schedule \"every 5m\"", listens)
	}
}

func TestDerive_StoreHandOffsCarryTheStoreEndpoint(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Write" {
    when Customer places an order
        Ordering notifies store.orders_db.inserted
        Ordering notifies store.carts.updated
}
use_case "Read" {
    when Billing listens store.orders_db.inserted cron "*/5 * * * *"
        Billing charges the order
}
`})
	var pubs, listens []interaction.Interaction
	for _, it := range its {
		switch it.Kind {
		case interaction.Publish:
			pubs = append(pubs, it)
		case interaction.Listen:
			listens = append(listens, it)
		}
	}
	if v := pubs[0].Via; v.Key != "orders_db" || v.Kind != model.ParticipantDataStore || v.Service != "ShopService" {
		t.Errorf("resolved store Via = %+v", v)
	}
	if v := pubs[1].Via; v.Key != "store.carts" || v.Name != "carts" || v.Kind != model.ParticipantDataStore {
		t.Errorf("unresolved store Via = %+v", v)
	}
	if len(listens) != 1 || listens[0].From.Name != "Ordering" || listens[0].Via.Key != "orders_db" || listens[0].Schedule != "cron */5 * * * *" {
		t.Errorf("listen = %+v", listens)
	}
}

func TestDerive_ListensListGivesOneListenPerItemPerPublisher(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Pay" {
    when Customer pays
        Billing notifies sns.paid
        Shipping notifies sns.paid
}
use_case "Ledger" {
    when Ordering listens sns.paid, sns.refunded every "5m"
        Ordering writes a row
}
`})
	var got []string
	for _, it := range its {
		if it.Kind == interaction.Listen {
			got = append(got, it.From.Name+"->"+it.To.Name+" "+it.Event+" "+it.Schedule)
		}
	}
	want := []string{"Billing->Ordering sns.paid every 5m", "Shipping->Ordering sns.paid every 5m", "External->Ordering sns.refunded every 5m"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestDerive_OperationListIsCopied(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Keys" {
    when Customer asks for a key
        Ordering asks Billing to create the key and bind it  [CreateApiKey, CreateUsagePlanKey]
}
`})
	req := its[1]
	if req.Operation == nil || req.Operation.Text != "CreateApiKey, CreateUsagePlanKey" || len(req.Operations) != 2 || req.Operations[1].Text != "CreateUsagePlanKey" {
		t.Errorf("request = %+v", req)
	}
}
