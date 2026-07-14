package syntax_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/model"
	"github.com/tcarcao/craft/internal/syntax"
)

func parseRoot(t *testing.T, src string) syntax.SyntaxNode {
	t.Helper()
	g, _, _ := syntax.Parse(src)
	return syntax.Root(g)
}

func TestParseTree_ActorSyntaxTree(t *testing.T) {
	g, _, diags := syntax.Parse("actor user Alice")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node, got %d", len(actorNodes))
	}
	a := actorNodes[0]
	kw := a.ChildToken(syntax.SyntaxKindKwActor)
	if kw == nil || kw.Text() != "actor" {
		t.Errorf("missing actor keyword token")
	}
	actorType := a.ChildToken(syntax.SyntaxKindKwUser, syntax.SyntaxKindKwSystem, syntax.SyntaxKindKwService)
	if actorType == nil || actorType.Text() != "user" {
		t.Errorf("missing actor type token, got %v", actorType)
	}
	name := a.ChildToken(syntax.SyntaxKindIdent)
	if name == nil || name.Text() != "Alice" {
		t.Errorf("missing name token, got %v", name)
	}
}

func TestParseTree_DomainSyntaxTree(t *testing.T) {
	g, _, diags := syntax.Parse("domain Ordering { Cart Checkout }")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	domainNodes := tree.ChildNodes(syntax.SyntaxKindDomainDecl)
	if len(domainNodes) != 1 {
		t.Fatalf("expected 1 domain node, got %d", len(domainNodes))
	}
	d := domainNodes[0]
	name := d.ChildToken(syntax.SyntaxKindIdent)
	if name == nil || name.Text() != "Ordering" {
		t.Errorf("expected domain name Ordering, got %v", name)
	}
	bcs := d.ChildNodes(syntax.SyntaxKindBoundedContext)
	if len(bcs) != 2 {
		t.Errorf("expected 2 bounded context nodes, got %d", len(bcs))
	}
}

func TestParseTree_ActorCommentPreserved(t *testing.T) {
	tree := parseRoot(t, "// leading comment\nactor user Alice")
	actorNodes := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorNodes) != 1 {
		t.Fatalf("expected 1 actor node")
	}
	// The comment should appear as a child of the actor node (trivia).
	allToks := actorNodes[0].AllTokens()
	hasComment := false
	for _, tok := range allToks {
		if tok.Kind() == syntax.SyntaxKindLineComment {
			hasComment = true
		}
	}
	if !hasComment {
		t.Error("expected leading comment to appear in actor node's AllTokens()")
	}
}

func TestParseTree_DocCommentPreserved(t *testing.T) {
	src := "/// Doc for actor\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	actors := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	allToks := actors[0].AllTokens()
	hasDoc := false
	for _, tok := range allToks {
		if tok.Kind() == syntax.SyntaxKindDocComment {
			hasDoc = true
			break
		}
	}
	if !hasDoc {
		t.Error("expected leading doc comment to appear in actor node's AllTokens()")
	}
}

func TestParseTree_DocCommentNotMistakenForLineComment(t *testing.T) {
	src := "/// Doc\nactor system Foo"
	g, _, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	actors := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	for _, tok := range actors[0].AllTokens() {
		if tok.Kind() == syntax.SyntaxKindLineComment {
			t.Error("/// should produce SyntaxKindDocComment, not SyntaxKindLineComment")
		}
	}
}

func TestParseTree_ServiceSyntaxTree(t *testing.T) {
	g, _, diags := syntax.Parse("service UserService {\n    contexts: Auth\n}")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	svcNodes := tree.ChildNodes(syntax.SyntaxKindServiceDecl)
	if len(svcNodes) != 1 {
		t.Fatalf("expected 1 service node, got %d", len(svcNodes))
	}
	svc := svcNodes[0]
	kw := svc.ChildToken(syntax.SyntaxKindKwService)
	if kw == nil || kw.Text() != "service" {
		t.Errorf("missing service keyword token")
	}
	name := svc.ChildToken(syntax.SyntaxKindIdent)
	if name == nil || name.Text() != "UserService" {
		t.Errorf("missing service name token, got %v", name)
	}
}

