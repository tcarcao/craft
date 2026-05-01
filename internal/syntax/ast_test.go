package syntax_test

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/syntax"
)

// astParse is a small helper that parses src and returns the wrapped red root.
func astParse(src string) syntax.SyntaxNode {
	g, _, _ := syntax.Parse(src)
	return syntax.Root(g)
}

func TestActorDecl_View(t *testing.T) {
	tree := astParse("actor user Alice")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	a := actors[0]
	if a.Name() == nil || a.Name().Text() != "Alice" {
		t.Errorf("expected name Alice, got %v", a.Name())
	}
	if a.ActorType() == nil || a.ActorType().Kind() != syntax.SyntaxKindKwUser {
		t.Errorf("expected user type, got %v", a.ActorType())
	}
	if a.Keyword() == nil {
		t.Errorf("expected keyword token, got nil")
	}
}

func TestActorsBlock_View(t *testing.T) {
	tree := astParse("actors { user Alice  system Bob }")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 actors from block, got %d", len(actors))
	}
}

func TestDomainDecl_View(t *testing.T) {
	tree := astParse("domain Ordering { Cart Checkout }")
	file := syntax.AsFile(tree)
	domains := file.Domains()
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}
	d := domains[0]
	if d.Name() == nil || d.Name().Text() != "Ordering" {
		t.Errorf("expected name Ordering, got %v", d.Name())
	}
	bcs := d.BoundedContexts()
	if len(bcs) != 2 {
		t.Errorf("expected 2 bounded contexts, got %d", len(bcs))
	}
	if bcs[0].Name() == nil || bcs[0].Name().Text() != "Cart" {
		t.Errorf("expected first BC Cart, got %v", bcs[0].Name())
	}
}

func TestFile_Actors_DocumentOrder(t *testing.T) {
	src := "actor user Alice\nactors { system Bob }\nactor user Carol"
	tree := astParse(src)
	actors := syntax.AsFile(tree).Actors()
	if len(actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(actors))
	}
	names := []string{actors[0].Name().Text(), actors[1].Name().Text(), actors[2].Name().Text()}
	expected := []string{"Alice", "Bob", "Carol"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("actor[%d]: expected %q, got %q", i, expected[i], n)
		}
	}
}

func TestFile_SyntaxTree(t *testing.T) {
	tree := astParse("actor user Alice")
	file := syntax.AsFile(tree)
	// AsFile with a zero SyntaxNode must not panic and must yield no actors.
	zeroFile := syntax.AsFile(syntax.SyntaxNode{})
	if len(zeroFile.Actors()) != 0 {
		t.Error("zero tree should return empty actors")
	}
	_ = file
}

func TestServiceDecl_View(t *testing.T) {
	tree := astParse("service order-service { contexts: [Cart] }")
	file := syntax.AsFile(tree)
	services := file.Services()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	s := services[0]
	if s.Name() == nil || s.Name().Text() != "order-service" {
		t.Errorf("expected name order-service, got %v", s.Name())
	}
	if s.Keyword() == nil {
		t.Error("expected service keyword")
	}
}

func TestUseCaseDecl_View(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree := astParse(src)
	file := syntax.AsFile(tree)
	ucs := file.UseCases()
	if len(ucs) != 1 {
		t.Fatalf("expected 1 use_case, got %d", len(ucs))
	}
	uc := ucs[0]
	if uc.Title() == nil || uc.Title().Text() != "Pay" {
		t.Errorf("expected title Pay, got %v", uc.Title())
	}
	scenarios := uc.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
}

func TestScenarioDecl_View(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree := astParse(src)
	uc := syntax.AsFile(tree).UseCases()[0]
	scenario := uc.Scenarios()[0]

	when := scenario.When()
	if when == nil || when.Kind() != syntax.SyntaxKindKwWhen {
		t.Errorf("expected when token, got %v", when)
	}
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
}

func TestActionDecl_VerbPosition(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree := astParse(src)
	scenario := syntax.AsFile(tree).UseCases()[0].Scenarios()[0]
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action")
	}
	verb := actions[0].Verb()
	if verb == nil {
		t.Fatal("expected verb token")
	}
	if verb.Kind() != syntax.SyntaxKindKwAsks {
		t.Errorf("expected SyntaxKindKwAsks, got %v", verb.Kind())
	}
	// Position queries via LineIndex are exercised in Task 10; here we only check
	// that the verb token exists at a non-zero source offset.
	if verb.Offset() == 0 {
		t.Errorf("verb offset should be non-zero, got %d", verb.Offset())
	}
}

