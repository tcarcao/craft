package lang

import (
	"sort"
	"testing"
)

func TestProtocolVerbsAreSorted(t *testing.T) {
	got := ProtocolVerbs()
	if len(got) != 10 || !sort.StringsAreSorted(got) {
		t.Errorf("ProtocolVerbs() = %v; want the 10 verbs, sorted", got)
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
