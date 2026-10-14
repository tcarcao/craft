package analysis

import "testing"

const sourcesSrc = `use_case "nMF mails the seller about an add-on event" {
    //@ uc/top.go:1
    when nmf_monetization listens nmfbus.AddOnPurchased
        //@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types
        nmf_monetization picks the mail resolver from the event type alone
        // a plain comment does not break the attachment
        //@ eb_email_sender.go:5-40
        alt "suppressed"
            nmf_monetization asks Siren to send the purchase mail  [POST /api/v1/event]
            //@ pkg/siren/siren.go:63-115 -- a 423 counts as delivered
            note "no retry"
            //@ notes.md:3
}
`

func TestAttachSources(t *testing.T) {
	uc := File("s.craft", sourcesSrc).Doc.UseCases[0]
	if len(uc.Sources) != 1 || uc.Sources[0].Path != "uc/top.go" {
		t.Errorf("use case sources = %+v", uc.Sources)
	}
	sc := uc.Scenarios[0]
	if got := sc.Trigger.Sources; len(got) != 1 || got[0].Repo != "re-monetization-framework" || got[0].Text != "one rule, eight detail-types" || got[0].Line != 4 {
		t.Errorf("trigger sources = %+v", got)
	}
	picks := sc.Steps[0].Action
	if len(picks.Sources) != 1 || picks.Sources[0].End != 40 {
		t.Errorf("action sources = %+v", picks.Sources)
	}
	br := sc.Steps[1].Block.Branches[0]
	if len(br.Sources) != 0 {
		t.Errorf("branch header has no cite under it: %+v", br.Sources)
	}
	if asks := br.Steps[0].Action; len(asks.Sources) != 1 || asks.Sources[0].Path != "pkg/siren/siren.go" {
		t.Errorf("asks sources = %+v", asks.Sources)
	}
	if note := br.Steps[1].Note; len(note.Sources) != 1 || note.Sources[0].Path != "notes.md" {
		t.Errorf("note sources = %+v", note.Sources)
	}
}

func TestAttachSources_BranchHeadersSplitParAndDetached(t *testing.T) {
	src := `//@ head.go:1
use_case "U" {
    when s does it
        alt "a"
            //@ alt.go:1
            s does a
        else "b"
            //@ else.go:1
            s does b
        par "fan"
            //@ par.go:1
            s does c
            s does d
}
//@ between.go:1
use_case "V" {
    when s does it
        s does e
}
`
	res := File("s.craft", src)
	uc := res.Doc.UseCases[0]
	if len(uc.Sources) != 0 {
		t.Errorf("a cite above use_case is detached: %+v", uc.Sources)
	}
	alt := uc.Scenarios[0].Steps[0].Block
	if got := alt.Branches[0].Sources; len(got) != 1 || got[0].Path != "alt.go" {
		t.Errorf("alt sources = %+v", got)
	}
	if got := alt.Branches[1].Sources; len(got) != 1 || got[0].Path != "else.go" {
		t.Errorf("else sources = %+v", got)
	}
	par := uc.Scenarios[0].Steps[1].Block
	if got := par.Branches[0].Sources; len(got) != 1 || got[0].Path != "par.go" {
		t.Errorf("split par sources = %+v", got)
	}
	if got := res.Doc.UseCases[1]; len(got.Sources) != 0 || len(got.Scenarios[0].Trigger.Sources) != 0 {
		t.Errorf("a cite between use cases is detached: %+v", got)
	}
	n := 0
	for _, d := range res.Diagnostics {
		if d.Code == "craft/lint/detached-source" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("detached-source = %d, want 2: %+v", n, res.Diagnostics)
	}
}
