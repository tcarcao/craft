package interaction_test

import (
	"fmt"
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

// The trigger reaches the first action in source order, even inside a block.
func TestDerive_TriggerTargetsFirstActionInsideABlock(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "with a voucher"
            Billing applies the voucher
        else "without"
            Ordering charges the card
}
`})
	if its[0].Kind != interaction.Trigger || its[0].To.Name != "Billing" {
		t.Errorf("trigger = %+v", its[0])
	}
	// The trigger comes from outside the scenario's steps, so it sits in no
	// branch even though the action it reaches does.
	if its[0].Branch != nil {
		t.Errorf("trigger branch = %+v, want nil", its[0].Branch)
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
