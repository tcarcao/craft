package parser

import (
	"fmt"
	"testing"
)

func TestSimpleServiceParsing(t *testing.T) {
	dslContent := `services {
  TestService {
    domains: TestDomain
  }
}`

	fmt.Printf("Testing DSL:\n%s\n\n", dslContent)

	// Parse the DSL
	model, err := ParseDSLToModel(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	fmt.Printf("Parsed model: %+v\n", model)
	fmt.Printf("Number of services: %d\n", len(model.Services))

	if len(model.Services) > 0 {
		fmt.Printf("First service: %+v\n", model.Services[0])
	}

	// Verify we have services
	if len(model.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(model.Services))
	}

	service := model.Services[0]
	if service.Name != "TestService" {
		t.Errorf("Expected service name 'TestService', got '%s'", service.Name)
	}

	if len(service.Domains) != 1 {
		t.Errorf("Expected 1 domain, got %d", len(service.Domains))
	}

	if len(service.Domains) > 0 && service.Domains[0] != "TestDomain" {
		t.Errorf("Expected domain 'TestDomain', got '%s'", service.Domains[0])
	}
}

func TestServiceNameParsing(t *testing.T) {
	// Test DSL with all supported service name formats
	dslContent := `services {
  WalletService {
    domains: Wallet, WalletItemPurchase
    data-stores: wallet_db
  }
  "Order Service" {
    domains: OrderManagement
    data-stores: order_db
  }
  service-re-go-vas {
    domains: VASScheduling, VASProcessing
    data-stores: vas_db, vas_cache
  }
  "complex-service name" {
    domains: ComplexDomain
    data-stores: complex_db
  }
  simple_underscore_service {
    domains: UnderscoreDomain
    data-stores: underscore_db
  }
}

use_case "Test Case" {
  when Business_User performs action
    Wallet notifies "test event"
}`

	// Parse the DSL
	model, err := ParseDSLToModel(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	// Verify we have the expected number of services
	expectedServiceCount := 5
	if len(model.Services) != expectedServiceCount {
		t.Fatalf("Expected %d services, got %d", expectedServiceCount, len(model.Services))
	}

	// Define expected services with their properties
	expectedServices := []struct {
		name       string
		domains    []string
		dataStores []string
	}{
		{
			name:       "WalletService",
			domains:    []string{"Wallet", "WalletItemPurchase"},
			dataStores: []string{"wallet_db"},
		},
		{
			name:       "Order Service",
			domains:    []string{"OrderManagement"},
			dataStores: []string{"order_db"},
		},
		{
			name:       "service-re-go-vas",
			domains:    []string{"VASScheduling", "VASProcessing"},
			dataStores: []string{"vas_db", "vas_cache"},
		},
		{
			name:       "complex-service name",
			domains:    []string{"ComplexDomain"},
			dataStores: []string{"complex_db"},
		},
		{
			name:       "simple_underscore_service",
			domains:    []string{"UnderscoreDomain"},
			dataStores: []string{"underscore_db"},
		},
	}

	// Create a map for easier lookup
	serviceMap := make(map[string]Service)
	for _, service := range model.Services {
		serviceMap[service.Name] = service
	}

	// Verify each expected service
	for _, expected := range expectedServices {
		t.Run("Service_"+expected.name, func(t *testing.T) {
			service, exists := serviceMap[expected.name]
			if !exists {
				t.Fatalf("Service '%s' not found in parsed model", expected.name)
			}

			// Verify service name
			if service.Name != expected.name {
				t.Errorf("Expected service name '%s', got '%s'", expected.name, service.Name)
			}

			// Verify domains
			if len(service.Domains) != len(expected.domains) {
				t.Errorf("Expected %d domains for service '%s', got %d",
					len(expected.domains), expected.name, len(service.Domains))
			} else {
				for i, expectedDomain := range expected.domains {
					if service.Domains[i] != expectedDomain {
						t.Errorf("Expected domain '%s' at index %d for service '%s', got '%s'",
							expectedDomain, i, expected.name, service.Domains[i])
					}
				}
			}

			// Verify data stores
			if len(service.DataStores) != len(expected.dataStores) {
				t.Errorf("Expected %d data stores for service '%s', got %d",
					len(expected.dataStores), expected.name, len(service.DataStores))
			} else {
				for i, expectedDataStore := range expected.dataStores {
					if service.DataStores[i] != expectedDataStore {
						t.Errorf("Expected data store '%s' at index %d for service '%s', got '%s'",
							expectedDataStore, i, expected.name, service.DataStores[i])
					}
				}
			}
		})
	}

	// Verify use cases are still parsed correctly
	if len(model.UseCases) != 1 {
		t.Fatalf("Expected 1 use case, got %d", len(model.UseCases))
	}

	if model.UseCases[0].Name != "Test Case" {
		t.Errorf("Expected use case name 'Test Case', got '%s'", model.UseCases[0].Name)
	}
}

func TestServiceNameEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		dslContent  string
		expectedErr bool
		serviceName string
	}{
		{
			name: "Service with hyphens and numbers",
			dslContent: `services {
  service-123-test {
    domains: TestDomain,
    data-stores: test_db
  }
}`,
			expectedErr: false,
			serviceName: "service-123-test",
		},
		{
			name: "Service with underscores",
			dslContent: `services {
  service_test_123 {
    domains: TestDomain,
    data-stores: test_db
  }
}`,
			expectedErr: false,
			serviceName: "service_test_123",
		},
		{
			name: "Quoted service with special characters",
			dslContent: `services {
  "Service with spaces & symbols" {
    domains: TestDomain,
    data-stores: test_db
  }
}`,
			expectedErr: false,
			serviceName: "Service with spaces & symbols",
		},
		{
			name: "Single character service name",
			dslContent: `services {
  A {
    domains: TestDomain,
    data-stores: test_db
  }
}`,
			expectedErr: false,
			serviceName: "A",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := ParseDSLToModel(tc.dslContent)

			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(model.Services) != 1 {
				t.Fatalf("Expected 1 service, got %d", len(model.Services))
			}

			if model.Services[0].Name != tc.serviceName {
				t.Errorf("Expected service name '%s', got '%s'", tc.serviceName, model.Services[0].Name)
			}
		})
	}
}

