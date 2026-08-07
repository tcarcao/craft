package lsp

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestFormatDocument_DomainsBlock(t *testing.T) {
	input := `domains{Auth{Login Register}Commerce{Cart Checkout}}`
	got := FormatDocument(input)

	checks := []string{
		"domains {",
		"\n  Auth {",
		"\n    Login",
		"\n    Register",
		"\n  }",
		"\n  Commerce {",
		"\n    Cart",
		"\n    Checkout",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDocument: missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestFormatDocument_ServiceFields(t *testing.T) {
	input := `service Foo{language:golang contexts:Auth,Profile}`
	got := FormatDocument(input)

	checks := []string{
		"service Foo {",
		"\n  language: golang",
		"\n  contexts: Auth, Profile",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDocument: missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestFormatDocument_ActorsBlock(t *testing.T) {
	input := `actors{user Alice system Bot}`
	got := FormatDocument(input)

	checks := []string{
		"actors {",
		"\n  user Alice",
		"\n  system Bot",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDocument: missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestFormatDocument_TopLevelBlankLines(t *testing.T) {
	input := "domain Auth{Login}\nactors{user Alice}"
	got := FormatDocument(input)

	if !strings.Contains(got, "}\n\nactors") {
		t.Errorf("FormatDocument: want blank line between top-level declarations\ngot:\n%s", got)
	}
}

func TestFormatDocument_EmptyInput(t *testing.T) {
	got := FormatDocument("")
	if got != "\n" {
		t.Errorf("FormatDocument empty: got %q, want %q", got, "\n")
	}
}

func TestFormatDocument_ParseErrorPreservesContent(t *testing.T) {
	// Broken input (unclosed brace) should be returned unchanged.
	broken := "service Foo {\n  language: golang\n"
	got := FormatDocument(broken)
	if got != broken {
		t.Errorf("FormatDocument with parse error: want original content unchanged\ngot:\n%s", got)
	}
}

func TestFormatDocument_UseCaseFormatted(t *testing.T) {
	// Messy indentation: extra spaces on when, no blank line between scenarios.
	input := "use_case \"Registration\" {\n    when Actor initiates x\n      Auth validates email\n    when Actor does y\n      Auth validates token\n}"
	got := FormatDocument(input)
	checks := []string{
		"use_case \"Registration\" {",
		"\n  when Actor initiates x\n",
		"\n    Auth validates email\n",
		"\n\n  when Actor does y\n",
		"\n    Auth validates token\n",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDocument use_case: missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestFormatDocument_MixedUseCaseAndService(t *testing.T) {
	input := "service Foo{language:golang}\nuse_case \"Bar\" {\n  when Actor does x\n}"
	got := FormatDocument(input)
	if !strings.Contains(got, "}\n\nuse_case") {
		t.Errorf("FormatDocument: want blank line between service and use_case\ngot:\n%s", got)
	}
	if !strings.Contains(got, "service Foo {") {
		t.Errorf("FormatDocument: service should be formatted\ngot:\n%s", got)
	}
}

func TestFormatDocument_QuotedServiceNameRoundTrip(t *testing.T) {
	// "Order Service" has a space — must be quoted in output; losing quotes breaks DSL.
	input := `service "Order Service"{language:golang contexts:Cart,Checkout}`
	got := FormatDocument(input)
	checks := []string{
		`service "Order Service" {`,
		"\n  language: golang",
		"\n  contexts: Cart, Checkout",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("FormatDocument quoted service: missing %q\ngot:\n%s", want, got)
		}
	}
}

// TestFormatDocument_QualifiedRefRoundTrip is the regression lock for a
// formatter bug that turned a valid file into an invalid one.
//
// TriggerDecl.Description() rebuilt the trigger line by space-joining the
// node's flat tokens, which is a one-token-per-slot assumption in disguise: a
// qualified subject ("re/billing") and a ref-wrapped listens event
// ("re/OrderPlaced") each flatten to three leaf tokens, so the formatter
// emitted "re / billing" and the result no longer parsed. Formatting must be
// a fixed point on already-canonical input, and must never emit source that
// fails to parse.
func TestFormatDocument_QualifiedRefRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "qualified listens context",
			src:  "use_case \"X\" {\n  when re/billing listens vas.VasApplied\n    A does x\n}\n",
			want: "  when re/billing listens vas.VasApplied\n",
		},
		{
			name: "ref-wrapped listens event",
			src:  "use_case \"X\" {\n  when Billing listens re/OrderPlaced\n    A does x\n}\n",
			want: "  when Billing listens re/OrderPlaced\n",
		},
		{
			name: "qualified external actor",
			src:  "use_case \"X\" {\n  when re/billing creates the Account\n    A does x\n}\n",
			want: "  when re/billing creates the Account\n",
		},
		{
			name: "qualified action subject and targets",
			src: "use_case \"X\" {\n  when U does x\n" +
				"    re/billing asks re/ledger to record the entry\n" +
				"    re/subscriptions returns charge result to re/billing\n" +
				"    re/billing validates invoice format\n}\n",
			want: "    re/billing asks re/ledger to record the entry\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDocument(tc.src)
			if !strings.Contains(got, tc.want) {
				t.Errorf("missing %q\ngot:\n%s", tc.want, got)
			}
			if _, _, diags := syntax.Parse(got); len(diags) != 0 {
				t.Errorf("formatted output does not parse cleanly: %+v\ngot:\n%s", diags, got)
			}
			// Formatting is idempotent: the already-canonical source must be
			// a fixed point, so a second pass cannot drift further.
			if again := FormatDocument(got); again != got {
				t.Errorf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
			}
		})
	}
}
