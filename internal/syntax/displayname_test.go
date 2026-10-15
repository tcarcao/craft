package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestDisplayNames_Contexts(t *testing.T) {
	src := "domains {\n    billing {\n        nmf_monetization  \"nMF, the money engine\"\n        portfoliovalue\n        esc \"say \\\"hi\\\"\"\n    }\n}\ndomain Payments {\n  ledger \"the ledger\"\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	doms := syntax.AsFile(syntax.Root(g)).Domains()
	if len(doms) != 2 {
		t.Fatalf("domains = %d", len(doms))
	}
	bcs := doms[0].BoundedContexts()
	if len(bcs) != 3 {
		t.Fatalf("bcs = %d", len(bcs))
	}
	want := []struct{ name, dn string }{{"nmf_monetization", "nMF, the money engine"}, {"portfoliovalue", ""}, {"esc", `say "hi"`}}
	for i, w := range want {
		if got := bcs[i].Name().Text(); got != w.name {
			t.Errorf("bc %d name = %q, want %q", i, got, w.name)
		}
		dn, _ := bcs[i].DisplayName()
		if dn != w.dn {
			t.Errorf("bc %d display name = %q, want %q", i, dn, w.dn)
		}
	}
	if _, tok := bcs[1].DisplayName(); tok != nil {
		t.Error("a context without a display name has no string token")
	}
	if dn, _ := doms[1].BoundedContexts()[0].DisplayName(); dn != "the ledger" {
		t.Errorf("standalone domain display name = %q", dn)
	}
}

func TestDisplayNames_Actors(t *testing.T) {
	src := "actors {\n    user Seller \"the seller\"\n    scheduler CRON \"the nightly clock\"\n    system AdCore\n}\nactor system Gateway \"the payment gateway\"\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	actors := syntax.AsFile(syntax.Root(g)).Actors()
	want := map[string]string{"Seller": "the seller", "CRON": "the nightly clock", "AdCore": "", "Gateway": "the payment gateway"}
	if len(actors) != len(want) {
		t.Fatalf("actors = %d", len(actors))
	}
	for _, a := range actors {
		name := a.Name().Text()
		dn, _ := a.DisplayName()
		if dn != want[name] {
			t.Errorf("actor %s display name = %q, want %q", name, dn, want[name])
		}
	}
}

// A string that does not share the name's line is not a display name: it
// stays the error it was before 2.27.
func TestDisplayNames_StringOnNextLineIsAnError(t *testing.T) {
	for _, src := range []string{
		"domain B {\n  ctx\n  \"floating\"\n}\n",
		"actors {\n  user Seller\n  \"floating\"\n}\n",
	} {
		_, _, diags := syntax.Parse(src)
		if len(diags) == 0 || diags[0].Code != "craft/syntax/unexpected-token" {
			t.Errorf("%q: diags = %v", src, diags)
		}
	}
}

// A repeated context is dropped with its string, as the name alone was.
func TestDisplayNames_DuplicateContextDropsString(t *testing.T) {
	g, _, diags := syntax.Parse("domain B {\n  ctx \"one\"\n  ctx \"two\"\n}\n")
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diags)
	}
	bcs := syntax.AsFile(syntax.Root(g)).Domains()[0].BoundedContexts()
	if len(bcs) != 1 {
		t.Fatalf("bcs = %d", len(bcs))
	}
	if dn, _ := bcs[0].DisplayName(); dn != "one" {
		t.Errorf("display name = %q", dn)
	}
}

func TestDisplayNames_Lossless(t *testing.T) {
	src := "domains {\n  b {\n    x   \"X one\"  // note\n  }\n}\nactors {\n  user U \"u\" // c\n}\n"
	g, _, _ := syntax.Parse(src)
	got := ""
	for _, tok := range syntax.Root(g).AllTokens() {
		got += tok.Text()
	}
	if got != src {
		t.Fatalf("round trip:\n%q\n%q", got, src)
	}
}
