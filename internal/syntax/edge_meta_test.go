package syntax

import "testing"

func TestEdgeVerbMetas_MatchKeywordsAndDirection(t *testing.T) {
	if len(EdgeVerbMetas()) != len(edgeKeywords) {
		t.Fatalf("metas=%d keywords=%d", len(EdgeVerbMetas()), len(edgeKeywords))
	}
	cs, ok := LookupEdgeVerbMeta("customer_supplier")
	if !ok || cs.Class != EdgeDirectional || cs.UpstreamRole != "supplier" || cs.DownstreamRole != "customer" {
		t.Fatalf("customer_supplier meta = %+v ok=%v", cs, ok)
	}
	if !EdgeVerbSymmetric("partnership") || EdgeVerbSymmetric("conformist") {
		t.Errorf("symmetry flags wrong")
	}
	for _, k := range edgeKeywords {
		if _, ok := LookupEdgeVerbMeta(k); !ok {
			t.Errorf("keyword %q missing from metas", k)
		}
	}
}
