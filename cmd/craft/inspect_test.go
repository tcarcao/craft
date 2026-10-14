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
