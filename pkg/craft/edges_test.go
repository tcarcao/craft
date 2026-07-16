package craft

import "testing"

func TestEdgeVerbs(t *testing.T) {
	info, ok := LookupEdgeVerb("customer_supplier")
	if !ok {
		t.Fatalf("LookupEdgeVerb(customer_supplier) not found")
	}
	if info.Class != EdgeDirectional {
		t.Errorf("customer_supplier Class = %v, want %v", info.Class, EdgeDirectional)
	}
	if info.UpstreamRole != "supplier" {
		t.Errorf("customer_supplier UpstreamRole = %q, want %q", info.UpstreamRole, "supplier")
	}
	if info.DownstreamRole != "customer" {
		t.Errorf("customer_supplier DownstreamRole = %q, want %q", info.DownstreamRole, "customer")
	}
	if info.Symmetric {
		t.Errorf("customer_supplier Symmetric = true, want false")
	}

	partnership, ok := LookupEdgeVerb("partnership")
	if !ok {
		t.Fatalf("LookupEdgeVerb(partnership) not found")
	}
	if !partnership.Symmetric {
		t.Errorf("partnership Symmetric = false, want true")
	}
	if partnership.Class != EdgeSymmetric {
		t.Errorf("partnership Class = %v, want %v", partnership.Class, EdgeSymmetric)
	}

	_, ok = LookupEdgeVerb("not_a_real_verb")
	if ok {
		t.Errorf("LookupEdgeVerb(not_a_real_verb) found, want not found")
	}

	verbs := EdgeVerbs()
	if len(verbs) != 8 {
		t.Fatalf("len(EdgeVerbs()) = %d, want 8", len(verbs))
	}

	for _, v := range verbs {
		wantSymmetric := v.Class == EdgeSymmetric
		if v.Symmetric != wantSymmetric {
			t.Errorf("verb %q: Symmetric = %v, want %v (Class = %v)", v.Verb, v.Symmetric, wantSymmetric, v.Class)
		}
	}
}
