package parser

import (
	"testing"
)

func TestParser_UserManagementExample(t *testing.T) {
	// This is the exact content from /Users/tiago.carcao/projects/poc/craft/examples/user-management.craft
	dsl := `actors {
    user Business_User
    system CronA
    service Database
}

actor user Customer_Support

arch {
    presentation:
        WebApp[framework:react, ssl]
        MobileApp

    gateway:
        LoadBalancer[ssl:true] > APIGateway[type:nginx]
}

exposure default {
    to: Business_User
    through: APIGateway
}

domain User {
    Authentication
    Profile
}

services {
    UserService {
        domains: Authentication, Profile
        data-stores: user_db
        language: golang
    }
    CommsService {
        domains: Notifier
    }
}

// this is a comment
use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email format
        Authentication asks Database to check email uniqueness
        Profile creates user profile
        Authentication notifies "User Registered"

    when Profile listens "User Registered"
        Profile asks Database to store profile data
        Profile asks Notifier to send welcome email
}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error parsing user-management.craft, got: %v", err)
	}

	// Test 1: Validate Actor Parsing (4 actors with correct types)
	t.Run("ValidateActors", func(t *testing.T) {
		expectedActors := map[string]ActorType{
			"Business_User":     ActorTypeUser,
			"CronA":             ActorTypeSystem,
			"Database":          ActorTypeService,
			"Customer_Support":  ActorTypeUser,
		}

		if len(model.Actors) != 4 {
			t.Errorf("Expected 4 actors, got %d", len(model.Actors))
		}

		actorMap := make(map[string]ActorType)
		for _, actor := range model.Actors {
			actorMap[actor.Name] = actor.Type
		}

		for expectedName, expectedType := range expectedActors {
			actualType, exists := actorMap[expectedName]
			if !exists {
				t.Errorf("Expected actor '%s' not found", expectedName)
			} else if actualType != expectedType {
				t.Errorf("Expected actor '%s' to be type '%s', got '%s'", expectedName, expectedType, actualType)
			}
		}
	})

	// Test 2: Validate Architecture Block
	t.Run("ValidateArchitecture", func(t *testing.T) {
		if len(model.Architectures) != 1 {
			t.Errorf("Expected 1 architecture block, got %d", len(model.Architectures))
		}

		arch := model.Architectures[0]
		
		// Check presentation components
		if len(arch.Presentation) != 2 {
			t.Errorf("Expected 2 presentation components, got %d", len(arch.Presentation))
		}

		// Check for WebApp and MobileApp
		componentNames := make(map[string]bool)
		for _, comp := range arch.Presentation {
			componentNames[comp.Name] = true
		}

		if !componentNames["WebApp"] {
			t.Error("Expected WebApp component in presentation section")
		}
		if !componentNames["MobileApp"] {
			t.Error("Expected MobileApp component in presentation section")
		}

		// Check gateway components
		// The gateway line "LoadBalancer[ssl:true] > APIGateway[type:nginx]" should create a flow
		if len(arch.Gateway) != 1 {
			t.Errorf("Expected 1 gateway component (flow), got %d", len(arch.Gateway))
		} else if arch.Gateway[0].Type != ComponentTypeFlow {
			t.Errorf("Expected gateway component to be a flow, got %s", arch.Gateway[0].Type)
		} else if len(arch.Gateway[0].Chain) != 2 {
			t.Errorf("Expected gateway flow to have 2 components in chain, got %d", len(arch.Gateway[0].Chain))
		}
	})

	// Test 3: Validate Exposure
	t.Run("ValidateExposure", func(t *testing.T) {
		if len(model.Exposures) != 1 {
			t.Errorf("Expected 1 exposure, got %d", len(model.Exposures))
		}

		exposure := model.Exposures[0]
		if exposure.Name != "default" {
			t.Errorf("Expected exposure name 'default', got '%s'", exposure.Name)
		}

		if len(exposure.To) != 1 || exposure.To[0] != "Business_User" {
			t.Errorf("Expected exposure to 'Business_User', got %v", exposure.To)
		}

		if len(exposure.Through) != 1 || exposure.Through[0] != "APIGateway" {
			t.Errorf("Expected exposure through 'APIGateway', got %v", exposure.Through)
		}
	})

	// Test 4: Validate Domains
	t.Run("ValidateDomains", func(t *testing.T) {
		if len(model.Domains) != 1 {
			t.Errorf("Expected 1 domain definition, got %d", len(model.Domains))
		}

		domain := model.Domains[0]
		if domain.Name != "User" {
			t.Errorf("Expected domain name 'User', got '%s'", domain.Name)
		}

		expectedSubdomains := []string{"Authentication", "Profile"}
		if len(domain.SubDomains) != len(expectedSubdomains) {
			t.Errorf("Expected %d subdomains, got %d", len(expectedSubdomains), len(domain.SubDomains))
		}

		subdomainMap := make(map[string]bool)
		for _, subdomain := range domain.SubDomains {
			subdomainMap[subdomain] = true
		}

		for _, expected := range expectedSubdomains {
			if !subdomainMap[expected] {
				t.Errorf("Expected subdomain '%s' not found", expected)
			}
		}
	})

	// Test 5: Validate Services
	t.Run("ValidateServices", func(t *testing.T) {
		if len(model.Services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(model.Services))
		}

		serviceMap := make(map[string]*Service)
		for i := range model.Services {
			serviceMap[model.Services[i].Name] = &model.Services[i]
		}

		// Check UserService
		userService, exists := serviceMap["UserService"]
		if !exists {
			t.Error("Expected UserService not found")
		} else {
			expectedDomains := []string{"Authentication", "Profile"}
			if len(userService.Domains) != len(expectedDomains) {
				t.Errorf("Expected UserService to have %d domains, got %d", len(expectedDomains), len(userService.Domains))
			}

			if userService.Language != "golang" {
				t.Errorf("Expected UserService language 'golang', got '%s'", userService.Language)
			}

			expectedDataStores := []string{"user_db"}
			if len(userService.DataStores) != len(expectedDataStores) {
				t.Errorf("Expected UserService to have %d data stores, got %d", len(expectedDataStores), len(userService.DataStores))
			}
		}

		// Check CommsService
		commsService, exists := serviceMap["CommsService"]
		if !exists {
			t.Error("Expected CommsService not found")
		} else {
			expectedDomains := []string{"Notifier"}
			if len(commsService.Domains) != len(expectedDomains) {
				t.Errorf("Expected CommsService to have %d domains, got %d", len(expectedDomains), len(commsService.Domains))
			}
		}
	})

	// Test 6: Validate Use Cases
	t.Run("ValidateUseCases", func(t *testing.T) {
		if len(model.UseCases) != 1 {
			t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
		}

		useCase := model.UseCases[0]
		if useCase.Name != "User Registration" {
			t.Errorf("Expected use case name 'User Registration', got '%s'", useCase.Name)
		}

		if len(useCase.Scenarios) != 2 {
			t.Errorf("Expected 2 scenarios, got %d", len(useCase.Scenarios))
		}

		// Check first scenario - external trigger
		scenario1 := useCase.Scenarios[0]
		if scenario1.Trigger.Type != TriggerTypeExternal {
			t.Errorf("Expected first scenario to have external trigger, got %s", scenario1.Trigger.Type)
		}

		if scenario1.Trigger.Actor != "Business_User" {
			t.Errorf("Expected first scenario actor 'Business_User', got '%s'", scenario1.Trigger.Actor)
		}

		if scenario1.Trigger.Verb != "creates" {
			t.Errorf("Expected first scenario verb 'creates', got '%s'", scenario1.Trigger.Verb)
		}

		if scenario1.Trigger.Phrase != "Account" {
			t.Errorf("Expected first scenario phrase 'Account', got '%s'", scenario1.Trigger.Phrase)
		}

		// Check second scenario - listener trigger
		scenario2 := useCase.Scenarios[1]
		if scenario2.Trigger.Type != TriggerTypeDomainListen {
			t.Errorf("Expected second scenario to have domain listen trigger, got %s", scenario2.Trigger.Type)
		}

		if scenario2.Trigger.Domain != "Profile" {
			t.Errorf("Expected second scenario domain 'Profile', got '%s'", scenario2.Trigger.Domain)
		}

		if scenario2.Trigger.Event != "User Registered" {
			t.Errorf("Expected second scenario event 'User Registered', got '%s'", scenario2.Trigger.Event)
		}

		// Validate actions in first scenario
		expectedActions := 4
		if len(scenario1.Actions) != expectedActions {
			t.Errorf("Expected %d actions in first scenario, got %d", expectedActions, len(scenario1.Actions))
		}

		// Validate actions in second scenario
		expectedActions2 := 2
		if len(scenario2.Actions) != expectedActions2 {
			t.Errorf("Expected %d actions in second scenario, got %d", expectedActions2, len(scenario2.Actions))
		}
	})

	// Test 7: Integration Test - Ensure actors are available for use case triggers
	t.Run("ValidateActorIntegration", func(t *testing.T) {
		// Check that actors defined in the DSL are used in use cases
		actorNames := make(map[string]bool)
		for _, actor := range model.Actors {
			actorNames[actor.Name] = true
		}

		// Business_User should be defined as an actor and used in use case
		if !actorNames["Business_User"] {
			t.Error("Business_User should be defined as an actor")
		}

		// Check that Business_User is used in the use case trigger
		useCase := model.UseCases[0]
		scenario := useCase.Scenarios[0]
		if scenario.Trigger.Actor != "Business_User" {
			t.Errorf("Business_User should be used in use case trigger, got '%s'", scenario.Trigger.Actor)
		}
	})
}