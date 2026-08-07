package lsp

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// The braces here are minified and the statements are not, which is exactly
// the split the formatter supports. Every brace is placed by the formatter, so
// `domains{` becomes `domains {` and `}Commerce{` breaks. The statements keep
// the author's line breaks, because a statement boundary is not something the
// token stream carries: see
// TestSeparatorFor_SeveralStatementsOnOneLineStayThere.
func TestFormatDocument_DomainsBlock(t *testing.T) {
	input := "domains{Auth{Login\nRegister}Commerce{Cart\nCheckout}}"
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
	input := "service Foo{language:golang\ncontexts:Auth,Profile}"
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
	input := "actors{user Alice\nsystem Bot}"
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
	input := "service \"Order Service\"{language:golang\ncontexts:Cart,Checkout}"
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
// The formatter used to re-render each trigger and action from typed
// accessors rather than copying source text, so any accessor that dropped or
// rewrote part of the line silently rewrote the user's file. Three separate
// defects of exactly that shape shipped before this test existed: a qualified
// ref got split into "re / billing", a typed event ref got requoted into the
// deprecated string form, and a trailing [POST /v1/charges] annotation was
// deleted outright.
//
// The token walker cannot reintroduce any of them, because it copies token
// text verbatim and re-renders nothing. These cases stay as the lock on that.
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
		// Single-line runs align to their own length + 2, i.e. two spaces
		// before "[". These two fixtures were single-space before the
		// alignment pass existed; they are updated here to their new
		// canonical (already-aligned) form so this guard keeps asserting
		// byte-identical output instead of being weakened.
		{"sync action with annotation", "use_case \"X\" {\n  when U does x\n    Auth asks DB to check email  [POST /v1/check]\n}\n"},
		{"async action with typed event ref", "use_case \"X\" {\n  when U does x\n    Billing notifies billing.ChargeSucceeded\n}\n"},
		{"async action with legacy quoted event", "use_case \"X\" {\n  when U does x\n    Billing notifies \"Order Created\"\n}\n"},
		{"async action with annotation", "use_case \"X\" {\n  when U does x\n    Billing notifies billing.ChargeSucceeded  [GRPC Pay]\n}\n"},
		// Multi-line run, already column-aligned: the shorter second line is
		// padded out to the longer first line's column + 2. Byte-identical
		// output here is what proves alignment is a no-op on already-aligned
		// input, not just on single-annotation lines.
		{"multi-line run already aligned", "use_case \"X\" {\n  when CRON detects a failed charge\n" +
			"    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]\n" +
			"    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]\n}\n"},
		// A non-annotated action line inside the run is left un-padded, but it
		// does not reset the alignment column for the annotated lines that
		// follow it: the whole scenario's action block is one run.
		{"non-annotated action inside run does not break alignment", "use_case \"X\" {\n  when CRON detects a failed charge\n" +
			"    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]\n" +
			"    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]\n" +
			"    Billing asks Ledger to record the entry\n" +
			"    Gateway returns to Billing the authorization result    [200 AuthorizationResult]\n}\n"},
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

// formatSource calls the same entry point the LSP formatting handler uses.
func formatSource(t *testing.T, src string) string {
	t.Helper()
	got, blocked := FormatDocumentChecked(src)
	// FormatDocument discards the diagnostic, and a drift refusal returns the
	// input unchanged, so a fixture written in canonical form would still
	// compare equal and pass while the formatter was in fact refusing to run.
	// Every byte-identity assertion built on this helper would go hollow at
	// exactly the moment it mattered. Declining a genuinely broken parse is
	// still fine and is left to the caller.
	if blocked != nil && blocked.Code == "craft/internal/formatter-content-drift" {
		t.Fatalf("contentDrift fired: the formatter refused to format, so any byte-identity assertion below would pass vacuously\nsrc:\n%s", src)
	}
	return got
}

// TestFormatDocument_AlignsOpAnnotations is the RED/GREEN case for the
// alignment pass: two annotations at mismatched columns must be pushed out to
// a shared column (the longer line's length + 2), and the unannotated third
// action must be left alone.
func TestFormatDocument_AlignsOpAnnotations(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
}
`
	want := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
}
`
	got := formatSource(t, src)
	if got != want {
		t.Errorf("format mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
	if _, _, diags := syntax.Parse(got); len(diags) != 0 {
		t.Errorf("aligned output does not parse cleanly: %+v\ngot:\n%s", diags, got)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
	wantShapes, haveShapes := shapesOf(t, src), shapesOf(t, got)
	if !reflect.DeepEqual(wantShapes, haveShapes) {
		t.Errorf("alignment changed the parsed model\nbefore: %+v\nafter:  %+v", wantShapes, haveShapes)
	}
}

// TestFormatDocument_AlignmentIsIdempotent formats an already-aligned document
// a second time and requires no further change: alignment must be a fixed
// point, not a moving target on repeated saves.
func TestFormatDocument_AlignmentIsIdempotent(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
}
`
	once := formatSource(t, src)
	twice := formatSource(t, once)
	if once != twice {
		t.Errorf("format is not idempotent\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

// TestFormatDocument_BlankLineResetsAnnotationRun proves a run does not reach
// across scenarios: the second scenario's short action line must align to its
// own length, not stretch out to the far column of the first scenario's long
// one.
func TestFormatDocument_BlankLineResetsAnnotationRun(t *testing.T) {
	src := `use_case "Retry" {
  when A does x
    Subscriptions asks Billing for a very long fresh charge attempt [POST /v1/charges]

  when B does y
    A asks C for d [GET /v1/d]
}
`
	got := formatSource(t, src)
	if !strings.Contains(got, "A asks C for d  [GET /v1/d]") {
		t.Errorf("second run should align independently, got:\n%s", got)
	}
}

// TestFormatDocument_NonAnnotatedActionInsideRunKeepsAlignment covers the
// motivating example from the task brief: a scenario's action block is a
// single run even when one of its lines carries no annotation. That line is
// left un-padded, but it does not reset the alignment column for the
// annotated lines around it, so the whole block still reads as one column.
//
// This is a deliberate reading of "a line that is not an annotated action
// ends the run": within one scenario there is no line between actions other
// than other actions (blank lines and `when` only ever appear at a scenario
// boundary), so the run is scoped to the scenario's action list, and a
// non-annotated action inside it is inert rather than a boundary.
func TestFormatDocument_NonAnnotatedActionInsideRunKeepsAlignment(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
    Gateway returns to Billing the authorization result [200 AuthorizationResult]
}
`
	want := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
    Gateway returns to Billing the authorization result    [200 AuthorizationResult]
}
`
	got := formatSource(t, src)
	if got != want {
		t.Errorf("format mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

// TestFormatDocument_NoAnnotationsUnaffected is the assertion-1 control: a
// scenario with no operation annotations at all must format identically to
// how it did before the alignment pass existed.
func TestFormatDocument_NoAnnotationsUnaffected(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    Auth asks DB to check email\n    Auth validates the result\n}\n"
	got := formatSource(t, src)
	if got != src {
		t.Errorf("annotation-free scenario must be unchanged\nwant: %q\ngot:  %q", src, got)
	}
}

// TestFormatDocument_PhraseBraceIsNotReformatted is the round-trip fixture for
// the phrase-brace regression (C3).
//
// `charge {amount} [POST /pay]` used to parse with zero diagnostics, an
// Operation of "[POST /pay", and format to
// `charge {amount  [[POST /pay]`, with the `}` deleted and the `[` duplicated,
// on a document that reported clean, so the formatter's bail-out never engaged.
//
// The parser now emits a not-yet-implemented warning for the leftover bracket,
// and the formatter treats that code as a bail-out signal, so the document is
// returned byte-identical instead of being rewritten from an incomplete tree.
func TestFormatDocument_PhraseBraceIsNotReformatted(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    Billing asks Gateway to charge {amount} [POST /pay]\n}\n"

	_, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic so the formatter has something to bail on")
	}

	got := formatSource(t, src)
	if got != src {
		t.Errorf("a document the parser could not fully place must be left alone\nwant: %q\ngot:  %q", src, got)
	}
	if strings.Contains(got, "[[") {
		t.Errorf("formatter duplicated the opening bracket:\n%s", got)
	}
	if !strings.Contains(got, "{amount}") {
		t.Errorf("formatter dropped the phrase brace:\n%s", got)
	}
}

// TestFormatDocument_TemplatedPathStillAligns is the control for the fix
// above: braces INSIDE an annotation are still tracked, so a templated payload
// parses cleanly and takes part in alignment as normal.
func TestFormatDocument_TemplatedPathStillAligns(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c [POST /v1/accounts/{id}/charges]\n}\n"
	want := "use_case \"X\" {\n  when U does x\n    A asks B for c  [POST /v1/accounts/{id}/charges]\n}\n"
	got := formatSource(t, src)
	if got != want {
		t.Errorf("format mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
}

// TestFormatDocument_UnbalancedRBraceDoesNotPanic covers I2: a stray `}` only
// produces a warning-severity diagnostic, so it used to reach the renderer's
// RBrace branch with depth 0 and crash on `strings.Repeat("  ", -1)`. indentFor
// floors at zero for the same reason.
func TestFormatDocument_UnbalancedRBraceDoesNotPanic(t *testing.T) {
	src := "use_case \"X\" {\n when U does y\n A does {x}\n}\n"
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("FormatDocument panicked on an unbalanced }: %v", r)
		}
	}()
	if got := formatSource(t, src); got == "" {
		t.Error("expected non-empty output")
	}
}

// A negative depth is reachable with an unbalanced `}` through a top-level
// stray brace as well, so exercise that shape directly too rather than relying
// on the use_case route staying the same.
func TestFormatDocument_StrayTopLevelRBraceDoesNotPanic(t *testing.T) {
	src := "services {\n  A {\n    contexts: X\n  }\n}\n}\n"
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("FormatDocument panicked on a stray top-level }: %v", r)
		}
	}()
	formatSource(t, src)
}

// TestFormatDocument_ContextMapRoundTrip is the context_map half of the I1
// fix. Before it, formatDecl space-joined every token, so `re/billing` came
// back as `re / billing` and two edges collapsed onto one line, and the result
// did not parse.
func TestFormatDocument_ContextMapRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"bare endpoints", "context_map re {\n  billing customer_supplier vas\n  billing anticorruption_layer subscriptions\n  billing partnership vas\n}\n"},
		{"qualified endpoints", "context_map {\n  re/billing separate_ways legacy/reporting\n}\n"},
		{"empty block", "context_map {\n}\n"},
		{"comment between edges", "context_map re {\n  // upstream first\n  billing customer_supplier vas\n  billing partnership subscriptions\n}\n"},
		{"comment above block", "// strategic view\ncontext_map re {\n  billing partnership vas\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_GlossaryRoundTrip is the glossary half. A three-segment
// term node carries two slashes, so it was the worst hit.
func TestFormatDocument_GlossaryRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"two-segment terms", "glossary re {\n  billing/Invoice same_as subscriptions/Invoice\n  billing/dunning contrasts subscriptions/dunning\n}\n"},
		{"three-segment terms", "glossary {\n  re/billing/Invoice distinct_from legacy/reporting/Invoice\n}\n"},
		{"comment between relations", "glossary re {\n  billing/Invoice same_as subscriptions/Invoice\n  // the dunning pair is not the same concept\n  billing/dunning contrasts subscriptions/dunning\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_QualifiedFieldValuesSurvive covers the instance of the
// same defect that nobody had reported: a service field value is a ref too, so
// `repo: olxeu/realestate/subscriptions` was space-joined into
// `olxeu / realestate / subscriptions`, which does not parse. Fixing it
// centrally rather than per block covers exposure and domain values too, and
// the token walker now makes it structural: a ref has no gaps between its
// parts, so its separators are all empty and the parts concatenate.
func TestFormatDocument_QualifiedFieldValuesSurvive(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"service repo", "services {\n  SubscriptionsApi {\n    contexts: Subscriptions\n    catalog_ref: subscriptions-api\n    repo: olxeu/realestate/subscriptions\n  }\n}\n"},
		{"service contexts list", "services {\n  A {\n    contexts: X, Y\n    data-stores: db1, db2\n    language: golang\n  }\n}\n"},
		{"exposure fields", "exposure api {\n  to: Business_User\n  through: gateway\n}\n"},
		{"domain bounded contexts", "domain re {\n  billing\n  vas\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_CommentsSurvive covers the defect the corpus guard
// surfaced: Tokens() excludes line and block comments because they are trivia,
// so every renderer built on it silently deleted them. internal/visualizer/
// testdata/vas.craft lost all 47 of its comments.
func TestFormatDocument_CommentsSurvive(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"above a top-level block", "// leading\nservices {\n  A {\n    contexts: X\n  }\n}\n"},
		{"inside a nested block", "services {\n  // about A\n  A {\n    // about contexts\n    contexts: X\n  }\n}\n"},
		{"above a use_case", "// this is a comment\nuse_case \"X\" {\n  when U does x\n    A asks B for c\n}\n"},
		// The blank line the formatter puts before every scenario lands after a
		// leading comment rather than before it, because the comment is just the
		// token preceding the `when`. Canonical form therefore carries the blank
		// line here.
		{"above a scenario", "use_case \"X\" {\n  // first flow\n\n  when U does x\n    A asks B for c\n}\n"},
		{"above an action", "use_case \"X\" {\n  when U does x\n    // why this call\n    A asks B for c\n}\n"},
		{"after the last action", "use_case \"X\" {\n  when U does x\n    A asks B for c\n    // TODO: confirm subject\n}\n"},
		{"doc comment", "/// documented\nactor user Alice\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_TagsBlockSurvives covers a third instance of the same
// class, found by the corpus guard rather than by review: a tags { } block is
// a child of the use case and not of any scenario, so formatUseCaseDecl walked
// straight past it and the whole block was deleted.
func TestFormatDocument_TagsBlockSurvives(t *testing.T) {
	src := "use_case \"Renewal\" {\n" +
		"  tags {\n" +
		"    journey: re/renewal-flow\n" +
		"    owner: \"team billing\"\n" +
		"    tier: gold\n" +
		"  }\n" +
		"\n" +
		"  when Customer creates Account\n" +
		"    Billing notifies \"Account Created\"\n" +
		"}\n"
	assertFormatIsFaithful(t, src)
}

// assertFormatIsFaithful asserts the four properties that define a safe
// formatter, on a fixture already written in canonical form: byte-identical
// output, a clean reparse, idempotence, and no comment lost.
func assertFormatIsFaithful(t *testing.T, src string) {
	t.Helper()
	got := formatSource(t, src)
	if got != src {
		t.Errorf("canonical input must be returned unchanged\nwant:\n%s\ngot:\n%s", src, got)
	}
	if _, _, diags := syntax.Parse(got); len(diags) != 0 {
		t.Errorf("formatted output does not parse cleanly: %+v\ngot:\n%s", diags, got)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("format is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
	wantComments, haveComments := commentTexts(t, src), commentTexts(t, got)
	if !reflect.DeepEqual(wantComments, haveComments) {
		t.Errorf("comments lost\nwant: %q\nhave: %q", wantComments, haveComments)
	}
	assertContentPreserved(t, src, got)
}

// TestFormatDocument_CommentOnlyBlockBodies covers the degenerate shape the
// leading/trailing split does not otherwise reach: a block whose entire body
// is a comment, so there is no statement for the comment to attach to.
func TestFormatDocument_CommentOnlyBlockBodies(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"use_case", "use_case \"X\" {\n  // nothing modelled yet\n}\n"},
		{"context_map", "context_map re {\n  // no edges agreed yet\n}\n"},
		{"glossary", "glossary {\n  // terms pending\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_TrailingCommentsSurvive covers the positions the parser
// never gives a comment token kind to. Everything after the last real token is
// folded into a single Whitespace token, so a comment with nothing after it is
// present in the text and invisible to any filter on token kind. Measured
// before the fix: a file ending in a comment went 43 bytes to 24, and a
// comment-only file went 18 bytes to 0, both with exit 0.
func TestFormatDocument_TrailingCommentsSurvive(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"file ends in a comment", "actor user Alice\n\n// trailing note\n"},
		{"comment only", "// only a comment\n"},
		{"several trailing comments", "actor user Alice\n\n// a\n// b\n"},
		{"block comment at eof", "actor user Alice\n\n/* note */\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestFormatDocument_TrailingCommentWithoutFinalNewline is the shape the
// UTF-16 twin fixture uses, and the one the content-drift check caught first.
func TestFormatDocument_TrailingCommentWithoutFinalNewline(t *testing.T) {
	src := "actors {\n\t\tuser Bob\n}\n// fim café"
	got := formatSource(t, src)
	if !strings.Contains(got, "// fim café") {
		t.Errorf("trailing comment was dropped:\n%s", got)
	}
	assertContentPreserved(t, src, got)
}

// TestFormatDocument_RefAdjacentCommentsSurvive covers a comment the parser
// attached inside a Ref node. The atomic-ref branch used to run before the
// comment branch and rendered the ref through RefText, which reads the
// trivia-free token list, so the comment was swallowed by the very fix that
// stopped `re/billing` being split.
//
// The token walker has no atomic-ref branch to swallow it: a comment is a
// token like any other and is written where the author put it. The value that
// follows keeps the author's line break, which it must, because a line comment
// runs to end of line and would otherwise swallow the value. It sits at the
// block's own indent rather than a hanging indent, because the formatter is no
// longer the one splitting the field across two lines.
func TestFormatDocument_RefAdjacentCommentsSurvive(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"comment before a field value", "services {\n  A {\n    repo: // note\n    olxeu/realestate\n  }\n}\n"},
		{"comment inside a list", "services {\n  A {\n    contexts: X, // why\n    Y\n  }\n}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertFormatIsFaithful(t, tc.src)
		})
	}
}

// TestSquashWhitespace pins the comparison the invariant is built on.
func TestSquashWhitespace(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"re/billing", "re / billing", true},
		{"a b c", "a\n  b\tc\n", true},
		{"", "\n", true},
		{"a b", "a", false},
		{"a b", "b a", false},
		{"[POST /pay]", "[[POST /pay]", false},
	}
	for _, tc := range cases {
		if got := squashWhitespace(tc.a) == squashWhitespace(tc.b); got != tc.want {
			t.Errorf("squashWhitespace(%q) == squashWhitespace(%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestContentDrift_RefusesToLoseContent proves the safety net does what it is
// there for, without needing a live formatter bug to demonstrate it. This is
// the piece that protects against the construct nobody has written a branch
// for yet, so it needs its own test rather than only being exercised by
// whatever happens to be broken today.
func TestContentDrift_RefusesToLoseContent(t *testing.T) {
	if d := contentDrift("actor user Alice\n", "actor  user\n  Alice\n"); d != nil {
		t.Errorf("pure whitespace change must be allowed, got %+v", d)
	}
	d := contentDrift("actor user Alice\n// note\n", "actor user Alice\n")
	if d == nil {
		t.Fatal("dropping a comment must be refused")
	}
	if d.Code != "craft/internal/formatter-content-drift" {
		t.Errorf("Code = %q, want the drift code so the refusal is discoverable", d.Code)
	}
	if d := contentDrift("a\n", "a a\n"); d == nil {
		t.Error("duplicating content must be refused")
	}
	if d := contentDrift("a b\n", "b a\n"); d == nil {
		t.Error("reordering content must be refused")
	}
}

// assertContentPreserved is the invariant on its own: formatting changes
// whitespace and nothing else.
func assertContentPreserved(t *testing.T, in, out string) {
	t.Helper()
	if squashWhitespace(in) != squashWhitespace(out) {
		t.Errorf("formatting changed more than whitespace\nin:\n%s\nout:\n%s", in, out)
	}
}

// TestWriteTokens_PreservesEveryNonWhitespaceToken is the invariant this whole
// rewrite exists to make structural: the walker emits every non-whitespace
// token verbatim, exactly once, in order.
func TestWriteTokens_PreservesEveryNonWhitespaceToken(t *testing.T) {
	srcs := []string{
		"domain re {\n  Billing\n}\n",
		"services {\n  S {\n    contexts: A, B\n    repo: olxeu/realestate/subs\n  }\n}\n",
		"context_map {\n  re/billing customer_supplier re/vas\n}\n",
		"glossary re {\n  billing/Invoice same_as subs/Invoice\n}\n",
		"use_case \"X\" {\n  when U does x\n    A asks re/b for c  [POST /v1/x]\n}\n",
		// The five fixtures above are the constructs the old renderer already
		// had a branch for. The five below are exactly the constructs nobody
		// wrote one for, which is the class of defect this rewrite closes: the
		// walker has no branches at all, so there is nothing left to omit.
		"actor user Alice\n",
		"actors {\n  user Alice\n  system Cron\n}\n",
		"arch {\n  presentation:\n    WebApp\n}\n",
		"exposure default {\n  to: Alice\n  through: Gateway\n}\n",
		"import \"other.craft\"\n",
		"use_case \"X\" {\n  tags {\n    owner: team\n  }\n  when U does x\n    A does y\n}\n",
	}
	for _, src := range srcs {
		gn, _, _ := syntax.Parse(src)
		root := syntax.Root(gn)

		var want []string
		for _, tk := range root.AllTokens() {
			if tk.Kind() == syntax.SyntaxKindWhitespace || tk.Kind() == syntax.SyntaxKindEOF {
				continue
			}
			want = append(want, tk.Text())
		}

		var sb strings.Builder
		for el := range root.ChildrenIter() {
			if node, ok := el.(syntax.SyntaxNode); ok {
				writeTokens(&sb, node)
			}
		}

		var got []string
		outGn, _, _ := syntax.Parse(sb.String())
		for _, tk := range syntax.Root(outGn).AllTokens() {
			if tk.Kind() == syntax.SyntaxKindWhitespace || tk.Kind() == syntax.SyntaxKindEOF {
				continue
			}
			got = append(got, tk.Text())
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("token stream changed for %q\nwant %v\ngot  %v", src, want, got)
		}
	}
}

// TestWriteTokens_KeepsAuthorLineBreaks pins the fifth behaviour change: a
// value the author wrapped across lines stays wrapped.
func TestWriteTokens_KeepsAuthorLineBreaks(t *testing.T) {
	src := "services {\n  S {\n    contexts: A,\n      B\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	if !strings.Contains(sb.String(), "A,\n") {
		t.Errorf("author line break inside contexts was joined:\n%s", sb.String())
	}
}

// TestWriteTokens_KeepsTrailingCommentsInPlace pins the first behaviour change.
func TestWriteTokens_KeepsTrailingCommentsInPlace(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A notifies a.B  // note\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	out := sb.String()
	if !strings.Contains(out, "a.B  // note") && !strings.Contains(out, "a.B // note") {
		t.Errorf("trailing comment did not stay on its line:\n%s", out)
	}
}

// TestWriteTokens_WrappedColonAndCommaStayInSync pins the field colon and the
// comma to the same rule: a line break the author wrote after either one
// survives. This is one fixture wrapping both `contexts:` and its list, so
// the two branches in separatorFor cannot drift apart again without this
// test catching it, the way a comma-only fix once did.
func TestWriteTokens_WrappedColonAndCommaStayInSync(t *testing.T) {
	src := "services {\n  S {\n    contexts:\n    Authentication,\n    Profile\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	// writeTokens renders one declaration and never appends the document's
	// final newline (that is FormatDocumentChecked's job, not called here),
	// so the trailing "\n" is trimmed before the byte-for-byte comparison.
	want := strings.TrimSuffix(src, "\n")
	if got := sb.String(); got != want {
		t.Errorf("wrapped contexts list did not round-trip byte-for-byte:\nwant %q\ngot  %q", want, got)
	}
}

// TestWriteTokens_ScenarioBodyIndentsDeeper pins scenario depth: a `when` at
// brace depth 1 sits at the use_case's own level, but the lines after it,
// until the next `when` at that depth or the enclosing `}`, sit one level
// deeper. A brace-depth-only indent would put both at the same level.
func TestWriteTokens_ScenarioBodyIndentsDeeper(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c\n\n  when V does y\n    D asks E for f\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	got := sb.String()
	for _, want := range []string{"\n  when U does x", "\n    A asks B for c", "\n  when V does y", "\n    D asks E for f"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestWriteTokens_ExpandsMinifiedDeclaration pins the block-boundary rule at
// the writeTokens level: a minified declaration must expand onto multiple
// lines even though the author left no gaps to drive it. The rule forces a
// break after `{` and before `}`; it says nothing about the gap before `{`
// itself, so `Foo{` (an empty, adjacent gap) stays adjacent, consistent with
// the general empty-gap-preserved rule that also keeps `re/billing` joined.
func TestWriteTokens_ExpandsMinifiedDeclaration(t *testing.T) {
	src := "service Foo{contexts: A}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	got := sb.String()
	// `Foo {` with the space, not `Foo{`. The original assertion here was
	// written before the `curr == LBrace` mirror existed and encoded its
	// absence, so it certified a half-expanded declaration as correct.
	if !strings.Contains(got, "Foo {\n  contexts: A\n}") {
		t.Errorf("minified declaration did not expand:\n%q", got)
	}
}

// TestFormatDocument_ArchModifiersAreNotAligned is the regression lock for the
// alignment pass reaching the arch verbatim slice.
//
// `WebApp[ssl, cache]` is a component modifier list, not a trailing operation
// annotation, but splitAnnotation cannot tell them apart: both are a bracketed
// run at end of line. While alignment ran over the whole assembled document it
// padded these out to a column, which silently broke the standing guarantee
// that arch is reproduced byte for byte. Alignment now runs per declaration on
// the walker's own output, so it never sees this text.
func TestFormatDocument_ArchModifiersAreNotAligned(t *testing.T) {
	src := "arch ModifiedArch {\n" +
		"    presentation:\n" +
		"        WebApp[ssl, cache]\n" +
		"        MobileApp[auth:oauth, timeout:30s]\n" +
		"}\n"
	got := FormatDocument(src)
	if got != src {
		t.Errorf("arch must be reproduced verbatim\nwant:\n%s\ngot:\n%s", src, got)
	}
	if strings.Contains(got, "WebApp ") {
		t.Errorf("arch modifiers were column-aligned:\n%s", got)
	}
}

// TestFormatDocument_ClosingBraceBeforeWhenIsIdempotent covers an ordering bug
// in separatorFor. A `tags { }` block puts a `}` directly before a `when`.
// While the `prev == RBrace` rule ran first it returned a plain newline, and
// the scenario blank line only appeared on the second pass once that newline
// was in the gap, so formatting was not a fixed point for this shape. No file
// in the repo has it, which is why the corpus guard did not catch it.
func TestFormatDocument_ClosingBraceBeforeWhenIsIdempotent(t *testing.T) {
	src := "use_case \"X\" {\n  tags {\n    owner: billing\n  }when U does x\n    A asks B for c\n}\n"
	once := FormatDocument(src)
	twice := FormatDocument(once)
	if once != twice {
		t.Errorf("formatting is not idempotent for `}when`\nfirst:\n%s\nsecond:\n%s", once, twice)
	}
	if !strings.Contains(once, "  }\n\n  when U does x") {
		t.Errorf("a scenario after a tags block must still get its blank line:\n%s", once)
	}
}

// TestFormatDocument_SeveralStatementsOnOneLineRoundTrip pins the accepted
// limit of minified expansion end to end, not just at the separator. The
// formatter leaves these crammed lines alone, and doing so must still produce
// output that parses, is a fixed point, and has lost nothing.
func TestFormatDocument_SeveralStatementsOnOneLineRoundTrip(t *testing.T) {
	src := "actors{user Alice system Bot}\n"
	got := formatSource(t, src)
	if _, _, diags := syntax.Parse(got); len(diags) != 0 {
		t.Errorf("output does not parse cleanly: %+v\ngot:\n%s", diags, got)
	}
	if again := formatSource(t, got); again != got {
		t.Errorf("not a fixed point\nfirst:\n%s\nsecond:\n%s", got, again)
	}
	if !strings.Contains(got, "user Alice system Bot") {
		t.Errorf("the crammed statements should be left as written:\n%s", got)
	}
	assertContentPreserved(t, src, got)
}
