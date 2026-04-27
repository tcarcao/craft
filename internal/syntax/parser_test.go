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
	if a.TargetContext != "User" {
		t.Errorf("targetDomain: got %q want User", a.TargetContext)
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
	if a.TargetContext != "" {
		t.Errorf("targetDomain: got %q want empty", a.TargetContext)
	}
	if a.Phrase != "confirmation status" {
		t.Errorf("phrase: got %q want %q", a.Phrase, "confirmation status")
	}
}

func TestParse_NumberInActionPhrase(t *testing.T) {
	src := `use_case "Test" {
    when Customer initiates transfer
        PaymentProcessing asks TransactionValidation to check 3 transfer limits
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	a := f.UseCases[0].Scenarios[0].Actions[0]
	if a.ActionType != "sync_action" {
		t.Errorf("type: got %q want sync_action", a.ActionType)
	}
	if a.Phrase != "check 3 transfer limits" {
		t.Errorf("phrase: got %q want %q", a.Phrase, "check 3 transfer limits")
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

func TestParse_ServiceEndLine(t *testing.T) {
	src := "services {\n  MyService {\n    contexts: Foo\n  }\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	if f.Services[0].EndLine != 4 {
		t.Errorf("expected EndLine=4, got %d", f.Services[0].EndLine)
	}
}

func TestParse_DomainEndLine(t *testing.T) {
	src := "domain Commerce {\n  Orders\n  Payments\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	if f.Domains[0].EndLine != 4 {
		t.Errorf("expected EndLine=4, got %d", f.Domains[0].EndLine)
	}
}

func TestParse_UseCaseEndLine(t *testing.T) {
	src := "actor user Foo\nuse_case \"DoThing\" {\n  when Foo does something\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if len(f.UseCases) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(f.UseCases))
	}
	if f.UseCases[0].EndLine != 4 {
		t.Errorf("expected EndLine=4, got %d", f.UseCases[0].EndLine)
	}
}

func TestParse_ActorBlockRange(t *testing.T) {
	src := "actors {\n  user Alice\n  system Bob\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if len(f.ActorBlocks) != 1 {
		t.Fatalf("expected 1 actor block, got %d", len(f.ActorBlocks))
	}
	if f.ActorBlocks[0].Line != 1 {
		t.Errorf("expected block Line=1, got %d", f.ActorBlocks[0].Line)
	}
	if f.ActorBlocks[0].EndLine != 4 {
		t.Errorf("expected block EndLine=4, got %d", f.ActorBlocks[0].EndLine)
	}
}

func TestParse_ServiceIsGrouped_InsideBlock(t *testing.T) {
	src := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	if !f.Services[0].IsGrouped {
		t.Error("expected IsGrouped=true for service inside services { } block")
	}
}

func TestParse_ServiceIsGrouped_TopLevel(t *testing.T) {
	src := "service OrderSvc {\n  contexts: Orders\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	if f.Services[0].IsGrouped {
		t.Error("expected IsGrouped=false for top-level service declaration")
	}
}

func TestParse_DomainIsGrouped_InsideBlock(t *testing.T) {
	src := "domains {\n  Commerce {\n    Orders\n  }\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	if !f.Domains[0].IsGrouped {
		t.Error("expected IsGrouped=true for domain inside domains { } block")
	}
}

func TestParse_DomainIsGrouped_TopLevel(t *testing.T) {
	src := "domain Commerce {\n  Orders\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	if f.Domains[0].IsGrouped {
		t.Error("expected IsGrouped=false for top-level domain declaration")
	}
}

// --- position tracking tests ---
// These verify that Line and Column are set correctly on AST nodes so that
// LSP semantic tokens point to the exact character where each name starts.

func TestParse_ActorColumnTracking(t *testing.T) {
	// Line 2: "    user Alice" — Alice starts at column 10 (1-based)
	src := "actors {\n    user Alice\n    system Bob\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 2 {
		t.Fatalf("expected 2 actors, got %d", len(f.Actors))
	}
	alice := f.Actors[0]
	if alice.Line != 2 {
		t.Errorf("Alice: want Line=2, got %d", alice.Line)
	}
	if alice.Column != 10 {
		t.Errorf("Alice: want Column=10, got %d", alice.Column)
	}
	bob := f.Actors[1]
	if bob.Line != 3 {
		t.Errorf("Bob: want Line=3, got %d", bob.Line)
	}
	if bob.Column != 12 {
		t.Errorf("Bob: want Column=12, got %d", bob.Column)
	}
}

func TestParse_DomainNameColumnTracking(t *testing.T) {
	// "domain ECommerce {" — ECommerce starts at column 8 (after "domain ")
	src := "domain ECommerce {\n    Auth\n    Profile\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	d := f.Domains[0]
	if d.Line != 1 {
		t.Errorf("domain name: want Line=1, got %d", d.Line)
	}
	if d.Column != 8 {
		t.Errorf("domain name: want Column=8, got %d", d.Column)
	}
}

func TestParse_BoundedContextPositionTracking(t *testing.T) {
	// Line 2: "    Auth"   — Auth starts at column 5
	// Line 3: "    Profile" — Profile starts at column 5
	src := "domain ECommerce {\n    Auth\n    Profile\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	d := f.Domains[0]
	if len(d.BoundedContexts) != 2 {
		t.Fatalf("expected 2 bounded contexts, got %d", len(d.BoundedContexts))
	}
	auth := d.BoundedContexts[0]
	if auth.Name != "Auth" {
		t.Errorf("want Name=Auth, got %q", auth.Name)
	}
	if auth.Line != 2 {
		t.Errorf("Auth: want Line=2, got %d", auth.Line)
	}
	if auth.Column != 5 {
		t.Errorf("Auth: want Column=5, got %d", auth.Column)
	}
	profile := d.BoundedContexts[1]
	if profile.Name != "Profile" {
		t.Errorf("want Name=Profile, got %q", profile.Name)
	}
	if profile.Line != 3 {
		t.Errorf("Profile: want Line=3, got %d", profile.Line)
	}
	if profile.Column != 5 {
		t.Errorf("Profile: want Column=5, got %d", profile.Column)
	}
}

func TestParse_ServiceColumnTracking(t *testing.T) {
	// "service MyService {" — MyService starts at column 9 (after "service ")
	src := "service MyService {\n    contexts: Auth\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	svc := f.Services[0]
	if svc.Line != 1 {
		t.Errorf("service: want Line=1, got %d", svc.Line)
	}
	if svc.Column != 9 {
		t.Errorf("service: want Column=9, got %d", svc.Column)
	}
}

func TestParse_GroupedServiceColumnTracking(t *testing.T) {
	// Inside services block: "  OrderSvc {" — OrderSvc starts at column 3
	src := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(f.Services))
	}
	svc := f.Services[0]
	if svc.Line != 2 {
		t.Errorf("service: want Line=2, got %d", svc.Line)
	}
	if svc.Column != 3 {
		t.Errorf("service: want Column=3, got %d", svc.Column)
	}
}
