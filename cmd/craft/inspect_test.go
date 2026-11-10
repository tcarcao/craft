package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func runInspect(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"inspect"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("inspect %v: %v", args, err)
	}
	return buf.String()
}

func TestInspectCmd_JSONMerge(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.craft")
	b := filepath.Join(tmp, "b.craft")
	os.WriteFile(a, []byte("domain Billing {\n  Invoicing\n}\n"), 0644)
	os.WriteFile(b, []byte("domain Catalog {\n  Products\n}\n"), 0644)

	// pass in sorted order so output is order-stable (see plan D5)
	out := runInspect(t, "--format", "json", a, b)
	var got struct {
		Domains []struct{ Name string } `json:"domains"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d: %s", len(got.Domains), out)
	}
}

// A `listens` list and a `notifies one of` report every item, as
// Trigger.EventList and Action.EventList expose them, not the first only.
func TestInspectCmd_EventListsReportEveryItem(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.craft")
	src := `use_case "Settle" {
  when billing listens orders.OrderPlaced, orders.OrderCancelled, orders.OrderPlaced
    billing notifies one of billing.Settled, billing.Rejected
}
`
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := runInspect(t, "--format", "json", f)
	var got struct {
		UseCases []struct {
			Name            string   `json:"name"`
			EventsPublished []string `json:"events_published"`
			EventsConsumed  []string `json:"events_consumed"`
		} `json:"use_cases"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.UseCases) != 1 {
		t.Fatalf("expected 1 use case, got %d: %s", len(got.UseCases), out)
	}
	uc := got.UseCases[0]
	if want := []string{"orders.OrderPlaced", "orders.OrderCancelled"}; !slices.Equal(uc.EventsConsumed, want) {
		t.Errorf("events_consumed = %v, want %v", uc.EventsConsumed, want)
	}
	if want := []string{"billing.Settled", "billing.Rejected"}; !slices.Equal(uc.EventsPublished, want) {
		t.Errorf("events_published = %v, want %v", uc.EventsPublished, want)
	}
}

const flowsCraft = `flow lead-message "Lead generation (phone and message)" {
    priority: P1
    owner: seeker-engagement
    metric: "Leads"
}

use_case "Seeker sends a message" {
    tags {
        realises: lead-message
    }
    when Seeker sends a message
        chat asks messaging to deliver the message
}
`

// Flows and the use cases that realise them are part of the inspect model
// (v2.35).
func TestInspectCmd_Flows(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.craft")
	if err := os.WriteFile(f, []byte(flowsCraft), 0644); err != nil {
		t.Fatal(err)
	}
	out := runInspect(t, "--format", "json", f)
	var got struct {
		UseCases []struct {
			Name     string   `json:"name"`
			Realises []string `json:"realises"`
		} `json:"use_cases"`
		Flows []struct {
			Ref        string   `json:"ref"`
			Name       string   `json:"name"`
			Priority   string   `json:"priority"`
			Owner      string   `json:"owner"`
			Metric     string   `json:"metric"`
			RealisedBy []string `json:"realised_by"`
		} `json:"flows"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.UseCases) != 1 || !slices.Equal(got.UseCases[0].Realises, []string{"lead-message"}) {
		t.Errorf("use_cases = %+v", got.UseCases)
	}
	if len(got.Flows) != 1 {
		t.Fatalf("flows = %+v\n%s", got.Flows, out)
	}
	fl := got.Flows[0]
	if fl.Ref != "lead-message" || fl.Name != "Lead generation (phone and message)" || fl.Priority != "P1" ||
		fl.Owner != "seeker-engagement" || fl.Metric != "Leads" || !slices.Equal(fl.RealisedBy, []string{"Seeker sends a message"}) {
		t.Errorf("flow = %+v", fl)
	}
}

const pagesCraft = `page account-adverts "My Account: My Ads" {
    url: "https://www.otodom.pl/wiadomosci/moje-ogloszenia"
    platform: frontoffice
    product_domain: ad-inventory
}

use_case "Seller promotes an ad" {
    tags {
        entry: account-adverts
    }
    when Seller promotes an ad
        promotion asks billing to charge the seller
}
`

// Pages and the use cases that start on them are part of the inspect model
// (v2.36).
func TestInspectCmd_Pages(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.craft")
	if err := os.WriteFile(f, []byte(pagesCraft), 0644); err != nil {
		t.Fatal(err)
	}
	out := runInspect(t, "--format", "json", f)
	var got struct {
		Pages []struct {
			Ref           string   `json:"ref"`
			Name          string   `json:"name"`
			URLs          []string `json:"urls"`
			Platform      string   `json:"platform"`
			ProductDomain string   `json:"product_domain"`
			UsedBy        []string `json:"used_by"`
		} `json:"pages"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Pages) != 1 {
		t.Fatalf("pages = %+v\n%s", got.Pages, out)
	}
	pg := got.Pages[0]
	if pg.Ref != "account-adverts" || pg.Name != "My Account: My Ads" || pg.Platform != "frontoffice" || pg.ProductDomain != "ad-inventory" ||
		!slices.Equal(pg.URLs, []string{"https://www.otodom.pl/wiadomosci/moje-ogloszenia"}) || !slices.Equal(pg.UsedBy, []string{"Seller promotes an ad"}) {
		t.Errorf("page = %+v", pg)
	}
}

// Page and flow aliases are part of the inspect model (v2.38), in JSON and
// in text.
func TestInspectCmd_Aliases(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.craft")
	src := "page p \"P\" {\n    product_domain: x\n    alias: performance\n    alias: \"ad performance\"\n}\n" +
		"flow fl \"F\" {\n    priority: P1\n    owner: o\n    alias: promotion\n}\n"
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Pages []struct {
			Aliases []string `json:"aliases"`
		} `json:"pages"`
		Flows []struct {
			Aliases []string `json:"aliases"`
		} `json:"flows"`
	}
	out := runInspect(t, "--format", "json", f)
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Pages) != 1 || !slices.Equal(got.Pages[0].Aliases, []string{"performance", "ad performance"}) ||
		len(got.Flows) != 1 || !slices.Equal(got.Flows[0].Aliases, []string{"promotion"}) {
		t.Errorf("inspect = %s", out)
	}
}

// An actor's AI kind and owner are part of the inspect model (v2.37); an
// actor without a body prints as before.
func TestInspectCmd_ActorAI(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.craft")
	src := "actors {\n    system Gen \"generator\" {\n        ai: genai\n        owner: \"OLX Data\"\n    }\n    system Siren\n}\n"
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := runInspect(t, "--format", "json", f)
	var got struct {
		Actors []struct {
			Name  string `json:"name"`
			AI    string `json:"ai"`
			Owner string `json:"owner"`
		} `json:"actors"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Actors) != 2 || got.Actors[0].AI != "genai" || got.Actors[0].Owner != "OLX Data" || got.Actors[1].AI != "" {
		t.Errorf("actors = %+v", got.Actors)
	}
}
