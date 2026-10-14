package lang_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
)

func TestParseStoreRef(t *testing.T) {
	cases := []struct {
		ref, name string
		change    lang.StoreChange
		ok        bool
	}{
		{"store.sms_queue.inserted", "sms_queue", lang.StoreInserted, true},
		{"store.orders.updated", "orders", lang.StoreUpdated, true},
		{"store.redis.leads.pending", "redis.leads.pending", "", true},
		{"store.redis.leads.deleted", "redis.leads", lang.StoreDeleted, true},
		{"store.inserted", "", "", false},
		{"store.", "", "", false},
		{"sqs.LeadRequested", "", "", false},
		{"storefront.OrderPlaced", "", "", false},
	}
	for _, c := range cases {
		name, change, ok := lang.ParseStoreRef(c.ref)
		if name != c.name || change != c.change || ok != c.ok {
			t.Errorf("%q: got (%q, %q, %v)", c.ref, name, change, ok)
		}
	}
}