func TestParseTree_UseCaseSyntaxTree(t *testing.T) {
	// "when Customer initiates payment" is the trigger (Customer=actor, initiates=verb, payment=phrase).
	// "PaymentService asks Bank to process" is an action line (asks=action verb).
	src := "use_case \"Pay\" {\n    when Customer initiates payment\n        PaymentService asks Bank to process\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	ucNodes := tree.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	scenarios := ucNodes[0].ChildNodes(syntax.SyntaxKindScenario)
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	whenTok := scenarios[0].ChildToken(syntax.SyntaxKindKwWhen)
	if whenTok == nil {
		t.Error("expected when keyword token in scenario")
	}
	actions := scenarios[0].ChildNodes(syntax.SyntaxKindAction)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	verbTok := actions[0].ChildToken(syntax.SyntaxKindKwAsks)
	if verbTok == nil {
		t.Error("expected asks verb token in action")
	}
}

func TestParseTree_ArchSyntaxTree(t *testing.T) {
	src := "arch MyArch {\n    presentation: ComponentA\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	archNodes := tree.ChildNodes(syntax.SyntaxKindArchDecl)
	if len(archNodes) != 1 {
		t.Fatalf("expected 1 arch node, got %d", len(archNodes))
	}
	sections := archNodes[0].ChildNodes(syntax.SyntaxKindArchSection)
	if len(sections) != 1 {
		t.Fatalf("expected 1 arch section, got %d", len(sections))
	}
	presentationKw := sections[0].ChildToken(syntax.SyntaxKindKwPresentation)
	if presentationKw == nil {
		t.Error("expected presentation keyword token")
	}
}

func TestParseTree_ExposureSyntaxTree(t *testing.T) {
	src := "exposure MyExposure {\n    to: Business_User\n    through: rest\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	exposures := tree.ChildNodes(syntax.SyntaxKindExposureDecl)
	if len(exposures) != 1 {
		t.Fatalf("expected 1 exposure node, got %d", len(exposures))
	}
	rules := exposures[0].ChildNodes(syntax.SyntaxKindDeploymentRule)
	if len(rules) < 1 {
		t.Fatalf("expected at least 1 deployment rule, got %d", len(rules))
	}
	throughTok := rules[0].ChildToken(syntax.SyntaxKindKwThrough)
	if throughTok == nil {
		t.Error("expected through keyword token")
	}
}

func TestParseTree_ActorsBlockSyntaxTree(t *testing.T) {
	src := "actors {\n    user Alice\n    system Bob\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	blockNodes := tree.ChildNodes(syntax.SyntaxKindActorsBlock)
	if len(blockNodes) != 1 {
		t.Fatalf("expected 1 actors block node, got %d", len(blockNodes))
	}
	block := blockNodes[0]
	kw := block.ChildToken(syntax.SyntaxKindKwActors)
	if kw == nil || kw.Text() != "actors" {
		t.Errorf("missing actors keyword token")
	}
	actorDecls := block.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actorDecls) != 2 {
		t.Errorf("expected 2 actor decl nodes inside block, got %d", len(actorDecls))
	}
}

func TestRecovery_TriggerBadSubjectConsumed(t *testing.T) {
	// A bad token in the trigger subject position must not leave the token
	// stream stuck, which would cause the action loop to spin on the same token.
	src := "use_case \"X\" {\n  when 42 bad stuff\n    Domain asks Other to something\n}"
	gn, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic for bad trigger subject")
	}
	root := syntax.Root(gn)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	// Parser must not loop: the use_case node must be closed (test completes without hanging).
}

