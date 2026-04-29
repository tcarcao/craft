package syntax_test

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestActorDecl_View(t *testing.T) {
	tree, _ := syntax.Parse("actor user Alice")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	a := actors[0]
	if a.Name() == nil || a.Name().Value != "Alice" {
		t.Errorf("expected name Alice, got %v", a.Name())
	}
	if a.ActorType() == nil || a.ActorType().Kind != syntax.SyntaxKindKwUser {
		t.Errorf("expected user type, got %v", a.ActorType())
	}
	if a.Keyword() == nil || a.Keyword().Line != 1 {
		t.Errorf("expected keyword on line 1, got %v", a.Keyword())
	}
}

func TestActorsBlock_View(t *testing.T) {
	tree, _ := syntax.Parse("actors { user Alice  system Bob }")
	file := syntax.AsFile(tree)
	actors := file.Actors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 actors from block, got %d", len(actors))
	}
}

func TestDomainDecl_View(t *testing.T) {
	tree, _ := syntax.Parse("domain Ordering { Cart Checkout }")
	file := syntax.AsFile(tree)
	domains := file.Domains()
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}
	d := domains[0]
	if d.Name() == nil || d.Name().Value != "Ordering" {
		t.Errorf("expected name Ordering, got %v", d.Name())
	}
	bcs := d.BoundedContexts()
	if len(bcs) != 2 {
		t.Errorf("expected 2 bounded contexts, got %d", len(bcs))
	}
	if bcs[0].Name() == nil || bcs[0].Name().Value != "Cart" {
		t.Errorf("expected first BC Cart, got %v", bcs[0].Name())
	}
}

func TestFile_Actors_DocumentOrder(t *testing.T) {
	src := "actor user Alice\nactors { system Bob }\nactor user Carol"
	tree, _ := syntax.Parse(src)
	actors := syntax.AsFile(tree).Actors()
	if len(actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(actors))
	}
	names := []string{actors[0].Name().Value, actors[1].Name().Value, actors[2].Name().Value}
	expected := []string{"Alice", "Bob", "Carol"}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("actor[%d]: expected %q, got %q", i, expected[i], n)
		}
	}
}

func TestFile_SyntaxTree(t *testing.T) {
	tree, _ := syntax.Parse("actor user Alice")
	file := syntax.AsFile(tree)
	// AsFile must not panic on nil
	nilFile := syntax.AsFile(nil)
	if len(nilFile.Actors()) != 0 {
		t.Error("nil tree should return empty actors")
	}
	_ = file
}

func TestServiceDecl_View(t *testing.T) {
	tree, _ := syntax.Parse("service order-service { contexts: [Cart] }")
	file := syntax.AsFile(tree)
	services := file.Services()
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	s := services[0]
	if s.Name() == nil || s.Name().Value != "order-service" {
		t.Errorf("expected name order-service, got %v", s.Name())
	}
	if s.Keyword() == nil {
		t.Error("expected service keyword")
	}
}

func TestUseCaseDecl_View(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree, _ := syntax.Parse(src)
	file := syntax.AsFile(tree)
	ucs := file.UseCases()
	if len(ucs) != 1 {
		t.Fatalf("expected 1 use_case, got %d", len(ucs))
	}
	uc := ucs[0]
	if uc.Title() == nil || uc.Title().Value != "Pay" {
		t.Errorf("expected title Pay, got %v", uc.Title())
	}
	scenarios := uc.Scenarios()
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
}

func TestScenarioDecl_View(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree, _ := syntax.Parse(src)
	uc := syntax.AsFile(tree).UseCases()[0]
	scenario := uc.Scenarios()[0]

	when := scenario.When()
	if when == nil || when.Kind != syntax.SyntaxKindKwWhen {
		t.Errorf("expected when token, got %v", when)
	}
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
}

