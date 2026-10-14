package lang

import (
	"reflect"
	"testing"
)

func TestParseSourceComment(t *testing.T) {
	cases := []struct {
		in    string
		cites []Cite
		text  string
		ok    bool
	}{
		{"//@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types",
			[]Cite{{Repo: "re-monetization-framework", Path: "functions/eb_email_sender/eb_email_sender.go", Start: 5, End: 5}}, "one rule, eight detail-types", true},
		{"//@ eb_email_sender.go:5-40", []Cite{{Path: "eb_email_sender.go", Start: 5, End: 40}}, "", true},
		{"//@ sqs.tf:3,26", []Cite{{Path: "sqs.tf", Start: 3, End: 3}, {Path: "sqs.tf", Start: 26, End: 26}}, "", true},
		{"//@ a/b.go:10, :40-42, c/d.go", []Cite{{Path: "a/b.go", Start: 10, End: 10}, {Path: "a/b.go", Start: 40, End: 42}, {Path: "c/d.go"}}, "", true},
		{"//@ repo a/b.go:10, :12", []Cite{{Repo: "repo", Path: "a/b.go", Start: 10, End: 10}, {Repo: "repo", Path: "a/b.go", Start: 12, End: 12}}, "", true},
		{"//@ pkg/siren/siren.go:63-115 -- a 423 for a suppressed address counts as delivered",
			[]Cite{{Path: "pkg/siren/siren.go", Start: 63, End: 115}}, "a 423 for a suppressed address counts as delivered", true},
		{"//@ pkg/render.go:40-12 -- reversed", nil, "", false},
		{"//@ eb_email_sender.go:5 one rule", nil, "", false},
		{"//@ README", nil, "", false},
		{"//@", nil, "", false},
		{"//@ :12", nil, "", false},
		{"//@ a.go:", nil, "", false},
		{"//@ a.go:0", nil, "", false},
		{"//@ Repo a.go", nil, "", false},
		{"// a.go:1", nil, "", false},
	}
	for _, c := range cases {
		cites, text, ok := ParseSourceComment(c.in)
		for i := range cites {
			cites[i].At, cites[i].Len = 0, 0 // spans are checked below
		}
		if ok != c.ok || text != c.text || !reflect.DeepEqual(cites, c.cites) {
			t.Errorf("%q: got %+v %q %v", c.in, cites, text, ok)
		}
	}
	cites, _, _ := ParseSourceComment("//@ repo a/b.go:5")
	if got := "//@ repo a/b.go:5"[cites[0].At : cites[0].At+cites[0].Len]; got != "repo a/b.go:5" {
		t.Errorf("span = %q", got)
	}
}