func TestServiceLanguageParsing(t *testing.T) {
	dslContent := `services {
  TestService {
    domains: TestDomain
	language: golang
  }
}`

	fmt.Printf("Testing DSL:\n%s\n\n", dslContent)

	// Parse the DSL
	model, err := ParseDSLToModel(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL: %v", err)
	}

	if len(model.Services) > 0 {
		fmt.Printf("First service: %+v\n", model.Services[0])
	}

	// Verify we have services
	if len(model.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(model.Services))
	}

	service := model.Services[0]
	if service.Name != "TestService" {
		t.Errorf("Expected service name 'TestService', got '%s'", service.Name)
	}

	if service.Language != "golang" {
		t.Errorf("Expected language 'golang', got '%s'", service.Language)
	}
}

func TestEmptyServicesSection(t *testing.T) {
	dslContent := `services {
}

use_case "Test Case" {
  when Business_User performs action
    Wallet notifies "test event"
}`

	model, err := ParseDSLToModel(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL with empty services: %v", err)
	}

	if len(model.Services) != 0 {
		t.Errorf("Expected 0 services for empty services section, got %d", len(model.Services))
	}

	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}
}

func TestDSLWithoutServices(t *testing.T) {
	dslContent := `use_case "Test Case" {
  when Business_User performs action
    Wallet notifies "test event"
}`

	model, err := ParseDSLToModel(dslContent)
	if err != nil {
		t.Fatalf("Failed to parse DSL without services: %v", err)
	}

	if len(model.Services) != 0 {
		t.Errorf("Expected 0 services when no services section, got %d", len(model.Services))
	}

	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}
}

