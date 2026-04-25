package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/syntax"
)

func TestParse_IndividualActor(t *testing.T) {
	f, diags := syntax.Parse("actor user Customer_Support")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(f.Actors))
	}
	a := f.Actors[0]
	if a.Name != "Customer_Support" || a.Type != ast.ActorTypeUser {
		t.Errorf("got %+v", a)
	}
}

func TestParse_ActorsBlock(t *testing.T) {
	src := `actors {
    user Alice
    system Bob
    service DB
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(f.Actors))
	}
}

func TestParse_MixedActors(t *testing.T) {
	src := `actors {
    user Alice
    system Bob
}

actor service DB`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(f.Actors))
	}
}

func TestParse_UseCaseParsedWithActor(t *testing.T) {
	// S6: use_case is now supported — should parse cleanly alongside an actor.
	src := `actor user Foo
use_case "DoSomething" {
    when User does something
}`
	f, diags := syntax.Parse(src)
	if len(f.Actors) != 1 {
		t.Errorf("expected 1 actor, got %d", len(f.Actors))
	}
	if len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
	if len(f.UseCases) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(f.UseCases))
	}
	uc := f.UseCases[0]
	if uc.Name != "DoSomething" {
		t.Errorf("expected use case name 'DoSomething', got %q", uc.Name)
	}
	if len(uc.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(uc.Scenarios))
	}
}

func TestParse_IndividualDomain(t *testing.T) {
	src := `domain ECommerce {
    User
    Product
    Order
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	d := f.Domains[0]
	if d.Name != "ECommerce" {
		t.Errorf("expected name ECommerce, got %q", d.Name)
	}
	if len(d.BoundedContexts) != 3 {
		t.Errorf("expected 3 bounded contexts, got %d", len(d.BoundedContexts))
	}
}

func TestParse_DomainsBlock(t *testing.T) {
	src := `domains {
    Auth {
        Login
        Logout
    }
    Billing {
        Invoice
    }
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(f.Domains))
	}
	if f.Domains[0].Name != "Auth" || f.Domains[1].Name != "Billing" {
		t.Errorf("unexpected domain names: %v, %v", f.Domains[0].Name, f.Domains[1].Name)
	}
}

func TestParse_ActorsAndDomains(t *testing.T) {
	src := `actor user Customer
domain User {
    Authentication
    Profile
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 1 || len(f.Domains) != 1 {
		t.Errorf("expected 1 actor and 1 domain, got %d actors, %d domains", len(f.Actors), len(f.Domains))
	}
}

func TestParse_ExposureSimple(t *testing.T) {
	src := `actor user Business_User

exposure default {
    to: Business_User
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if exp.Name != "default" {
		t.Errorf("exposure name: got %q, want %q", exp.Name, "default")
	}
	if len(exp.To) != 1 || exp.To[0] != "Business_User" {
		t.Errorf("exposure.to: got %v", exp.To)
	}
}

func TestParse_ExposureWithThrough(t *testing.T) {
	src := `exposure api {
    to: Business_User, Customer_Support
    through: APIGateway
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if exp.Name != "api" {
		t.Errorf("exposure name: got %q, want %q", exp.Name, "api")
	}
	if len(exp.To) != 2 || exp.To[0] != "Business_User" || exp.To[1] != "Customer_Support" {
		t.Errorf("exposure.to: got %v", exp.To)
	}
	if len(exp.Through) != 1 || exp.Through[0] != "APIGateway" {
		t.Errorf("exposure.through: got %v", exp.Through)
	}
}

func TestParse_ExposureWithContexts(t *testing.T) {
	src := `exposure web {
    to: Business_User
    contexts: Authentication, Profile
    through: LoadBalancer
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if len(exp.Contexts) != 2 {
		t.Errorf("exposure.contexts: got %v", exp.Contexts)
	}
}

func TestParse_MultipleExposures(t *testing.T) {
	src := `exposure PublicAPI {
    to: Business_User
    through: APIGateway
}

exposure InternalAPI {
    to: InternalSystem
    through: InternalGateway
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 2 {
		t.Fatalf("expected 2 exposures, got %d", len(f.Exposures))
	}
	if f.Exposures[0].Name != "PublicAPI" || f.Exposures[1].Name != "InternalAPI" {
		t.Errorf("wrong exposure names: %v", []string{f.Exposures[0].Name, f.Exposures[1].Name})
	}
}

func TestParse_OpenActorType(t *testing.T) {
	tests := []struct {
		src      string
		wantName string
		wantType string
	}{
		{"actor bot SlackBot", "SlackBot", "bot"},
		{"actor integration ExternalCRM", "ExternalCRM", "integration"},
	}
	for _, tc := range tests {
		t.Run(tc.wantType, func(t *testing.T) {
			f, diags := syntax.Parse(tc.src)
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if len(f.Actors) != 1 {
				t.Fatalf("expected 1 actor, got %d", len(f.Actors))
			}
			a := f.Actors[0]
			if a.Name != tc.wantName || string(a.Type) != tc.wantType {
				t.Errorf("got name=%q type=%q", a.Name, a.Type)
			}
		})
	}
}

func TestParse_OpenActorTypeInBlock(t *testing.T) {
	src := `actors {
    bot SlackBot
    integration ExternalCRM
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 2 {
		t.Fatalf("expected 2 actors, got %d", len(f.Actors))
	}
}

func TestParse_DeploymentRules(t *testing.T) {
	src := `services {
    CommsService {
        contexts: Notifier
        deployment: canary(90% -> stable, 10% -> experimental)
    }
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service")
	}
	svc := f.Services[0]
	if svc.DeploymentType != "canary" {
		t.Errorf("type: got %q want canary", svc.DeploymentType)
	}
	if len(svc.DeploymentRules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(svc.DeploymentRules))
	}
	if svc.DeploymentRules[0].Percentage != "90%" || svc.DeploymentRules[0].Target != "stable" {
		t.Errorf("rule[0]: %+v", svc.DeploymentRules[0])
	}
	if svc.DeploymentRules[1].Percentage != "10%" || svc.DeploymentRules[1].Target != "experimental" {
		t.Errorf("rule[1]: %+v", svc.DeploymentRules[1])
	}
}

func TestParse_DeploymentTypeOnly(t *testing.T) {
	src := `services { UserService { deployment: blue_green } }`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if f.Services[0].DeploymentType != "blue_green" {
		t.Errorf("type: got %q", f.Services[0].DeploymentType)
	}
	if len(f.Services[0].DeploymentRules) != 0 {
		t.Errorf("expected no rules, got %d", len(f.Services[0].DeploymentRules))
	}
}

func TestParse_SingleServiceForm(t *testing.T) {
	src := `service UserService {
    contexts: Auth, Profile
    language: golang
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	svc := f.Services[0]
	if svc.Name != "UserService" {
		t.Errorf("name: got %q", svc.Name)
	}
	if len(svc.Contexts) != 2 {
		t.Errorf("contexts: got %d want 2", len(svc.Contexts))
	}
	if svc.Language != "golang" {
		t.Errorf("language: got %q", svc.Language)
	}
}

// Q2: returns to <target> <phrase>
func TestParse_ReturnWithTarget(t *testing.T) {
	src := `use_case "Test" {
    when User checks balance
        AccountManagement returns to User confirmation status
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	a := f.UseCases[0].Scenarios[0].Actions[0]
	if a.TargetDomain != "User" {
		t.Errorf("targetDomain: got %q want User", a.TargetDomain)
	}
	if a.Phrase != "confirmation status" {
		t.Errorf("phrase: got %q want %q", a.Phrase, "confirmation status")
	}
}

// Q2: returns without target
func TestParse_ReturnWithoutTarget(t *testing.T) {
	src := `use_case "Test" {
    when User checks balance
        AccountManagement returns confirmation status
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	a := f.UseCases[0].Scenarios[0].Actions[0]
	if a.TargetDomain != "" {
		t.Errorf("targetDomain: got %q want empty", a.TargetDomain)
	}
	if a.Phrase != "confirmation status" {
		t.Errorf("phrase: got %q want %q", a.Phrase, "confirmation status")
	}
}

// Q3: single-line services block parses identically to multi-line
func TestParse_SingleLineServicesEquivalent(t *testing.T) {
	multi := `services {
    A { contexts: X }
    B { contexts: Y }
}`
	single := `services { A { contexts: X } B { contexts: Y } }`
	f1, d1 := syntax.Parse(multi)
	f2, d2 := syntax.Parse(single)
	if len(d1) != 0 || len(d2) != 0 {
		t.Fatalf("diagnostics: multi=%v single=%v", d1, d2)
	}
	if len(f1.Services) != len(f2.Services) {
		t.Errorf("service count: multi=%d single=%d", len(f1.Services), len(f2.Services))
	}
}

// Q8: block comments are ignored (parser-level)
func TestParse_BlockComment(t *testing.T) {
	src := `/* opening comment */
services {
    /* inline comment */ UserService {
        contexts: Auth
    }
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 || f.Services[0].Name != "UserService" {
		t.Fatalf("unexpected services: %+v", f.Services)
	}
}

// Q10: use_case name with escaped quotes
func TestParse_UseCaseEscapedName(t *testing.T) {
	src := "use_case \"He said \\\"hello\\\"\" {\n    when User greets\n        System confirms greeting\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if f.UseCases[0].Name != `He said "hello"` {
		t.Errorf("name: got %q", f.UseCases[0].Name)
	}
}

// Q12: subject-less event trigger
func TestParse_SubjectlessTrigger(t *testing.T) {
	src := `use_case "Cron" {
    when "DailyCron"
        PaymentProcessing processes payments
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	trigger := f.UseCases[0].Scenarios[0].Trigger
	if trigger.TriggerType != "event" {
		t.Errorf("type: got %q want event", trigger.TriggerType)
	}
	if trigger.Event != "DailyCron" {
		t.Errorf("event: got %q want DailyCron", trigger.Event)
	}
}
