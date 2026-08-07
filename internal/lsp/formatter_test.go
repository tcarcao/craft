package lsp

import (
	"reflect"
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

// scenarioShape is the semantic content of one scenario, used to prove that
// formatting preserves meaning and not merely that the output parses.
//
// The trigger is compared as well as the actions. An earlier version of this
// helper collected only actions, which is why the guard stayed green while the
// formatter was space-joining trigger phrases and turning "(1! & 2!)" into
// "( 1 ! & 2 ! )".
type scenarioShape struct {
	trigger triggerShape
	actions []actionShape
}

type triggerShape struct {
	kind, actor, context, event, phrase string
	eventQuoted                         bool
}

type actionShape struct {
	kind, subject, target, event, connector, phrase, op string
	eventQuoted                                         bool
}

func shapesOf(t *testing.T, src string) []scenarioShape {
	t.Helper()
	g, _, diags := syntax.Parse(src)
	for _, d := range diags {
		t.Fatalf("fixture does not parse: [%s] %s\nsrc:\n%s", d.Code, d.Message, src)
	}
	var out []scenarioShape
	for _, uc := range syntax.AsFile(syntax.Root(g)).UseCases() {
		for _, sc := range uc.Scenarios() {
			tr := sc.Trigger()
			shape := scenarioShape{trigger: triggerShape{
				kind: tr.Kind(), actor: tr.ActorName(), context: tr.ContextName(),
				event: tr.EventValue(), phrase: tr.PhraseText(), eventQuoted: tr.EventIsString(),
			}}
			for _, a := range sc.Actions() {
				shape.actions = append(shape.actions, actionShape{
					kind: a.Kind(), subject: a.SubjectName(), target: a.TargetName(),
					event: a.EventValue(), connector: a.ConnectorValue(),
					phrase: a.PhraseText(), op: a.OpText(), eventQuoted: a.EventIsString(),
				})
			}
			out = append(out, shape)
		}
	}
	return out
}

// TestFormatDocument_UseCaseRoundTrip is the guard for a whole class of
// formatter bugs, not one instance of it.
//
// formatUseCaseDecl does not copy source text; it re-renders each trigger and
// action from typed accessors. Any accessor that drops or rewrites part of the
// line therefore silently rewrites the user's file. Three separate defects of
// exactly that shape shipped before this test existed: a qualified ref got
// split into "re / billing", a typed event ref got requoted into the
// deprecated string form, and a trailing [POST /v1/charges] annotation was
// deleted outright.
//
// Each case is already in canonical form, so all four assertions below must
// hold: the output is byte-identical to the input, it parses clean, formatting
// is idempotent, and the parsed model is unchanged. The last one is what makes
// this a semantic guard rather than a spelling guard.
func TestFormatDocument_UseCaseRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"sync action", "use_case \"X\" {\n  when User creates Account\n    Auth asks DB to check email\n}\n"},
		{"sync action with annotation", "use_case \"X\" {\n  when U does x\n    Auth asks DB to check email [POST /v1/check]\n}\n"},
		{"async action with typed event ref", "use_case \"X\" {\n  when U does x\n    Billing notifies billing.ChargeSucceeded\n}\n"},
		{"async action with legacy quoted event", "use_case \"X\" {\n  when U does x\n    Billing notifies \"Order Created\"\n}\n"},
		{"async action with annotation", "use_case \"X\" {\n  when U does x\n    Billing notifies billing.ChargeSucceeded [GRPC Pay]\n}\n"},
		{"return action with target", "use_case \"X\" {\n  when U does x\n    Auth returns to User charge result\n}\n"},
		{"return action without target", "use_case \"X\" {\n  when U does x\n    Auth returns charge result\n}\n"},
		{"internal action", "use_case \"X\" {\n  when U does x\n    Auth validates email format\n}\n"},
		{"internal action with connector", "use_case \"X\" {\n  when U does x\n    Wallet creates an unconfirmed reservation\n}\n"},
		{"qualified refs in every slot", "use_case \"X\" {\n  when re/billing listens vas.VasApplied\n" +
			"    re/billing asks re/ledger to record the entry\n" +
			"    re/billing notifies billing.ChargeSucceeded\n" +
			"    re/subscriptions returns to re/billing charge result\n" +
			"    re/billing validates invoice format\n}\n"},
		{"event trigger", "use_case \"X\" {\n  when \"SomeEvent\"\n    A does x\n}\n"},
		{"cron trigger", "use_case \"X\" {\n  when cron \"0 * * * *\"\n    A does x\n}\n"},
		{"periodic trigger", "use_case \"X\" {\n  when every \"1h\"\n    A does x\n}\n"},
		{"legacy quoted listens event", "use_case \"X\" {\n  when Billing listens \"Charged\"\n    A does x\n}\n"},
		// Trigger phrases carry free text, so they must survive punctuation
		// that a space-joined token walk would pull apart. Without these the
		// fixture set is all punctuation-free and cannot fail on the trigger
		// side at all.
		{"trigger phrase with tight punctuation", "use_case \"X\" {\n  when User creates (1! & 2!)\n    A does x\n}\n"},
		{"trigger phrase with slash", "use_case \"X\" {\n  when User does and/or something\n    A does x\n}\n"},
		{"trigger phrase with comma", "use_case \"X\" {\n  when User creates Account, quickly\n    A does x\n}\n"},
		{"trigger phrase with qualified actor and punctuation", "use_case \"X\" {\n  when re/billing creates (1! & 2!)\n    A does x\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDocument(tc.src)
			if got != tc.src {
				t.Errorf("not a fixed point on canonical source\nwant: %q\ngot:  %q", tc.src, got)
			}
			if _, _, diags := syntax.Parse(got); len(diags) != 0 {
				t.Errorf("formatted output does not parse cleanly: %+v\ngot:\n%s", diags, got)
			}
			if again := FormatDocument(got); again != got {
				t.Errorf("format is not idempotent\nfirst:  %q\nsecond: %q", got, again)
			}
			want, have := shapesOf(t, tc.src), shapesOf(t, got)
			if !reflect.DeepEqual(want, have) {
				t.Errorf("formatting changed the parsed model\nbefore: %+v\nafter:  %+v", want, have)
			}
		})
	}
}
