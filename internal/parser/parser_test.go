package parser

import (	"testing"
)

func TestParser_InvalidSyntax(t *testing.T) {
	testCases := []struct {
		name        string
		dsl         string
		description string
	}{
		{
			name: "Missing verb",
			dsl: `use_case "Invalid Test 1" {
				when user creates account
					authentication "missing verb here"
			}`,
			description: "domain without verb should fail",
		},
		{
			name: "Incomplete trigger",
			dsl: `use_case "Invalid Test 2" {
				when
					domain verb phrase
			}`,
			description: "trigger without content should fail",
		},
		{
			name: "Unclosed string",
			dsl: `use_case "Invalid Test 3" {
				when user creates account
					notification notifies "unclosed string
			}`,
			description: "unclosed quoted string should fail",
		},
		{
			name: "Missing use case name",
			dsl: `use_case {
				when user creates account
					domain verb phrase
			}`,
			description: "use case without name should fail",
		},
		{
			name: "Malformed sync action",
			dsl: `use_case "Invalid Test 5" {
				when user creates account
					domain1 domain2 to do something
			}`,
			description: "sync action missing 'asks' keyword should fail",
		},
		{
			name: "Missing use case braces",
			dsl: `use_case "Invalid Test 6"
				when user creates account
					domain verb phrase`,
			description: "use case without braces should fail",
		},
		{
			name: "Empty quoted event",
			dsl: `use_case "Invalid Test 7" {
				when user creates account
					domain notifies ""
			}`,
			description: "empty quoted event might be invalid",
		},
	}

	parser := NewParser()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := parser.ParseString(tc.dsl)

			if err == nil {
				t.Errorf("Expected parse error for '%s', got nil. %s", tc.name, tc.description)
			}

			if model != nil {
				t.Errorf("Expected nil model for '%s', got non-nil model. %s", tc.name, tc.description)
			}

			t.Logf("Successfully caught error for '%s': %v", tc.name, err)
		})
	}
}

func TestParser_EmptyInput(t *testing.T) {
	dsl := ""

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	// Empty input should parse successfully but produce empty model
	if err != nil {
		t.Errorf("Expected no error for empty input, got: %v", err)
	}

	if model == nil {
		t.Error("Expected model, got nil")
	}

	if len(model.UseCases) != 0 {
		t.Errorf("Expected 0 use cases for empty input, got %d", len(model.UseCases))
	}
}

func TestParser_ComplexMixedDSL(t *testing.T) {
	dsl := `arch MainArch {
		presentation:
			WebApp[ssl, cache] > APIGateway[auth:jwt]
		gateway:
			LoadBalancer > ServiceMesh
	}

	exposure PublicAPI {
		to: external_clients
		of: UserService, OrderService
		through: APIGateway
	}

	services {
		UserService {
			domains: User, Profile
			data-stores: user_db
			language: golang
			deployment: canary(20% -> staging, 80% -> production)
		}
		OrderService {
			domains: Order, Payment
			data-stores: order_db, payment_db
			language: java
			deployment: blue_green
		}
	}

	use_case "User Registration" {
		when user creates account
			UserService marks the user as verified
			NotificationService notifies "User Registered"
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate all sections are present
	if len(model.Architectures) != 1 {
		t.Errorf("Expected 1 architecture, got %d", len(model.Architectures))
	}

	if len(model.Exposures) != 1 {
		t.Errorf("Expected 1 exposure, got %d", len(model.Exposures))
	}

	if len(model.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(model.Services))
	}

	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	// Quick validation of each section
	arch := model.Architectures[0]
	if arch.Name != "MainArch" {
		t.Errorf("Expected architecture name 'MainArch', got '%s'", arch.Name)
	}

	exposure := model.Exposures[0]
	if exposure.Name != "PublicAPI" {
		t.Errorf("Expected exposure name 'PublicAPI', got '%s'", exposure.Name)
	}

	// Validate services have deployment strategies
	userService := findServiceByName(model.Services, "UserService")
	if userService == nil {
		t.Fatal("UserService not found")
	}

	if userService.Deployment.Type != "canary" {
		t.Errorf("Expected UserService deployment type 'canary', got '%s'", userService.Deployment.Type)
	}

	orderService := findServiceByName(model.Services, "OrderService")
	if orderService == nil {
		t.Fatal("OrderService not found")
	}

	if orderService.Deployment.Type != "blue_green" {
		t.Errorf("Expected OrderService deployment type 'blue_green', got '%s'", orderService.Deployment.Type)
	}

	useCase := model.UseCases[0]
	if useCase.Name != "User Registration" {
		t.Errorf("Expected use case name 'User Registration', got '%s'", useCase.Name)
	}
}

func TestParser_EmptyDSLSections(t *testing.T) {
	testCases := []struct {
		name string
		dsl  string
	}{
		{
			name: "Empty services only",
			dsl:  `services {}`,
		},
		{
			name: "Multiple empty sections",
			dsl: `services {}
			
			arch EmptyArch {
				presentation:
					WebApp
				gateway:
					Gateway
			}`,
		},
		{
			name: "Empty DSL",
			dsl:  ``,
		},
	}

	parser := NewParser()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := parser.ParseString(tc.dsl)

			if err != nil {
				t.Fatalf("Expected no error for '%s', got: %v", tc.name, err)
			}

			if model == nil {
				t.Errorf("Expected model for '%s', got nil", tc.name)
			}
		})
	}
}

func TestParser_OrderIndependence(t *testing.T) {
	// Test that DSL sections can appear in any order
	dsl := `use_case "First Use Case" {
		when user logs in
			AuthService validates credentials
	}

	services {
		AuthService {
			domains: Auth, User
			deployment: blue_green
		}
	}

	arch MyArch {
		presentation:
			WebApp
		gateway:
			Gateway
	}

	exposure PublicAPI {
		to: clients
		of: AuthService
	}

	use_case "Second Use Case" {
		when user logs out
			AuthService invalidates session
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate all sections are present regardless of order
	if len(model.UseCases) != 2 {
		t.Errorf("Expected 2 use cases, got %d", len(model.UseCases))
	}

	if len(model.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(model.Services))
	}

	if len(model.Architectures) != 1 {
		t.Errorf("Expected 1 architecture, got %d", len(model.Architectures))
	}

	if len(model.Exposures) != 1 {
		t.Errorf("Expected 1 exposure, got %d", len(model.Exposures))
	}

	// Validate specific content
	expectedUseCaseNames := []string{"First Use Case", "Second Use Case"}
	for i, useCase := range model.UseCases {
		if useCase.Name != expectedUseCaseNames[i] {
			t.Errorf("Expected use case name '%s', got '%s'", expectedUseCaseNames[i], useCase.Name)
		}
	}
}

func BenchmarkParser_ComplexMixedDSL(b *testing.B) {
	dsl := `arch ComplexArch {
		presentation:
			WebApp[ssl,cache] > APIGateway[auth:jwt] > ServiceMesh[tracing]
		gateway:
			LoadBalancer > ReverseProxy
	}

	exposure ComplexAPI {
		to: external_clients, mobile_apps
		of: UserService, OrderService
		through: APIGateway, LoadBalancer
	}

	services {
		UserService {
			domains: User, Profile, Authentication
			data-stores: user_db, profile_cache
			language: golang
			deployment: canary(10% -> staging, 90% -> production)
		}
	}

	use_case "Complex Operation" {
		when user performs action
			UserService validates request
			OrderService processes order
			PaymentService charges payment
			NotificationService notifies "Operation Completed"
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

