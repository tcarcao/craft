package parser

import (	"testing"
)

func TestParser_BasicExposureDefinition(t *testing.T) {
	dsl := `exposure PublicAPI {
		to: external_clients, mobile_apps
		contexts: UserService, OrderService
		through: APIGateway, LoadBalancer
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Exposures) != 1 {
		t.Errorf("Expected 1 exposure, got %d", len(model.Exposures))
	}

	exposure := model.Exposures[0]
	if exposure.Name != "PublicAPI" {
		t.Errorf("Expected exposure name 'PublicAPI', got '%s'", exposure.Name)
	}

	// Validate 'to' targets
	expectedTargets := []string{"external_clients", "mobile_apps"}
	if len(exposure.To) != len(expectedTargets) {
		t.Errorf("Expected %d targets, got %d", len(expectedTargets), len(exposure.To))
	}

	for i, target := range exposure.To {
		if target != expectedTargets[i] {
			t.Errorf("Expected target '%s', got '%s'", expectedTargets[i], target)
		}
	}

	// Validate 'of' domains
	expectedDomains := []string{"UserService", "OrderService"}
	if len(exposure.Contexts) != len(expectedDomains) {
		t.Errorf("Expected %d domains, got %d", len(expectedDomains), len(exposure.Contexts))
	}

	for i, domain := range exposure.Contexts {
		if domain != expectedDomains[i] {
			t.Errorf("Expected domain '%s', got '%s'", expectedDomains[i], domain)
		}
	}

	// Validate 'through' gateways
	expectedGateways := []string{"APIGateway", "LoadBalancer"}
	if len(exposure.Through) != len(expectedGateways) {
		t.Errorf("Expected %d gateways, got %d", len(expectedGateways), len(exposure.Through))
	}

	for i, gateway := range exposure.Through {
		if gateway != expectedGateways[i] {
			t.Errorf("Expected gateway '%s', got '%s'", expectedGateways[i], gateway)
		}
	}
}

func TestParser_PartialExposureDefinition(t *testing.T) {
	dsl := `exposure InternalAPI {
		to: internal_services
		contexts: PaymentService
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	exposure := model.Exposures[0]
	if exposure.Name != "InternalAPI" {
		t.Errorf("Expected exposure name 'InternalAPI', got '%s'", exposure.Name)
	}

	if len(exposure.To) != 1 || exposure.To[0] != "internal_services" {
		t.Errorf("Expected 'to' target 'internal_services', got %v", exposure.To)
	}

	if len(exposure.Contexts) != 1 || exposure.Contexts[0] != "PaymentService" {
		t.Errorf("Expected 'of' domain 'PaymentService', got %v", exposure.Contexts)
	}

	if len(exposure.Through) != 0 {
		t.Errorf("Expected no 'through' gateways, got %v", exposure.Through)
	}
}

func TestParser_MultipleExposures(t *testing.T) {
	dsl := `exposure PublicAPI {
		to: external_clients
		contexts: UserService
		through: APIGateway
	}

	exposure InternalAPI {
		to: internal_services
		contexts: PaymentService, OrderService
	}

	exposure PartnerAPI {
		to: trusted_partners
		contexts: DataService
		through: PartnerGateway
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Exposures) != 3 {
		t.Errorf("Expected 3 exposures, got %d", len(model.Exposures))
	}

	expectedNames := []string{"PublicAPI", "InternalAPI", "PartnerAPI"}
	for i, exposure := range model.Exposures {
		if exposure.Name != expectedNames[i] {
			t.Errorf("Expected exposure name '%s', got '%s'", expectedNames[i], exposure.Name)
		}
	}

	// Validate specific exposure properties
	internalAPI := model.Exposures[1]
	if len(internalAPI.Contexts) != 2 {
		t.Errorf("Expected 2 domains for InternalAPI, got %d", len(internalAPI.Contexts))
	}

	if len(internalAPI.Through) != 0 {
		t.Errorf("Expected no gateways for InternalAPI, got %d", len(internalAPI.Through))
	}
}

func TestParser_MultiLineExposureContextList(t *testing.T) {
	dsl := `exposure AccountAPI {
	to: Customer, Business_User
	contexts: AccountManagement, AccountManagementProcess,
		MemberInvitation, MemberRecommendation,
		AdAssignment, AdContact
	through: APIGateway
}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)
	if err != nil {
		t.Fatalf("Expected no error for multi-line exposure contexts list, got: %v", err)
	}

	if len(model.Exposures) != 1 {
		t.Fatalf("Expected 1 exposure, got %d", len(model.Exposures))
	}

	expected := []string{
		"AccountManagement", "AccountManagementProcess",
		"MemberInvitation", "MemberRecommendation",
		"AdAssignment", "AdContact",
	}
	exp := model.Exposures[0]
	if len(exp.Contexts) != len(expected) {
		t.Fatalf("Expected %d contexts, got %d: %v", len(expected), len(exp.Contexts), exp.Contexts)
	}
	for i, ctx := range exp.Contexts {
		if ctx != expected[i] {
			t.Errorf("contexts[%d]: expected %q, got %q", i, expected[i], ctx)
		}
	}
}

func TestParser_MultiLineExposureToList(t *testing.T) {
	dsl := `exposure PartnerAPI {
	to: PartnerA, PartnerB,
		PartnerC, PartnerD
	contexts: SharedService
	through: PartnerGateway
}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)
	if err != nil {
		t.Fatalf("Expected no error for multi-line exposure to list, got: %v", err)
	}

	expected := []string{"PartnerA", "PartnerB", "PartnerC", "PartnerD"}
	exp := model.Exposures[0]
	if len(exp.To) != len(expected) {
		t.Fatalf("Expected %d targets, got %d: %v", len(expected), len(exp.To), exp.To)
	}
	for i, target := range exp.To {
		if target != expected[i] {
			t.Errorf("to[%d]: expected %q, got %q", i, expected[i], target)
		}
	}
}

func BenchmarkParser_ExposureDefinition(b *testing.B) {
	dsl := `exposure BenchmarkAPI {
		to: external_clients, mobile_apps, third_party_services
		contexts: UserService, OrderService, PaymentService, NotificationService
		through: APIGateway, LoadBalancer, CDN, AuthProxy
	}`

	parser := NewParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseString(dsl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

