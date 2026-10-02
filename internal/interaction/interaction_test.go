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