func TestParser_SingleServiceDefinition(t *testing.T) {
	dsl := `service PaymentService {
		domains: ProcessPayment, ValidateCard
		data-stores: payment_db, audit_db
		language: java
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify we have one service
	if len(model.Services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(model.Services))
	}

	service := model.Services[0]
	if service.Name != "PaymentService" {
		t.Errorf("Expected service name 'PaymentService', got '%s'", service.Name)
	}

	// Verify domains
	expectedDomains := []string{"ProcessPayment", "ValidateCard"}
	if len(service.Domains) != len(expectedDomains) {
		t.Errorf("Expected %d domains, got %d", len(expectedDomains), len(service.Domains))
	}
	for i, expectedDomain := range expectedDomains {
		if service.Domains[i] != expectedDomain {
			t.Errorf("Expected domain '%s' at index %d, got '%s'", expectedDomain, i, service.Domains[i])
		}
	}

	// Verify data stores
	expectedDataStores := []string{"payment_db", "audit_db"}
	if len(service.DataStores) != len(expectedDataStores) {
		t.Errorf("Expected %d data stores, got %d", len(expectedDataStores), len(service.DataStores))
	}
	for i, expectedStore := range expectedDataStores {
		if service.DataStores[i] != expectedStore {
			t.Errorf("Expected data store '%s' at index %d, got '%s'", expectedStore, i, service.DataStores[i])
		}
	}

	// Verify language
	if service.Language != "java" {
		t.Errorf("Expected language 'java', got '%s'", service.Language)
	}
}

func TestParser_MixedServiceDefinitions(t *testing.T) {
	dsl := `service PaymentService {
		domains: ProcessPayment
		language: java
	}

	services {
		UserService {
			domains: CreateAccount, UpdateProfile
			language: golang
		}
		InventoryService {
			domains: AddItem, RemoveItem
			language: nodejs
		}
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify we have three services total
	if len(model.Services) != 3 {
		t.Fatalf("Expected 3 services, got %d", len(model.Services))
	}

	// Find services by name
	paymentService := findServiceByName(model.Services, "PaymentService")
	userService := findServiceByName(model.Services, "UserService")
	inventoryService := findServiceByName(model.Services, "InventoryService")

	if paymentService == nil {
		t.Error("PaymentService not found")
	} else {
		if paymentService.Language != "java" {
			t.Errorf("Expected PaymentService language 'java', got '%s'", paymentService.Language)
		}
		if len(paymentService.Domains) != 1 || paymentService.Domains[0] != "ProcessPayment" {
			t.Errorf("Expected PaymentService domains [ProcessPayment], got %v", paymentService.Domains)
		}
	}

	if userService == nil {
		t.Error("UserService not found")
	} else {
		if userService.Language != "golang" {
			t.Errorf("Expected UserService language 'golang', got '%s'", userService.Language)
		}
	}

	if inventoryService == nil {
		t.Error("InventoryService not found")
	} else {
		if inventoryService.Language != "nodejs" {
			t.Errorf("Expected InventoryService language 'nodejs', got '%s'", inventoryService.Language)
		}
	}
}

func TestParser_ServiceWithCanaryDeployment(t *testing.T) {
	dsl := `services {
		PaymentService {
			domains: Payment, Billing
			data-stores: payment_db, audit_log
			language: golang
			deployment: canary(10% -> staging, 90% -> production)
		}
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(model.Services))
	}

	service := model.Services[0]
	if service.Name != "PaymentService" {
		t.Errorf("Expected service name 'PaymentService', got '%s'", service.Name)
	}

	// Validate deployment strategy
	if service.Deployment.Type != "canary" {
		t.Errorf("Expected deployment type 'canary', got '%s'", service.Deployment.Type)
	}

	if len(service.Deployment.Rules) != 2 {
		t.Errorf("Expected 2 deployment rules, got %d", len(service.Deployment.Rules))
	}

	// Validate first deployment rule (10% -> staging)
	rule1 := service.Deployment.Rules[0]
	if rule1.Percentage != "10%" {
		t.Errorf("Expected percentage '10%%', got '%s'", rule1.Percentage)
	}

	if rule1.Target != "staging" {
		t.Errorf("Expected target 'staging', got '%s'", rule1.Target)
	}

	// Validate second deployment rule (90% -> production)
	rule2 := service.Deployment.Rules[1]
	if rule2.Percentage != "90%" {
		t.Errorf("Expected percentage '90%%', got '%s'", rule2.Percentage)
	}

	if rule2.Target != "production" {
		t.Errorf("Expected target 'production', got '%s'", rule2.Target)
	}
}

func TestParser_ServiceWithBlueGreenDeployment(t *testing.T) {
	dsl := `services {
		OrderService {
			domains: Order, Inventory
			deployment: blue_green
		}
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	service := model.Services[0]
	if service.Deployment.Type != "blue_green" {
		t.Errorf("Expected deployment type 'blue_green', got '%s'", service.Deployment.Type)
	}

	if len(service.Deployment.Rules) != 0 {
		t.Errorf("Expected no deployment rules for simple blue_green, got %d", len(service.Deployment.Rules))
	}
}

func TestParser_ServiceWithRollingDeployment(t *testing.T) {
	dsl := `services {
		UserService {
			domains: User, Profile
			deployment: rolling(25% -> batch1, 25% -> batch2, 25% -> batch3, 25% -> batch4)
		}
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	service := model.Services[0]
	if service.Deployment.Type != "rolling" {
		t.Errorf("Expected deployment type 'rolling', got '%s'", service.Deployment.Type)
	}

	if len(service.Deployment.Rules) != 4 {
		t.Errorf("Expected 4 deployment rules, got %d", len(service.Deployment.Rules))
	}

	expectedTargets := []string{"batch1", "batch2", "batch3", "batch4"}
	for i, rule := range service.Deployment.Rules {
		if rule.Percentage != "25%" {
			t.Errorf("Expected percentage '25%%' for rule %d, got '%s'", i, rule.Percentage)
		}

		if rule.Target != expectedTargets[i] {
			t.Errorf("Expected target '%s' for rule %d, got '%s'", expectedTargets[i], i, rule.Target)
		}
	}
}

func TestParser_DeploymentEdgeCases(t *testing.T) {
	dsl := `services {
		Service1 {
			domains: Domain1
			deployment: canary(100% -> production)
		}
		Service2 {
			domains: Domain2
			deployment: rolling(50% -> half1, 50% -> half2)
		}
		Service3 {
			domains: Domain3
			deployment: blue_green
		}
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Services) != 3 {
		t.Errorf("Expected 3 services, got %d", len(model.Services))
	}

	// Test single rule canary
	service1 := findServiceByName(model.Services, "Service1")
	if service1.Deployment.Type != "canary" {
		t.Errorf("Expected canary deployment for Service1, got %s", service1.Deployment.Type)
	}

	if len(service1.Deployment.Rules) != 1 {
		t.Errorf("Expected 1 deployment rule for Service1, got %d", len(service1.Deployment.Rules))
	}

	if service1.Deployment.Rules[0].Percentage != "100%" {
		t.Errorf("Expected 100%% for Service1, got %s", service1.Deployment.Rules[0].Percentage)
	}

	// Test rolling with two targets
	service2 := findServiceByName(model.Services, "Service2")
	if service2.Deployment.Type != "rolling" {
		t.Errorf("Expected rolling deployment for Service2, got %s", service2.Deployment.Type)
	}

	if len(service2.Deployment.Rules) != 2 {
		t.Errorf("Expected 2 deployment rules for Service2, got %d", len(service2.Deployment.Rules))
	}

	// Test blue_green without config
	service3 := findServiceByName(model.Services, "Service3")
	if service3.Deployment.Type != "blue_green" {
		t.Errorf("Expected blue_green deployment for Service3, got %s", service3.Deployment.Type)
	}

	if len(service3.Deployment.Rules) != 0 {
		t.Errorf("Expected no deployment rules for Service3, got %d", len(service3.Deployment.Rules))
	}
}

func TestParser_DomainsWithServices(t *testing.T) {
	dsl := `domains {
		ECommerce {
			User
			Product
			Order
		}

		Analytics {
			Reporting
			Metrics
		}
	}

	services {
		UserService {
			domains: User, Product
			data-stores: user_db
		}
		AnalyticsService {
			domains: Reporting, Metrics
			data-stores: analytics_db, metrics_cache
			language: python
		}
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate both domains and services
	if len(model.Domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(model.Domains))
	}

	if len(model.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(model.Services))
	}

	// Validate domain structure
	ecommerce := model.Domains[0]
	if ecommerce.Name != "ECommerce" {
		t.Errorf("Expected domain name 'ECommerce', got '%s'", ecommerce.Name)
	}

	analytics := model.Domains[1]
	if analytics.Name != "Analytics" {
		t.Errorf("Expected domain name 'Analytics', got '%s'", analytics.Name)
	}

	// Validate services reference the subdomains
	userService := findServiceByName(model.Services, "UserService")
	if userService == nil {
		t.Fatal("UserService not found")
	}

	expectedUserDomains := []string{"User", "Product"}
	if len(userService.Domains) != len(expectedUserDomains) {
		t.Errorf("Expected %d domains for UserService, got %d", len(expectedUserDomains), len(userService.Domains))
	}

	for i, domain := range userService.Domains {
		if domain != expectedUserDomains[i] {
			t.Errorf("Expected domain '%s' for UserService, got '%s'", expectedUserDomains[i], domain)
		}
	}
}

func BenchmarkParser_ServiceWithDeployment(b *testing.B) {
	dsl := `services {
		BenchmarkService {
			domains: Domain1, Domain2, Domain3
			data-stores: db1, db2, cache1, cache2
			language: golang
			deployment: canary(5% -> canary, 20% -> staging, 75% -> production)
		}
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

// Helper function
func findServiceByName(services []Service, name string) *Service {
	for _, service := range services {
		if service.Name == name {
			return &service
		}
	}
	return nil
}