func TestArchDecl_View(t *testing.T) {
	src := "arch MyArch {\n    presentation: ComponentA\n    gateway: ComponentB\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	archs := syntax.AsFile(tree).Archs()
	if len(archs) != 1 {
		t.Fatalf("expected 1 arch, got %d", len(archs))
	}
	arch := archs[0]
	if arch.Name() == nil || arch.Name().Text() != "MyArch" {
		t.Errorf("expected arch name MyArch, got %v", arch.Name())
	}
	sections := arch.Sections()
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	// First section should be presentation
	kw := sections[0].Keyword()
	if kw == nil || kw.Kind() != syntax.SyntaxKindKwPresentation {
		t.Errorf("expected presentation keyword, got %v", kw)
	}
	components := sections[0].Components()
	if len(components) != 1 {
		t.Errorf("expected 1 component, got %d", len(components))
	}
	if components[0].Name() == nil || components[0].Name().Text() != "ComponentA" {
		t.Errorf("expected ComponentA, got %v", components[0].Name())
	}
}

func TestExposureDecl_View(t *testing.T) {
	src := "exposure MyExposure {\n    to: Customer\n    through: rest\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	exposures := syntax.AsFile(tree).Exposures()
	if len(exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(exposures))
	}
	e := exposures[0]
	if e.Name() == nil || e.Name().Text() != "MyExposure" {
		t.Errorf("expected name MyExposure, got %v", e.Name())
	}
	rules := e.Rules()
	if len(rules) < 1 {
		t.Fatalf("expected at least 1 rule, got %d", len(rules))
	}
	through := rules[0].Through()
	if through == nil || through.Kind() != syntax.SyntaxKindKwThrough {
		t.Errorf("expected through keyword, got %v", through)
	}
}

func TestActionDecl_Notifies(t *testing.T) {
	src := "use_case \"Notify\" {\n    when Customer submits NotifyForm\n    EmailService notifies \"UserNotified\"\n}"
	tree := astParse(src)
	scenario := syntax.AsFile(tree).UseCases()[0].Scenarios()[0]
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action")
	}
	verb := actions[0].Verb()
	if verb == nil || verb.Kind() != syntax.SyntaxKindKwNotifies {
		t.Errorf("expected SyntaxKindKwNotifies, got %v", verb)
	}
}