func TestParseTree_CronTrigger(t *testing.T) {
	src := "use_case \"Nightly\" {\n  when cron \"0 0 * * *\"\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	scenarios := ucNodes[0].ChildNodes(syntax.SyntaxKindScenario)
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	trigger := scenarios[0].ChildNodes(syntax.SyntaxKindTrigger)
	if len(trigger) != 1 {
		t.Fatalf("expected 1 trigger node, got %d", len(trigger))
	}
	cronKw := trigger[0].ChildToken(syntax.SyntaxKindKwCron)
	if cronKw == nil || cronKw.Text() != "cron" {
		t.Errorf("expected cron keyword token in trigger, got %v", cronKw)
	}
}

func TestParseTree_EveryTrigger(t *testing.T) {
	src := "use_case \"Polling\" {\n  when every \"5m\"\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	ucDecls := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucDecls) == 0 {
		t.Fatal("expected 1 use_case node")
	}
	scenarios := ucDecls[0].ChildNodes(syntax.SyntaxKindScenario)
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	trigger := scenarios[0].ChildNodes(syntax.SyntaxKindTrigger)
	if len(trigger) == 0 {
		t.Fatal("expected 1 trigger node")
	}
	everyKw := trigger[0].ChildToken(syntax.SyntaxKindKwEvery)
	if everyKw == nil || everyKw.Text() != "every" {
		t.Errorf("expected every keyword token in trigger, got %v", everyKw)
	}
}

func TestParseTree_CronTriggerMissingString(t *testing.T) {
	src := "use_case \"Bad\" {\n  when cron\n}"
	_, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Error("expected diagnostic for missing cron expression string")
	}
}

func TestParseTree_ImportDecl(t *testing.T) {
	src := `import "services/payments.craft"`
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	imports := root.ChildNodes(syntax.SyntaxKindImportDecl)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import node, got %d", len(imports))
	}
	kwTok := imports[0].ChildToken(syntax.SyntaxKindKwImport)
	if kwTok == nil || kwTok.Text() != "import" {
		t.Errorf("missing import keyword token")
	}
	// pathTok.Text() is the raw source text and includes both quotes (Bug 8a
	// fix): the green tree's Text is now byte-for-byte exact source, no
	// longer the lexer's unescaped, quote-stripped Value.
	pathTok := imports[0].ChildToken(syntax.SyntaxKindString)
	if pathTok == nil || pathTok.Text() != `"services/payments.craft"` {
		t.Errorf("missing or wrong import path token, got %v", pathTok)
	}
}

func TestParseTree_MultipleImports(t *testing.T) {
	src := "import \"a.craft\"\nimport \"b.craft\"\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	imports := root.ChildNodes(syntax.SyntaxKindImportDecl)
	if len(imports) != 2 {
		t.Fatalf("expected 2 import nodes, got %d", len(imports))
	}
	actors := root.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor node, got %d", len(actors))
	}
}

func TestParseTree_ImportMissingPath(t *testing.T) {
	// import without a string path should produce a diagnostic and still parse the rest.
	src := "import\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Error("expected diagnostic for missing import path")
	}
	root := syntax.Root(g)
	actors := root.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor node after bad import, got %d", len(actors))
	}
}

// TestRecovery_NestedBraceInServiceField verifies that a service field whose
// value accidentally contains `{...}` does not cause the parser to consume
// the wrong `}` as the service block's closing brace, which would prevent
// subsequent services in the same block from being parsed.
func TestRecovery_NestedBraceInServiceField(t *testing.T) {
	src := `services {
  Foo {
    bad_field: {broken_value}
    language: golang
  }
  Bar {
    language: python
  }
}`
	gn, _, diags := syntax.Parse(src)
	root := syntax.Root(gn)

	services := syntax.AsFile(root).Services()
	if len(services) != 2 {
		t.Fatalf("want 2 services, got %d (diags: %v)", len(services), diags)
	}
	if name := services[1].Name(); name == nil || name.Text() != "Bar" {
		t.Errorf("want second service Bar, got %v", name)
	}
	if len(diags) == 0 {
		t.Error("want at least one diagnostic for bad_field, got none")
	}
}

