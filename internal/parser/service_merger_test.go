package parser

import (
	"testing"
)

func TestServiceMerging(t *testing.T) {
	// Test DSL with multiple services blocks containing same service name
	dslContent := `
services {
  UserService {
    domains: User, Authentication
    language: go
  }
}

services {
  UserService {
    domains: Profile, Settings
    data-stores: user_db, cache
  }
  PaymentService {
    domains: Payment
    language: java
  }
}
`

	// Parse the DSL
	parser := NewParser()
	model, err := parser.ParseString(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	// Debug output
	t.Logf("Parsed model: Architectures=%d, Exposures=%d, Services=%d, UseCases=%d, Domains=%d",
		len(model.Architectures), len(model.Exposures), len(model.Services), len(model.UseCases), len(model.Domains))

	for i, service := range model.Services {
		t.Logf("Service %d: Name='%s', Domains=%v, Language='%s'", i, service.Name, service.Domains, service.Language)
	}

	// Verify we have 2 services (UserService merged + PaymentService)
	if len(model.Services) != 2 {
		t.Errorf("Expected 2 services after merging, got %d", len(model.Services))
	}

	// Find the merged UserService
	var userService *Service
	var paymentService *Service
	for i := range model.Services {
		switch model.Services[i].Name {
		case "UserService":
			userService = &model.Services[i]
		case "PaymentService":
			paymentService = &model.Services[i]
		}
	}

	if userService == nil {
		t.Fatal("UserService not found in merged services")
	}

	if paymentService == nil {
		t.Fatal("PaymentService not found in merged services")
	}

	// Verify UserService has merged domains
	expectedDomains := []string{"User", "Authentication", "Profile", "Settings"}
	if len(userService.Domains) != len(expectedDomains) {
		t.Errorf("Expected %d domains for UserService, got %d", len(expectedDomains), len(userService.Domains))
	}

	// Check that all expected domains are present (order doesn't matter)
	domainMap := make(map[string]bool)
	for _, domain := range userService.Domains {
		domainMap[domain] = true
	}
	for _, expected := range expectedDomains {
		if !domainMap[expected] {
			t.Errorf("Expected domain %s not found in merged UserService", expected)
		}
	}

	// Verify UserService has merged data stores
	expectedDataStores := []string{"user_db", "cache"}
	if len(userService.DataStores) != len(expectedDataStores) {
		t.Errorf("Expected %d data stores for UserService, got %d", len(expectedDataStores), len(userService.DataStores))
	}

	// Verify language is preserved (should be "Go" from first definition)
	if userService.Language != "Go" {
		t.Errorf("Expected language 'Go' for UserService, got '%s'", userService.Language)
	}

	// Verify deployment strategy is preserved and rules are merged
	if userService.Deployment.Type != "canary" {
		t.Errorf("Expected deployment type 'canary' for UserService, got '%s'", userService.Deployment.Type)
	}

	if len(userService.Deployment.Rules) != 2 {
		t.Errorf("Expected 2 deployment rules for UserService, got %d", len(userService.Deployment.Rules))
	}

	// Verify PaymentService remains unchanged
	if paymentService.Language != "Java" {
		t.Errorf("Expected language 'Java' for PaymentService, got '%s'", paymentService.Language)
	}

	if len(paymentService.Domains) != 1 || paymentService.Domains[0] != "Payment" {
		t.Errorf("PaymentService domains were incorrectly modified")
	}
}

func TestServiceMergerDirectly(t *testing.T) {
	// Test the service merger directly
	merger := NewServiceMerger()

	// Add first service definition
	service1 := Service{
		Name:     "TestService",
		Domains:  []string{"Domain1", "Domain2"},
		Language: "Go",
	}
	merger.AddService(service1)

	// Add second service definition with same name
	service2 := Service{
		Name:       "TestService",
		Domains:    []string{"Domain2", "Domain3"}, // Domain2 should be deduplicated
		DataStores: []string{"db1", "cache1"},
	}
	merger.AddService(service2)

	// Get merged services
	merged := merger.GetMergedServices()

	if len(merged) != 1 {
		t.Fatalf("Expected 1 merged service, got %d", len(merged))
	}

	service := merged[0]

	// Check merged domains (should be 3: Domain1, Domain2, Domain3)
	if len(service.Domains) != 3 {
		t.Errorf("Expected 3 domains, got %d", len(service.Domains))
	}

	// Check language is preserved from first definition
	if service.Language != "Go" {
		t.Errorf("Expected language 'Go', got '%s'", service.Language)
	}

	// Check data stores are merged
	if len(service.DataStores) != 2 {
		t.Errorf("Expected 2 data stores, got %d", len(service.DataStores))
	}
}

func TestMergeStringSlices(t *testing.T) {
	slice1 := []string{"a", "b", "c"}
	slice2 := []string{"b", "c", "d"}

	merged := mergeStringSlices(slice1, slice2)

	expected := []string{"a", "b", "c", "d"}
	if len(merged) != len(expected) {
		t.Errorf("Expected %d items, got %d", len(expected), len(merged))
	}

	// Check all expected items are present
	mergedMap := make(map[string]bool)
	for _, item := range merged {
		mergedMap[item] = true
	}

	for _, expected := range expected {
		if !mergedMap[expected] {
			t.Errorf("Expected item %s not found in merged slice", expected)
		}
	}
}