func TestActionDecl_VerbPosition(t *testing.T) {
	src := "use_case \"Pay\" {\n    when Customer submits PaymentForm\n    PaymentService asks Bank to process\n}"
	tree, _ := syntax.Parse(src)
	scenario := syntax.AsFile(tree).UseCases()[0].Scenarios()[0]
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action")
	}
	verb := actions[0].Verb()
	if verb == nil {
		t.Fatal("expected verb token")
	}
	if verb.Kind != syntax.SyntaxKindKwAsks {
		t.Errorf("expected SyntaxKindKwAsks, got %v", verb.Kind)
	}
	if verb.Line <= 0 || verb.Col <= 0 {
		t.Errorf("verb position not set: line=%d col=%d", verb.Line, verb.Col)
	}
}

func TestArchDecl_View(t *testing.T) {
	src := "arch MyArch {\n    presentation: ComponentA\n    gateway: ComponentB\n}"
	tree, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	archs := syntax.AsFile(tree).Archs()
	if len(archs) != 1 {
		t.Fatalf("expected 1 arch, got %d", len(archs))
	}
	arch := archs[0]
	if arch.Name() == nil || arch.Name().Value != "MyArch" {
		t.Errorf("expected arch name MyArch, got %v", arch.Name())
	}
	sections := arch.Sections()
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	// First section should be presentation
	kw := sections[0].Keyword()
	if kw == nil || kw.Kind != syntax.SyntaxKindKwPresentation {
		t.Errorf("expected presentation keyword, got %v", kw)
	}
	components := sections[0].Components()
	if len(components) != 1 {
		t.Errorf("expected 1 component, got %d", len(components))
	}
	if components[0].Name() == nil || components[0].Name().Value != "ComponentA" {
		t.Errorf("expected ComponentA, got %v", components[0].Name())
	}
}

func TestExposureDecl_View(t *testing.T) {
	src := "exposure MyExposure {\n    to: Customer\n    through: rest\n}"
	tree, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	exposures := syntax.AsFile(tree).Exposures()
	if len(exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(exposures))
	}
	e := exposures[0]
	if e.Name() == nil || e.Name().Value != "MyExposure" {
		t.Errorf("expected name MyExposure, got %v", e.Name())
	}
	rules := e.Rules()
	if len(rules) < 1 {
		t.Fatalf("expected at least 1 rule, got %d", len(rules))
	}
	through := rules[0].Through()
	if through == nil || through.Kind != syntax.SyntaxKindKwThrough {
		t.Errorf("expected through keyword, got %v", through)
	}
}

func TestActionDecl_Notifies(t *testing.T) {
	src := "use_case \"Notify\" {\n    when Customer submits NotifyForm\n    EmailService notifies \"UserNotified\"\n}"
	tree, _ := syntax.Parse(src)
	scenario := syntax.AsFile(tree).UseCases()[0].Scenarios()[0]
	actions := scenario.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action")
	}
	verb := actions[0].Verb()
	if verb == nil || verb.Kind != syntax.SyntaxKindKwNotifies {
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
	tree, _ := syntax.Parse(src)
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
	tree, _ := syntax.Parse(src)
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
	tree, _ := syntax.Parse(src)
	f := syntax.AsFile(tree)
	doms := f.Domains()
	if len(doms) != 1 {
		t.Fatalf("want 1 domain, got %d", len(doms))
	}
	d := doms[0]
	if d.IsGrouped() {
		t.Error("want IsGrouped=false for standalone domain")
	}
	if got := d.EndLine(); got == 0 {
		t.Error("want EndLine > 0")
	}
}

func TestArchDecl_LineAccessors(t *testing.T) {
	src := `arch MyArch {
  presentation:
    WebApp
  gateway:
    APIGateway
}`
	tree, _ := syntax.Parse(src)
	f := syntax.AsFile(tree)
	archs := f.Archs()
	if len(archs) != 1 {
		t.Fatalf("want 1 arch, got %d", len(archs))
	}
	a := archs[0]
	if a.Line() == 0 {
		t.Error("Line should be non-zero")
	}
	if a.EndLine() == 0 {
		t.Error("EndLine should be non-zero")
	}
	if a.PresentationLine() == 0 {
		t.Error("PresentationLine should be non-zero")
	}
	if a.GatewayLine() == 0 {
		t.Error("GatewayLine should be non-zero")
	}
}