func TestProse_SpecialCharsUnquoted(t *testing.T) {
	src := `use_case "x" {
  when User taps Button
    Auth asks Billing for 1! & 2! and/maybe *
}`
	gn, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	root := syntax.Root(gn)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	uc := syntax.AsUseCaseDecl(ucNodes[0])
	scenarios := uc.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	actions := scenarios[0].Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	act := actions[0]
	// Note: "for" is the sync_action connector word (see ConnectorValue()/
	// isConnectorWord), which PhraseText() has always excluded — Description()
	// re-adds it separately (see TestActionDecl_Description_ConnectorPreservation).
	// So the phrase here is everything after "for": the special characters.
	if got := act.PhraseText(); got != "1! & 2! and/maybe *" {
		t.Fatalf("prose = %q, want %q", got, "1! & 2! and/maybe *")
	}
	if got := act.ConnectorValue(); got != "for" {
		t.Fatalf("connector = %q, want %q", got, "for")
	}
}

// TestProse_TrailingCommentAfterWhitespaceIsSeparated verifies that a
// trailing "// TODO" preceded by whitespace is NOT swept into the action's
// prose phrase (unlike bare TokenError punctuation, e.g. a lone '/' — see
// TestProse_SpecialCharsUnquoted): collectPhrase stops at the comment token,
// so the action's Description() reads "Auth checks x" and the comment
// survives as trivia elsewhere in the tree.
func TestProse_TrailingCommentAfterWhitespaceIsSeparated(t *testing.T) {
	src := `use_case "x" {
  when User taps Button
    Auth checks x  // TODO
}`
	gn, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	root := syntax.Root(gn)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	uc := syntax.AsUseCaseDecl(ucNodes[0])
	scenarios := uc.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	actions := scenarios[0].Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	act := actions[0]
	if got := act.PhraseText(); got != "x" {
		t.Fatalf("phrase = %q, want %q (comment must not be swept into prose)", got, "x")
	}
	if got := act.Description(); got != "Auth checks x" {
		t.Fatalf("description = %q, want %q", got, "Auth checks x")
	}
	// The "// TODO" comment must still appear as trivia somewhere in the tree.
	hasComment := false
	for _, tok := range root.AllTokens() {
		if tok.Kind() == syntax.SyntaxKindLineComment && strings.Contains(tok.Text(), "TODO") {
			hasComment = true
			break
		}
	}
	if !hasComment {
		t.Error("expected trailing '// TODO' comment to be preserved as trivia in the tree")
	}
}

// TestTypedRefs_NotifiesListensAsks is the Task 4 regression lock: it wires
// parseRef (Task 3) into notifies/listens/asks-target object positions and
// asserts a kind-prefixed slug (bc:re/billing) round-trips exactly through
// TargetName() — NOT truncated to the kind word "bc", which is what a naive
// ChildToken(SyntaxKindIdent)/Name() call on the ref node would yield.
func TestTypedRefs_NotifiesListensAsks(t *testing.T) {
	src := `use_case "x" {
  when Subscriptions listens vas.VasApplied
    Fulfillment asks bc:re/billing to record outcome
    Fulfillment notifies vas.VasFulfilled
}`
	gn, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	root := syntax.Root(gn)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	uc := syntax.AsUseCaseDecl(ucNodes[0])
	scenarios := uc.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	scenario := scenarios[0]

	// listens trigger ref
	trigger := scenario.Trigger()
	if got := trigger.Kind(); got != "domain_listen" {
		t.Fatalf("trigger kind = %q, want domain_listen", got)
	}
	if got := trigger.EventValue(); got != "vas.VasApplied" {
		t.Errorf("listens ref = %q, want %q", got, "vas.VasApplied")
	}

	actions := scenario.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	// asks target ref (kind-prefixed slug — the landmine case)
	asks := actions[0]
	if got := asks.Kind(); got != "sync_action" {
		t.Fatalf("action[0] kind = %q, want sync_action", got)
	}
	if got := asks.TargetName(); got != "bc:re/billing" {
		t.Errorf("asks target ref = %q, want %q (must NOT be truncated to the kind word \"bc\")", got, "bc:re/billing")
	}
	if got := asks.PhraseText(); got != "record outcome" {
		t.Errorf("asks phrase = %q, want %q", got, "record outcome")
	}

	// notifies ref
	notifies := actions[1]
	if got := notifies.Kind(); got != "async_action" {
		t.Fatalf("action[1] kind = %q, want async_action", got)
	}
	if got := notifies.EventValue(); got != "vas.VasFulfilled" {
		t.Errorf("notifies ref = %q, want %q", got, "vas.VasFulfilled")
	}
}