func TestServiceDecl_BodyAccessors(t *testing.T) {
	src := `services {
  PaymentService {
    contexts: Billing, Checkout
    data-stores: payments_db
    language: golang
  }
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	svcs := f.Services()
	if len(svcs) != 1 {
		t.Fatalf("want 1 service, got %d", len(svcs))
	}
	svc := svcs[0]
	if !svc.IsGrouped() {
		t.Error("want IsGrouped=true for service inside services{}")
	}
	if got := svc.Contexts(); !reflect.DeepEqual(got, []string{"Billing", "Checkout"}) {
		t.Errorf("Contexts: got %v", got)
	}
	if got := svc.DataStores(); !reflect.DeepEqual(got, []string{"payments_db"}) {
		t.Errorf("DataStores: got %v", got)
	}
	if got := svc.Language(); got != "golang" {
		t.Errorf("Language: got %q", got)
	}
}

func TestServiceDecl_StandaloneIsGrouped(t *testing.T) {
	src := `service UserService {
  contexts: Auth
  language: golang
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	svcs := f.Services()
	if len(svcs) == 0 {
		t.Fatal("want at least 1 service")
	}
	if svcs[0].IsGrouped() {
		t.Error("want IsGrouped=false for standalone service")
	}
}

func TestDomainDecl_BodyAccessors(t *testing.T) {
	src := `domain Payments {
  Billing
  Checkout
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	doms := f.Domains()
	if len(doms) != 1 {
		t.Fatalf("want 1 domain, got %d", len(doms))
	}
	d := doms[0]
	if d.IsGrouped() {
		t.Error("want IsGrouped=false for standalone domain")
	}
	// EndLine requires a LineIndex (Task 10); pass zero value — just verify it doesn't panic.
	_ = d.EndLine(green.LineIndex{})
}

func TestTriggerDecl_Kind(t *testing.T) {
	cases := []struct {
		src  string
		kind string
	}{
		{`use_case "X" { when Business_User creates Account }`, "external"},
		{`use_case "X" { when "User Registered" }`, "event"},
		{`use_case "X" { when Auth listens "User Registered" }`, "domain_listen"},
	}
	for _, tc := range cases {
		tree := astParse(tc.src)
		f := syntax.AsFile(tree)
		ucs := f.UseCases()
		if len(ucs) == 0 {
			t.Fatalf("no use cases parsed: %q", tc.src)
		}
		scenarios := ucs[0].Scenarios()
		if len(scenarios) == 0 {
			t.Fatalf("no scenarios: %q", tc.src)
		}
		got := scenarios[0].Trigger().Kind()
		if got != tc.kind {
			t.Errorf("src=%q: want Kind=%q got %q", tc.src, tc.kind, got)
		}
	}
}

func TestActionDecl_Kind(t *testing.T) {
	src := `use_case "X" {
  when User creates Account
    Auth asks DB to check email
    Auth notifies "Account Created"
    Auth returns result to User
    Auth validates email format
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	actions := f.UseCases()[0].Scenarios()[0].Actions()
	if len(actions) != 4 {
		t.Fatalf("want 4 actions, got %d", len(actions))
	}
	want := []string{"sync_action", "async_action", "return_action", "internal_action"}
	for i, a := range actions {
		if got := a.Kind(); got != want[i] {
			t.Errorf("action[%d]: want %q got %q", i, want[i], got)
		}
	}
}

func TestActionDecl_SubjectAndTarget(t *testing.T) {
	src := `use_case "X" {
  when User creates Account
    Auth asks DB to check email
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	actions := f.UseCases()[0].Scenarios()[0].Actions()
	if len(actions) == 0 {
		t.Fatal("no actions")
	}
	a := actions[0]
	if got := a.SubjectName(); got != "Auth" {
		t.Errorf("SubjectName: got %q", got)
	}
	if got := a.TargetName(); got != "DB" {
		t.Errorf("TargetName: got %q", got)
	}
	if got := a.Kind(); got != "sync_action" {
		t.Errorf("Kind: got %q", got)
	}
}

func TestArchDecl_LineAccessors(t *testing.T) {
	src := `arch MyArch {
  presentation:
    WebApp
  gateway:
    APIGateway
}`
	tree := astParse(src)
	f := syntax.AsFile(tree)
	archs := f.Archs()
	if len(archs) != 1 {
		t.Fatalf("want 1 arch, got %d", len(archs))
	}
	a := archs[0]
	// Position accessors require a LineIndex (Task 10). Pass zero value — just verify no panic.
	li := green.LineIndex{}
	_ = a.Line(li)
	_ = a.EndLine(li)
	_ = a.PresentationLine(li)
	_ = a.GatewayLine(li)
}

func TestServiceDecl_ContextTokens(t *testing.T) {
	src := `services {
	  Auth {
	    contexts: Login, Register
	    language: golang
	  }
	}`
	tree := astParse(src)
	file := syntax.AsFile(tree)
	svcs := file.Services()
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}
	toks := svcs[0].ContextTokens()
	if len(toks) != 2 {
		t.Fatalf("expected 2 context tokens (Login, Register), got %d: %v", len(toks), toks)
	}
	if toks[0].Text() != "Login" {
		t.Errorf("toks[0]: got %q want Login", toks[0].Text())
	}
	if toks[1].Text() != "Register" {
		t.Errorf("toks[1]: got %q want Register", toks[1].Text())
	}
}

func TestServiceDecl_ContextTokens_EdgeCases(t *testing.T) {
	// Service with no contexts: field at all.
	src1 := "services {\n  Svc {\n    language: golang\n  }\n}"
	tree1 := astParse(src1)
	file1 := syntax.AsFile(tree1)
	svcs1 := file1.Services()
	if len(svcs1) != 1 {
		t.Fatalf("src1: expected 1 service, got %d", len(svcs1))
	}
	toks1 := svcs1[0].ContextTokens()
	if len(toks1) != 0 {
		t.Errorf("no contexts field: expected 0 tokens, got %d: %v", len(toks1), toks1)
	}

	// Service with contexts: present but empty (next field follows immediately).
	src2 := "services {\n  Svc {\n    contexts:\n    language: golang\n  }\n}"
	tree2 := astParse(src2)
	file2 := syntax.AsFile(tree2)
	svcs2 := file2.Services()
	if len(svcs2) != 1 {
		t.Fatalf("src2: expected 1 service, got %d", len(svcs2))
	}
	toks2 := svcs2[0].ContextTokens()
	if len(toks2) != 0 {
		t.Errorf("empty contexts: expected 0 tokens, got %d: %v", len(toks2), toks2)
	}
}

func TestContextRefs_ReturnsRefDecls(t *testing.T) {
	src := "service Foo {\n  contexts: Auth, Profile\n  language: golang\n}"
	gn, li, _ := syntax.Parse(src)
	root := syntax.Root(gn)
	file := syntax.AsFile(root)
	svcs := file.Services()
	if len(svcs) != 1 {
		t.Fatalf("want 1 service, got %d", len(svcs))
	}
	refs := svcs[0].ContextRefs()
	if len(refs) != 2 {
		t.Fatalf("want 2 context refs, got %d", len(refs))
	}
	want := []string{"Auth", "Profile"}
	for i, ref := range refs {
		tok := ref.Name()
		if tok == nil {
			t.Errorf("refs[%d].Name() = nil", i)
			continue
		}
		if tok.Text() != want[i] {
			t.Errorf("refs[%d].Name() = %q, want %q", i, tok.Text(), want[i])
		}
		line, _ := li.LineCol(tok.Offset())
		if line != 2 {
			t.Errorf("refs[%d] line = %d, want 2", i, line)
		}
	}
}

func TestContextTokens_StillWorksAfterRefWrap(t *testing.T) {
	// Verify ContextTokens() (which uses Tokens() recursively) is unaffected
	// by the SyntaxKindRef wrapper added in this task.
	src := "service Foo {\n  contexts: Auth, Profile\n}"
	gn, _, _ := syntax.Parse(src)
	root := syntax.Root(gn)
	file := syntax.AsFile(root)
	toks := file.Services()[0].ContextTokens()
	if len(toks) != 2 {
		t.Fatalf("want 2 tokens, got %d", len(toks))
	}
	for i, want := range []string{"Auth", "Profile"} {
		if toks[i].Text() != want {
			t.Errorf("toks[%d] = %q, want %q", i, toks[i].Text(), want)
		}
	}
}

func TestExposureTokenAccessors(t *testing.T) {
	src := "actor user Alice\nactor user Bob\nservice PaySvc {\n  language: golang\n}\nexposure default {\n  to: Alice, Bob\n  through: PaySvc\n  contexts: Auth\n}"
	gn, li, _ := syntax.Parse(src)
	root := syntax.Root(gn)
	file := syntax.AsFile(root)
	exps := file.Exposures()
	if len(exps) != 1 {
		t.Fatalf("want 1 exposure, got %d", len(exps))
	}
	exp := exps[0]

	toToks := exp.ToTokens()
	if len(toToks) != 2 {
		t.Fatalf("ToTokens: want 2, got %d", len(toToks))
	}
	for i, want := range []string{"Alice", "Bob"} {
		if toToks[i].Text() != want {
			t.Errorf("ToTokens[%d] = %q, want %q", i, toToks[i].Text(), want)
		}
		line, _ := li.LineCol(toToks[i].Offset())
		if line != 7 {
			t.Errorf("ToTokens[%d] line = %d, want 7", i, line)
		}
	}

	throughToks := exp.ThroughTokens()
	if len(throughToks) != 1 || throughToks[0].Text() != "PaySvc" {
		t.Errorf("ThroughTokens = %v, want [PaySvc]", throughToks)
	}

	ctxToks := exp.ContextsTokens()
	if len(ctxToks) != 1 || ctxToks[0].Text() != "Auth" {
		t.Errorf("ContextsTokens = %v, want [Auth]", ctxToks)
	}
}

func TestResyncToTopLevel_EmitsErrorNode(t *testing.T) {
	// "service Foo extra" without "{" triggers resyncToTopLevel.
	// The tokens up to the next top-level keyword should be wrapped in
	// SyntaxKindErrorNode, not emitted as loose siblings of SyntaxKindServiceDecl.
	// We include extra non-top-level tokens ("extra") so resyncToTopLevel has
	// something to consume and wrap.
	src := "service Foo extra\nservice Bar {\n  language: golang\n}"
	gn, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Fatal("want at least one diagnostic for missing `{`")
	}

	root := syntax.Root(gn)
	file := syntax.AsFile(root)

	// Bar should be parsed successfully despite Foo being broken.
	svcs := file.Services()
	var barFound bool
	for _, svc := range svcs {
		tok := svc.Name()
		if tok != nil && tok.Text() == "Bar" {
			barFound = true
		}
	}
	if !barFound {
		t.Error("service Bar not found — resyncToTopLevel consumed it")
	}

	// Walk all tokens looking for any whose parent has kind SyntaxKindErrorNode.
	found := false
	for _, tok := range root.AllTokens() {
		if tok.Parent().Kind() == syntax.SyntaxKindErrorNode {
			found = true
			break
		}
	}
	if !found {
		t.Error("no SyntaxKindErrorNode found in tree after resyncToTopLevel")
	}
}
