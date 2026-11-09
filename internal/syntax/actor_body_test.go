package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const actorBodySrc = `actors {
    system DescriptionGenerator "description generator" {
        ai: genai
        owner: "OLX Data"
        note "drafts the description"
    }
    system Siren "Siren"
}

actor system Dapi "OLX data API" { ai: ml }
actor system Agent {
    ai: agentic
}
`

func TestActorBody_Parse(t *testing.T) {
	g, li, diags := syntax.Parse(actorBodySrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	as := syntax.AsFile(syntax.Root(g)).Actors()
	if len(as) != 4 {
		t.Fatalf("actors = %d", len(as))
	}
	type want struct {
		name, kind, display, ai, owner string
		body                           bool
		notes, end                     int
	}
	for i, w := range []want{
		{"DescriptionGenerator", "system", "description generator", "genai", "OLX Data", true, 1, 6},
		{"Siren", "system", "Siren", "", "", false, 0, 7},
		{"Dapi", "system", "OLX data API", "ml", "", true, 0, 10},
		{"Agent", "system", "", "agentic", "", true, 0, 13},
	} {
		a := as[i]
		dn, _ := a.DisplayName()
		ai, _ := a.Property(lang.ActorAI)
		owner, _ := a.Property(lang.ActorOwner)
		if a.Name().Text() != w.name || a.ActorTypeValue() != w.kind || dn != w.display || ai != w.ai || owner != w.owner ||
			a.HasBody() != w.body || len(a.Notes()) != w.notes || a.EndLine(li) != w.end {
			t.Errorf("actor %d = %s %s %q ai=%q owner=%q body=%v notes=%d end=%d", i, a.Name().Text(), a.ActorTypeValue(), dn, ai, owner,
				a.HasBody(), len(a.Notes()), a.EndLine(li))
		}
	}
}

// An open-taxonomy kind with a body still reads its kind and name.
func TestActorBody_OpenTaxonomyKind(t *testing.T) {
	g, _, diags := syntax.Parse("actor partner Gateway {\n    owner: Adyen\n}\n")
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	a := syntax.AsFile(syntax.Root(g)).Actors()[0]
	if a.ActorTypeValue() != "partner" || a.Name().Text() != "Gateway" {
		t.Errorf("kind/name = %q %q", a.ActorTypeValue(), a.Name().Text())
	}
	if v, _ := a.Property(lang.ActorOwner); v != "Adyen" {
		t.Errorf("owner = %q", v)
	}
}

func TestActorBody_Diagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		codes     []string
	}{
		{"unknown key kept", "actor system G {\n    vendor: Toqan\n}\n", []string{"craft/syntax/unknown-actor-property"}},
		{"missing value", "actor system G {\n    ai:\n}\n", []string{"craft/syntax/unexpected-token"}},
		{"two properties on a line", "actor system G { ai: ml owner: x }\n", []string{"craft/syntax/unexpected-token"}},
		{"stray line", "actor system G {\n    something else\n}\n", []string{"craft/syntax/unexpected-token"}},
		{"unclosed at eof", "actor system G {\n    ai: ml\n", []string{"craft/syntax/unclosed-block"}},
		// A `{` on a later line is not a body, as before 2.37.
		{"brace on next line", "actors {\n    system G\n    {\n    }\n}\n", []string{"craft/syntax/unexpected-token", "craft/syntax/skipped-construct"}},
	} {
		_, _, diags := syntax.Parse(tc.src)
		var got []string
		for _, d := range diags {
			got = append(got, d.Code)
		}
		if len(got) != len(tc.codes) {
			t.Errorf("%s: codes = %v, want %v", tc.name, got, tc.codes)
			continue
		}
		for i := range got {
			if got[i] != tc.codes[i] {
				t.Errorf("%s: codes = %v, want %v", tc.name, got, tc.codes)
			}
		}
	}
}

// In an actors block an unclosed body ends at the next actor line, so the
// rest of the block still parses.
func TestActorBody_UnclosedInBlockRecovers(t *testing.T) {
	g, _, diags := syntax.Parse("actors {\n    system G {\n        ai: genai\n    system Siren\n}\n\nactor user Seller\n")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unclosed-block" {
		t.Fatalf("diags = %v", diags)
	}
	var names []string
	for _, a := range syntax.AsFile(syntax.Root(g)).Actors() {
		names = append(names, a.Name().Text())
	}
	if len(names) != 3 || names[0] != "G" || names[1] != "Siren" || names[2] != "Seller" {
		t.Errorf("actors = %v", names)
	}
}

// A duplicate key and an unknown key stay in the tree; Property reads the
// last known value.
func TestActorBody_LastWins(t *testing.T) {
	g, _, _ := syntax.Parse("actor system G {\n    ai: ml\n    ai: genai\n    vendor: x\n}\n")
	a := syntax.AsFile(syntax.Root(g)).Actors()[0]
	if len(a.Properties()) != 3 {
		t.Errorf("properties = %d", len(a.Properties()))
	}
	if v, _ := a.Property(lang.ActorAI); v != "genai" {
		t.Errorf("ai = %q", v)
	}
	if _, ok := a.Property("vendor"); ok {
		t.Error("unknown key read as a property")
	}
}