// TestContextMap_Edges is the Task 5 TDD lock for the new `context_map { }`
// top-level block: edge_stmt := ref EDGE_KW ref, where EDGE_KW is a
// contextual keyword (realized_by/also_realizes/same_as/contrasts/
// distinct_from) matched by value, like asks/notifies. Asserts edges surface
// through the pkg/craft projection layer with Left/Right taken from
// RefText() — NOT Name(), which would truncate a kind-prefixed slug like
// "bc:re/subscriptions" down to just "bc".
func TestContextMap_Edges(t *testing.T) {
	src := `context_map {
  bc:re/subscriptions realized_by service:subscriptions-api
  term:subscriptions/dunning contrasts term:billing/dunning
}`
	gn, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Round-trip: the block must reassemble to the exact source text.
	if got := reassembleGreen(gn); got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}

	root := syntax.Root(gn)
	doc := syntax.ProjectFromTree(root, li)
	edges := doc.ContextMap
	if len(edges) != 2 {
		t.Fatalf("want 2 edges, got %d: %+v", len(edges), edges)
	}
	if edges[0].Left != "bc:re/subscriptions" || edges[0].Verb != "realized_by" || edges[0].Right != "service:subscriptions-api" {
		t.Fatalf("edge0 = %+v", edges[0])
	}
	if edges[1].Left != "term:subscriptions/dunning" || edges[1].Verb != "contrasts" || edges[1].Right != "term:billing/dunning" {
		t.Fatalf("edge1 = %+v", edges[1])
	}

	// Also verify the raw syntax-tree shape directly (belt-and-braces on the
	// AST layer, independent of the projection layer).
	cmNodes := root.ChildNodes(syntax.SyntaxKindContextMapDecl)
	if len(cmNodes) != 1 {
		t.Fatalf("expected 1 context_map node, got %d", len(cmNodes))
	}
	file := syntax.AsFile(root)
	cms := file.ContextMaps()
	if len(cms) != 1 {
		t.Fatalf("expected 1 ContextMapDecl view, got %d", len(cms))
	}
	astEdges := cms[0].Edges()
	if len(astEdges) != 2 {
		t.Fatalf("expected 2 EdgeDecl views, got %d", len(astEdges))
	}
	if got := astEdges[0].Left(); got != "bc:re/subscriptions" {
		t.Errorf("astEdges[0].Left() = %q, want %q", got, "bc:re/subscriptions")
	}
	if got := astEdges[0].Verb(); got != "realized_by" {
		t.Errorf("astEdges[0].Verb() = %q, want %q", got, "realized_by")
	}
	if got := astEdges[0].Right(); got != "service:subscriptions-api" {
		t.Errorf("astEdges[0].Right() = %q, want %q", got, "service:subscriptions-api")
	}
}

// parseWithHangGuard runs syntax.Parse on its own goroutine behind a short
// watchdog timeout, so a parser infinite loop fails this single test fast
// (a few seconds) instead of hanging the entire `go test` run until the
// outer test binary timeout kills it.
func parseWithHangGuard(t *testing.T, src string) (*green.GreenNode, green.LineIndex, []model.Diagnostic) {
	t.Helper()
	type result struct {
		gn    *green.GreenNode
		li    green.LineIndex
		diags []model.Diagnostic
	}
	done := make(chan result, 1)
	go func() {
		gn, li, diags := syntax.Parse(src)
		done <- result{gn, li, diags}
	}()
	select {
	case r := <-done:
		return r.gn, r.li, r.diags
	case <-time.After(5 * time.Second):
		t.Fatalf("syntax.Parse(%q) did not terminate within 5s (parser infinite loop)", src)
		return nil, green.LineIndex{}, nil
	}
}

