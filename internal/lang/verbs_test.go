package lang

import (
	"slices"
	"sort"
	"testing"
)

func TestProtocolVerbsAreSorted(t *testing.T) {
	got := ProtocolVerbs()
	want := []string{"ADDR", "DELETE", "GET", "GRPC", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "QUERY", "SQL", "TOPIC"}
	if !slices.Equal(got, want) || !sort.StringsAreSorted(got) {
		t.Errorf("ProtocolVerbs() = %v; want %v", got, want)
	}
}

func TestEdgeVerbs(t *testing.T) {
	cs, ok := LookupEdgeVerb("customer_supplier")
	if !ok || cs.Symmetric() || cs.UpstreamRole != "supplier" || cs.DownstreamRole != "customer" {
		t.Errorf("customer_supplier = %+v", cs)
	}
	for _, v := range EdgeVerbs() {
		if v.Symmetric() != (v.UpstreamRole == "" && v.DownstreamRole == "") {
			t.Errorf("%s: a symmetric verb has no roles and a directional one has both: %+v", v.Verb, v)
		}
	}
	if IsEdgeVerb("same_as") || !IsGlossaryVerb("same_as") {
		t.Error("glossary and context_map verbs must not mix")
	}
}
