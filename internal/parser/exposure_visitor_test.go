package parser

import (	"testing"
)

func TestParser_BasicExposureDefinition(t *testing.T) {
	dsl := `exposure PublicAPI {
		to: external_clients, mobile_apps
		of: UserService, OrderService
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
	if len(exposure.Of) != len(expectedDomains) {
		t.Errorf("Expected %d domains, got %d", len(expectedDomains), len(exposure.Of))
	}

	for i, domain := range exposure.Of {
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
		of: PaymentService
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

	if len(exposure.Of) != 1 || exposure.Of[0] != "PaymentService" {
		t.Errorf("Expected 'of' domain 'PaymentService', got %v", exposure.Of)
	}

	if len(exposure.Through) != 0 {
		t.Errorf("Expected no 'through' gateways, got %v", exposure.Through)
	}
}

func TestParser_MultipleExposures(t *testing.T) {
	dsl := `exposure PublicAPI {
		to: external_clients
		of: UserService
		through: APIGateway
	}

	exposure InternalAPI {
		to: internal_services
		of: PaymentService, OrderService
	}

	exposure PartnerAPI {
		to: trusted_partners
		of: DataService
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
	if len(internalAPI.Of) != 2 {
		t.Errorf("Expected 2 domains for InternalAPI, got %d", len(internalAPI.Of))
	}

	if len(internalAPI.Through) != 0 {
		t.Errorf("Expected no gateways for InternalAPI, got %d", len(internalAPI.Through))
	}
}

func BenchmarkParser_ExposureDefinition(b *testing.B) {
	dsl := `exposure BenchmarkAPI {
		to: external_clients, mobile_apps, third_party_services
		of: UserService, OrderService, PaymentService, NotificationService
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