// TestContextMap_HangRegression_BareKeywordLeftEndpoint locks in the Task 5
// fix for a parser infinite loop: a bare keyword-as-ident LEFT endpoint with
// no following ':' (the literal word "service", not "service:x") reaches
// parseEdgeStmt's endpoint gate (which accepts TokenIdent||isAnyKeywordAsIdent)
// but, before the fix, made parseRef() consume ZERO tokens — the kind-prefix
// branch requires an immediately-following ':', and the fallback loop only
// recognises TokenIdent/TokenNumber, not the keyword-as-ident token type a
// bare "service" lexes as. parseEdgeStmt then made zero progress, and
// parseContextMapBlock's `for !p.atEOF() && ...` loop called it again on the
// exact same position forever (this hung `go test` before the fix). The fix
// makes parseRef always consume its first token when called on an
// ident/keyword-as-ident start, guaranteeing forward progress.
func TestContextMap_HangRegression_BareKeywordLeftEndpoint(t *testing.T) {
	src := "context_map {\n  service realized_by service:x\n}"
	gn, _, diags := parseWithHangGuard(t, src)

	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic for malformed edge, got none")
	}
	// The parser must still fully consume the block through the closing `}`
	// rather than aborting partway (which would itself indicate leftover
	// unconsumed tokens from the malformed endpoint).
	if got := reassembleGreen(gn); got != src {
		t.Errorf("round-trip mismatch (parser did not fully consume input)\nwant: %q\ngot:  %q", src, got)
	}
}

// TestContextMap_HangRegression_BareKeywordRightEndpoint is the mirror image
// of TestContextMap_HangRegression_BareKeywordLeftEndpoint for the RIGHT
// endpoint: `term:x contrasts domain` — a valid left ref, a valid edge verb,
// then a bare keyword-as-ident right endpoint ("domain") with no colon. The
// same zero-progress hazard applied to the right endpoint's call to
// parseRef() before the fix.
func TestContextMap_HangRegression_BareKeywordRightEndpoint(t *testing.T) {
	src := "context_map {\n  term:x contrasts domain\n}"
	gn, _, diags := parseWithHangGuard(t, src)

	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic for malformed edge, got none")
	}
	if got := reassembleGreen(gn); got != src {
		t.Errorf("round-trip mismatch (parser did not fully consume input)\nwant: %q\ngot:  %q", src, got)
	}
}

// TestServiceAnchors covers Task 6: the optional `opslevel:` / `repo:`
// service properties. `repo:` is parsed via parseRef so a slash-bearing
// slug (e.g. "olxeu/realestate/subscriptions") is captured as one value.
func TestServiceAnchors(t *testing.T) {
	src := `services {
  SubscriptionsApi {
    contexts: Subscriptions
    opslevel: subscriptions-api
    repo: olxeu/realestate/subscriptions
  }
}`
	gn, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	// Round-trip: the block must reassemble to the exact source text.
	if got := reassembleGreen(gn); got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}

	root := syntax.Root(gn)
	doc := syntax.ProjectFromTree(root, li)
	if len(doc.Services) != 1 {
		t.Fatalf("want 1 service, got %d: %+v", len(doc.Services), doc.Services)
	}
	svc := doc.Services[0]
	if svc.OpsLevel != "subscriptions-api" || svc.Repo != "olxeu/realestate/subscriptions" {
		t.Fatalf("anchors = %q / %q", svc.OpsLevel, svc.Repo)
	}

	// Also verify the AST accessors directly.
	file := syntax.AsFile(root)
	astSvc := file.Services()[0]
	if got := astSvc.OpsLevel(); got != "subscriptions-api" {
		t.Errorf("ServiceDecl.OpsLevel() = %q, want %q", got, "subscriptions-api")
	}
	if got := astSvc.Repo(); got != "olxeu/realestate/subscriptions" {
		t.Errorf("ServiceDecl.Repo() = %q, want %q", got, "olxeu/realestate/subscriptions")
	}
}
