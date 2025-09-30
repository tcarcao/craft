package parser

import (
	"fmt"
	"testing"
)

func TestParser_BasicExternalTrigger(t *testing.T) {
	dsl := `use_case "Simple Registration" {
		when user creates account
			authentication marks the user as verified
			notification sends welcome email
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate structure
	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	useCase := model.UseCases[0]
	if useCase.Name != "Simple Registration" {
		t.Errorf("Expected use case name 'Simple Registration', got '%s'", useCase.Name)
	}

	if len(useCase.Scenarios) != 1 {
		t.Errorf("Expected 1 scenario, got %d", len(useCase.Scenarios))
	}

	scenario := useCase.Scenarios[0]

	// Validate trigger
	if scenario.Trigger.Type != TriggerTypeExternal {
		t.Errorf("Expected external trigger, got %s", scenario.Trigger.Type)
	}

	if scenario.Trigger.Actor != "user" {
		t.Errorf("Expected actor 'user', got '%s'", scenario.Trigger.Actor)
	}

	if scenario.Trigger.Verb != "creates" {
		t.Errorf("Expected verb 'creates', got '%s'", scenario.Trigger.Verb)
	}

	if scenario.Trigger.Phrase != "account" {
		t.Errorf("Expected phrase 'account', got '%s'", scenario.Trigger.Phrase)
	}

	// Validate actions
	if len(scenario.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(scenario.Actions))
	}

	// First action - internal action
	action1 := scenario.Actions[0]
	if action1.Type != ActionTypeInternal {
		t.Errorf("Expected internal action, got %s", action1.Type)
	}

	if action1.Domain != "authentication" {
		t.Errorf("Expected domain 'authentication', got '%s'", action1.Domain)
	}

	if action1.Verb != "marks" {
		t.Errorf("Expected verb 'marks', got '%s'", action1.Verb)
	}

	if action1.Connector != "the" {
		t.Errorf("Expected connector 'as', got '%s'", action1.Connector)
	}

	// Second action - internal action
	action2 := scenario.Actions[1]
	if action2.Type != ActionTypeInternal {
		t.Errorf("Expected internal action, got %s", action2.Type)
	}

	if action2.Domain != "notification" {
		t.Errorf("Expected domain 'notification', got '%s'", action2.Domain)
	}
}

func TestParser_SyncActions(t *testing.T) {
	dsl := `use_case "Sync Test" {
		when user places order
			inventory asks warehouse to check availability
			payment asks billing to process transaction
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]

	if len(scenario.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(scenario.Actions))
	}

	// First sync action
	action1 := scenario.Actions[0]
	if action1.Type != ActionTypeSync {
		t.Errorf("Expected sync action, got %s", action1.Type)
	}

	if action1.Domain != "inventory" {
		t.Errorf("Expected domain 'inventory', got '%s'", action1.Domain)
	}

	if action1.TargetDomain != "warehouse" {
		t.Errorf("Expected target domain 'warehouse', got '%s'", action1.TargetDomain)
	}

	if action1.Connector != "to" {
		t.Errorf("Expected connector 'to', got '%s'", action1.Connector)
	}

	if action1.Phrase != "check availability" {
		t.Errorf("Expected phrase 'check availability', got '%s'", action1.Phrase)
	}

	// Second sync action
	action2 := scenario.Actions[1]
	if action2.Type != ActionTypeSync {
		t.Errorf("Expected sync action, got %s", action2.Type)
	}

	if action2.Domain != "payment" {
		t.Errorf("Expected domain 'payment', got '%s'", action2.Domain)
	}

	if action2.TargetDomain != "billing" {
		t.Errorf("Expected target domain 'billing', got '%s'", action2.TargetDomain)
	}
}

func TestParser_AsyncActions(t *testing.T) {
	dsl := `use_case "Async Test" {
		when user registers
			authentication notifies "User Registered"
			email notifies "Welcome Email Sent"
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]

	if len(scenario.Actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(scenario.Actions))
	}

	// First async action
	action1 := scenario.Actions[0]
	if action1.Type != ActionTypeAsync {
		t.Errorf("Expected async action, got %s", action1.Type)
	}

	if action1.Domain != "authentication" {
		t.Errorf("Expected domain 'authentication', got '%s'", action1.Domain)
	}

	if action1.Event != "User Registered" {
		t.Errorf("Expected event 'User Registered', got '%s'", action1.Event)
	}

	// Second async action
	action2 := scenario.Actions[1]
	if action2.Type != ActionTypeAsync {
		t.Errorf("Expected async action, got %s", action2.Type)
	}

	if action2.Domain != "email" {
		t.Errorf("Expected domain 'email', got '%s'", action2.Domain)
	}

	if action2.Event != "Welcome Email Sent" {
		t.Errorf("Expected event 'Welcome Email Sent', got '%s'", action2.Event)
	}
}

func TestParser_DomainListenerTrigger(t *testing.T) {
	dsl := `use_case "Domain Listener Test" {
		when authentication listens "User Verified"
			profile creates user profile
			database stores user data
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]

	// Validate trigger
	if scenario.Trigger.Type != TriggerTypeDomainListen {
		t.Errorf("Expected domain listen trigger, got %s", scenario.Trigger.Type)
	}

	if scenario.Trigger.Domain != "authentication" {
		t.Errorf("Expected domain 'authentication', got '%s'", scenario.Trigger.Domain)
	}

	if scenario.Trigger.Event != "User Verified" {
		t.Errorf("Expected event 'User Verified', got '%s'", scenario.Trigger.Event)
	}

	expectedDesc := "when authentication listens \"User Verified\""
	if scenario.Trigger.Description != expectedDesc {
		t.Errorf("Expected description '%s', got '%s'", expectedDesc, scenario.Trigger.Description)
	}
}

func TestParser_EventTrigger(t *testing.T) {
	dsl := `use_case "Event Trigger Test" {
		when "Order Placed"
			notification sends confirmation email
			analytics tracks order event
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]

	// Validate trigger
	if scenario.Trigger.Type != TriggerTypeEvent {
		t.Errorf("Expected event trigger, got %s", scenario.Trigger.Type)
	}

	if scenario.Trigger.Event != "Order Placed" {
		t.Errorf("Expected event 'Order Placed', got '%s'", scenario.Trigger.Event)
	}

	expectedDesc := "when \"Order Placed\""
	if scenario.Trigger.Description != expectedDesc {
		t.Errorf("Expected description '%s', got '%s'", expectedDesc, scenario.Trigger.Description)
	}
}

func TestParser_VASWalletExample(t *testing.T) {
	dsl := `use_case "Purchase VAS to Wallet" {
		when Business_User purchases VAS to the wallet
			WalletItemPurchase asks Wallet to initiate a VAS addition
			Wallet notifies "VAS Addition Request Approved"

		when WalletItemPurchase listens "VAS Addition Request Approved"
			WalletItemPurchase asks OrderManagement to create an Order
			OrderManagement notifies "Order Created"
			OrderManagement marks the Order as payment deferred
			OrderManagement notifies "Order payment deferred"
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate use case
	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	useCase := model.UseCases[0]
	if useCase.Name != "Purchase VAS to Wallet" {
		t.Errorf("Expected use case name 'Purchase VAS to Wallet', got '%s'", useCase.Name)
	}

	// Validate scenarios
	if len(useCase.Scenarios) != 2 {
		t.Errorf("Expected 2 scenarios, got %d", len(useCase.Scenarios))
	}

	// First scenario - external trigger
	scenario1 := useCase.Scenarios[0]
	if scenario1.Trigger.Type != TriggerTypeExternal {
		t.Errorf("Expected external trigger, got %s", scenario1.Trigger.Type)
	}

	if scenario1.Trigger.Actor != "Business_User" {
		t.Errorf("Expected actor 'Business_User', got '%s'", scenario1.Trigger.Actor)
	}

	if len(scenario1.Actions) != 2 {
		t.Errorf("Expected 2 actions in first scenario, got %d", len(scenario1.Actions))
	}

	// Check sync action
	syncAction := scenario1.Actions[0]
	if syncAction.Type != ActionTypeSync {
		t.Errorf("Expected sync action, got %s", syncAction.Type)
	}

	if syncAction.Domain != "WalletItemPurchase" {
		t.Errorf("Expected domain 'WalletItemPurchase', got '%s'", syncAction.Domain)
	}

	if syncAction.TargetDomain != "Wallet" {
		t.Errorf("Expected target domain 'Wallet', got '%s'", syncAction.TargetDomain)
	}

	// Check async action
	asyncAction := scenario1.Actions[1]
	if asyncAction.Type != ActionTypeAsync {
		t.Errorf("Expected async action, got %s", asyncAction.Type)
	}

	if asyncAction.Event != "VAS Addition Request Approved" {
		t.Errorf("Expected event 'VAS Addition Request Approved', got '%s'", asyncAction.Event)
	}

	// Second scenario - domain listener
	scenario2 := useCase.Scenarios[1]
	if scenario2.Trigger.Type != TriggerTypeDomainListen {
		t.Errorf("Expected domain listen trigger, got %s", scenario2.Trigger.Type)
	}

	if scenario2.Trigger.Domain != "WalletItemPurchase" {
		t.Errorf("Expected domain 'WalletItemPurchase', got '%s'", scenario2.Trigger.Domain)
	}

	if len(scenario2.Actions) != 4 {
		t.Errorf("Expected 4 actions in second scenario, got %d", len(scenario2.Actions))
	}
}

func TestParser_MultipleUseCases(t *testing.T) {
	dsl := `use_case "User Registration" {
		when user creates account
			authentication marks user as verified
	}
	
	use_case "Order Processing" {
		when customer places order
			inventory checks availability
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.UseCases) != 2 {
		t.Errorf("Expected 2 use cases, got %d", len(model.UseCases))
	}

	expectedNames := []string{"User Registration", "Order Processing"}
	for i, useCase := range model.UseCases {
		if useCase.Name != expectedNames[i] {
			t.Errorf("Expected use case name '%s', got '%s'", expectedNames[i], useCase.Name)
		}
	}
}

func TestParser_ComplexMultipleCases(t *testing.T) {
	dsl := `use_case "Purchase VAS to Wallet" {
  when Business_User purchases VAS to wallet
    WalletItemPurchase asks Wallet to initiate a VAS addition
    Wallet notifies "VAS Addition Request Approved"

  when WalletItemPurchase listens "VAS Addition Request Approved"
    WalletItemPurchase asks OrderManagement to create an Order
    OrderManagement notifies "Order Created"
    OrderManagement marks the Order as payment deferred
    OrderManagement notifies "Order payment deferred"

  when OrderFulfilment listens "Order payment deferred"
    OrderFulfilment asks WalletItemAddition to fulfil the addition request
    WalletItemAddition asks Wallet to add VAS
    Wallet notifies "VAS added"
      
  when OrderFulfilment listens "VAS added"
    OrderFulfilment asks OrderManagement to mark the Order as fulfiled
    OrderManagement notifies "Order completed"
}

use_case "VAS scheduled and no VAS in Wallet" {
  when Business_User schedules VAS
    VASScheduling asks Wallet to reserve a VAS
    Wallet creates an unconfirmed VAS reservation
    Wallet notifies "VAS Addition Request Approved"
    VASScheduling drafts a VAS schedule

  when VASScheduling listens "VAS Addition Request Approved"
    VASScheduling asks OrderManagement to create an Order
    OrderManagement notifies "Order Created"
    OrderManagement marks the Order as payment deferred
    OrderManagement notifies "Order payment deferred"

  when OrderFulfilment listens "Order payment deferred"
    OrderFulfilment asks WalletItemAddition to fulfil the addition request
    WalletItemAddition asks Wallet to add VAS
    Wallet notifies "VAS added"
      
  when OrderFulfilment listens "VAS added"
    OrderFulfilment asks OrderManagement to mark the Order as fulfiled
    OrderManagement notifies "Order completed"
}

use_case "VAS scheduled and VAS in Wallet" {
  when Business_User schedules VAS
    VASScheduling asks Wallet to reserve a VAS
    Wallet reserves a VAS
    Wallet notifies "VAS Reserved"
    VASScheduling creates a VAS schedule
}

use_case "VAS unscheduled" {
  when Business_User unschedules a VAS scheduling
    VASScheduling asks Wallet to cancel the VAS reservation
    Wallet cancels VAS reservation
    Wallet notifies "VAS Reservation Canceled"
    VASScheduling cancels the VAS schedule
}

use_case "VAS expires" {
  when CRON identifies a VAS expiring
    Wallets removes VAS
    Wallets notifies "VAS Expired"
}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.UseCases) != 5 {
		t.Errorf("Expected 5 use cases, got %d", len(model.UseCases))
	}
}

func TestParser_ConnectorVariations(t *testing.T) {
	dsl := `use_case "Connector Test" {
		when user updates profile
			validation asks database to verify data
			profile marks the user as verified
			notification sends an email to user
			audit logs with detailed information
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]
	actions := scenario.Actions

	if len(actions) != 4 {
		t.Errorf("Expected 4 actions, got %d", len(actions))
		return
	}

	// Check sync action with "to" connector
	if actions[0].Type != ActionTypeSync {
		t.Errorf("Expected sync action, got %s", actions[0].Type)
	}
	if actions[0].Connector != "to" {
		t.Errorf("Expected sync action with 'to' connector, got '%s'", actions[0].Connector)
	}
	if actions[0].Phrase != "verify data" {
		t.Errorf("Expected phrase 'verify data', got '%s'", actions[0].Phrase)
	}

	// Check internal action with "the" connector
	if actions[1].Type != ActionTypeInternal {
		t.Errorf("Expected internal action, got %s", actions[1].Type)
	}
	if actions[1].Connector != "the" {
		t.Errorf("Expected internal action with 'the' connector, got '%s'", actions[1].Connector)
	}
	if actions[1].Phrase != "user as verified" {
		t.Errorf("Expected phrase 'user as verified', got '%s'", actions[1].Phrase)
	}

	// Check internal action with "an" connector
	if actions[2].Type != ActionTypeInternal {
		t.Errorf("Expected internal action, got %s", actions[2].Type)
	}
	if actions[2].Connector != "an" {
		t.Errorf("Expected internal action with 'an' connector, got '%s'", actions[2].Connector)
	}
	if actions[2].Phrase != "email to user" {
		t.Errorf("Expected phrase 'email to user', got '%s'", actions[2].Phrase)
	}

	// Check internal action with "with" connector
	if actions[3].Type != ActionTypeInternal {
		t.Errorf("Expected internal action, got %s", actions[3].Type)
	}
	if actions[3].Connector != "with" {
		t.Errorf("Expected internal action with 'with' connector, got '%s'", actions[3].Connector)
	}
	if actions[3].Phrase != "detailed information" {
		t.Errorf("Expected phrase 'detailed information', got '%s'", actions[3].Phrase)
	}
}

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

func TestParser_ActionDescriptions(t *testing.T) {
	dsl := `use_case "Description Test" {
		when user creates account
			WalletService asks PaymentService to validate card
			NotificationService notifies "Account Created"
			ProfileService marks the user as active
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	actions := model.UseCases[0].Scenarios[0].Actions

	expectedDescriptions := []string{
		"WalletService asks PaymentService to validate card",
		"NotificationService notifies \"Account Created\"",
		"ProfileService marks the user as active",
	}

	for i, action := range actions {
		if action.Description != expectedDescriptions[i] {
			t.Errorf("Expected description '%s', got '%s'", expectedDescriptions[i], action.Description)
		}
	}
}

func TestParser_IDGeneration(t *testing.T) {
	dsl := `use_case "ID Test" {
		when user creates account
			authentication marks user as verified
		
		when user updates profile
			profile validates data
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Collect all IDs to check uniqueness
	idSet := make(map[string]bool)

	for _, useCase := range model.UseCases {
		for _, scenario := range useCase.Scenarios {
			if scenario.ID == "" {
				t.Error("Scenario ID should not be empty")
			}

			if idSet[scenario.ID] {
				t.Errorf("Duplicate scenario ID: %s", scenario.ID)
			}
			idSet[scenario.ID] = true

			for _, action := range scenario.Actions {
				if action.ID == "" {
					t.Error("Action ID should not be empty")
				}

				if idSet[action.ID] {
					t.Errorf("Duplicate action ID: %s", action.ID)
				}
				idSet[action.ID] = true
			}
		}
	}

	// Should have at least 4 unique IDs (2 scenarios + 2 actions)
	if len(idSet) < 4 {
		t.Errorf("Expected at least 4 unique IDs, got %d", len(idSet))
	}
}

// Benchmark tests
func BenchmarkParser_SimpleCase(b *testing.B) {
	dsl := `use_case "Benchmark Test" {
		when user creates account
			authentication marks user as verified
			notification sends welcome email
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

func BenchmarkParser_ComplexCase(b *testing.B) {
	dsl := `use_case "Complex Benchmark" {
		when Business_User purchases VAS to the wallet
			WalletItemPurchase asks Wallet to initiate a VAS addition
			Wallet notifies "VAS Addition Request Approved"

		when WalletItemPurchase listens "VAS Addition Request Approved"
			WalletItemPurchase asks OrderManagement to create an Order
			OrderManagement notifies "Order Created"
			OrderManagement marks the Order as payment deferred
			OrderManagement notifies "Order payment deferred"

		when OrderFulfilment listens "Order payment deferred"
			OrderFulfilment asks WalletItemAddition to fulfil the addition request
			WalletItemAddition asks Wallet to add VAS
			Wallet notifies "VAS added"
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

// Test single service definition (service name: { ... })
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

// Test mixed service definitions (both single service and services block)
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

// Test Architecture Definitions
func TestParser_BasicArchitectureDefinition(t *testing.T) {
	dsl := `arch {
		presentation:
			Frontend
		gateway:
			APIGateway
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate architecture structure
	if len(model.Architectures) != 1 {
		t.Errorf("Expected 1 architecture, got %d", len(model.Architectures))
	}

	arch := model.Architectures[0]
	if arch.Name != "" {
		t.Errorf("Expected empty name for unnamed arch, got '%s'", arch.Name)
	}

	// Validate presentation section
	if len(arch.Presentation) != 1 {
		t.Errorf("Expected 1 presentation component, got %d", len(arch.Presentation))
	}

	if arch.Presentation[0].Name != "Frontend" {
		t.Errorf("Expected presentation component 'Frontend', got '%s'", arch.Presentation[0].Name)
	}

	// Validate gateway section
	if len(arch.Gateway) != 1 {
		t.Errorf("Expected 1 gateway component, got %d", len(arch.Gateway))
	}

	if arch.Gateway[0].Name != "APIGateway" {
		t.Errorf("Expected gateway component 'APIGateway', got '%s'", arch.Gateway[0].Name)
	}
}

func TestParser_NamedArchitectureDefinition(t *testing.T) {
	dsl := `arch MicroservicesArch {
		presentation:
			WebApp
			MobileApp
		gateway:
			LoadBalancer
			ReverseProxy
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	arch := model.Architectures[0]
	if arch.Name != "MicroservicesArch" {
		t.Errorf("Expected architecture name 'MicroservicesArch', got '%s'", arch.Name)
	}

	if len(arch.Presentation) != 2 {
		t.Errorf("Expected 2 presentation components, got %d", len(arch.Presentation))
	}

	expectedPresentation := []string{"WebApp", "MobileApp"}
	for i, component := range arch.Presentation {
		if component.Name != expectedPresentation[i] {
			t.Errorf("Expected presentation component '%s', got '%s'", expectedPresentation[i], component.Name)
		}
	}

	if len(arch.Gateway) != 2 {
		t.Errorf("Expected 2 gateway components, got %d", len(arch.Gateway))
	}

	expectedGateway := []string{"LoadBalancer", "ReverseProxy"}
	for i, component := range arch.Gateway {
		if component.Name != expectedGateway[i] {
			t.Errorf("Expected gateway component '%s', got '%s'", expectedGateway[i], component.Name)
		}
	}
}

func TestParser_ComponentFlow(t *testing.T) {
	dsl := `arch FlowArch {
		presentation:
			WebClient > APIGateway > ServiceMesh
		gateway:
			LoadBalancer > AuthService > BusinessLogic
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	arch := model.Architectures[0]

	// Validate presentation flow
	if len(arch.Presentation) != 1 {
		t.Errorf("Expected 1 presentation flow, got %d", len(arch.Presentation))
	}

	presentationFlow := arch.Presentation[0]
	if presentationFlow.Type != ComponentTypeFlow {
		t.Errorf("Expected component type Flow, got %s", presentationFlow.Type)
	}

	expectedPresentationChain := []string{"WebClient", "APIGateway", "ServiceMesh"}
	if len(presentationFlow.Chain) != len(expectedPresentationChain) {
		t.Errorf("Expected %d components in presentation chain, got %d", len(expectedPresentationChain), len(presentationFlow.Chain))
	}

	for i, component := range presentationFlow.Chain {
		if component.Name != expectedPresentationChain[i] {
			t.Errorf("Expected chain component '%s', got '%s'", expectedPresentationChain[i], component.Name)
		}
	}

	// Validate gateway flow
	if len(arch.Gateway) != 1 {
		t.Errorf("Expected 1 gateway flow, got %d", len(arch.Gateway))
	}

	gatewayFlow := arch.Gateway[0]
	expectedGatewayChain := []string{"LoadBalancer", "AuthService", "BusinessLogic"}
	if len(gatewayFlow.Chain) != len(expectedGatewayChain) {
		t.Errorf("Expected %d components in gateway chain, got %d", len(expectedGatewayChain), len(gatewayFlow.Chain))
	}

	for i, component := range gatewayFlow.Chain {
		if component.Name != expectedGatewayChain[i] {
			t.Errorf("Expected chain component '%s', got '%s'", expectedGatewayChain[i], component.Name)
		}
	}
}

func TestParser_ComponentModifiers(t *testing.T) {
	dsl := `arch ModifiedArch {
		presentation:
			WebApp[ssl, cache]
			MobileApp[auth:oauth, timeout:30s]
		gateway:
			APIGateway[rate_limit:1000, protocol:https]
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	arch := model.Architectures[0]

	// Validate WebApp modifiers
	webApp := arch.Presentation[0]
	if webApp.Name != "WebApp" {
		t.Errorf("Expected component name 'WebApp', got '%s'", webApp.Name)
	}

	if len(webApp.Modifiers) != 2 {
		t.Errorf("Expected 2 modifiers for WebApp, got %d", len(webApp.Modifiers))
	}

	expectedWebAppModifiers := []ComponentModifier{
		{Key: "ssl", Value: ""},
		{Key: "cache", Value: ""},
	}

	for i, modifier := range webApp.Modifiers {
		if modifier.Key != expectedWebAppModifiers[i].Key {
			t.Errorf("Expected modifier key '%s', got '%s'", expectedWebAppModifiers[i].Key, modifier.Key)
		}
		if modifier.Value != expectedWebAppModifiers[i].Value {
			t.Errorf("Expected modifier value '%s', got '%s'", expectedWebAppModifiers[i].Value, modifier.Value)
		}
	}

	// Validate MobileApp modifiers with values
	mobileApp := arch.Presentation[1]
	if mobileApp.Name != "MobileApp" {
		t.Errorf("Expected component name 'MobileApp', got '%s'", mobileApp.Name)
	}

	if len(mobileApp.Modifiers) != 2 {
		t.Errorf("Expected 2 modifiers for MobileApp, got %d", len(mobileApp.Modifiers))
	}

	expectedMobileModifiers := []ComponentModifier{
		{Key: "auth", Value: "oauth"},
		{Key: "timeout", Value: "30s"},
	}

	for i, modifier := range mobileApp.Modifiers {
		if modifier.Key != expectedMobileModifiers[i].Key {
			t.Errorf("Expected modifier key '%s', got '%s'", expectedMobileModifiers[i].Key, modifier.Key)
		}
		if modifier.Value != expectedMobileModifiers[i].Value {
			t.Errorf("Expected modifier value '%s', got '%s'", expectedMobileModifiers[i].Value, modifier.Value)
		}
	}
}

func TestParser_ComponentFlowWithModifiers(t *testing.T) {
	dsl := `arch ComplexFlowArch {
		presentation:
			WebClient[ssl] > APIGateway[auth:jwt] > ServiceMesh[tracing]
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	arch := model.Architectures[0]
	flow := arch.Presentation[0]

	if flow.Type != ComponentTypeFlow {
		t.Errorf("Expected component type Flow, got %s", flow.Type)
	}

	if len(flow.Chain) != 3 {
		t.Errorf("Expected 3 components in chain, got %d", len(flow.Chain))
	}

	// Validate WebClient with ssl modifier
	webClient := flow.Chain[0]
	if webClient.Name != "WebClient" {
		t.Errorf("Expected component name 'WebClient', got '%s'", webClient.Name)
	}

	if len(webClient.Modifiers) != 1 {
		t.Errorf("Expected 1 modifier for WebClient, got %d", len(webClient.Modifiers))
	}

	if webClient.Modifiers[0].Key != "ssl" {
		t.Errorf("Expected modifier key 'ssl', got '%s'", webClient.Modifiers[0].Key)
	}

	// Validate APIGateway with auth:jwt modifier
	apiGateway := flow.Chain[1]
	if apiGateway.Name != "APIGateway" {
		t.Errorf("Expected component name 'APIGateway', got '%s'", apiGateway.Name)
	}

	if len(apiGateway.Modifiers) != 1 {
		t.Errorf("Expected 1 modifier for APIGateway, got %d", len(apiGateway.Modifiers))
	}

	if apiGateway.Modifiers[0].Key != "auth" || apiGateway.Modifiers[0].Value != "jwt" {
		t.Errorf("Expected modifier auth:jwt, got %s:%s", apiGateway.Modifiers[0].Key, apiGateway.Modifiers[0].Value)
	}
}

// Test Exposure Definitions
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

// Test Enhanced Services with Deployment
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

// Test Mixed DSL Content
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

// Test Error Cases for New Grammar
func TestParser_InvalidArchitecture(t *testing.T) {
	testCases := []struct {
		name string
		dsl  string
	}{
		{
			name: "Missing architecture sections",
			dsl: `arch EmptyArch {
			}`,
		},
		{
			name: "Invalid component flow syntax",
			dsl: `arch BadFlow {
				presentation:
					WebApp > > APIGateway
			}`,
		},
		{
			name: "Malformed component modifiers",
			dsl: `arch BadModifiers {
				presentation:
					WebApp[ssl cache]
			}`,
		},
		{
			name: "Invalid deployment strategy",
			dsl: `services {
				TestService {
					domains: Test
					deployment: invalid_strategy
				}
			}`,
		},
		{
			name: "Malformed deployment config",
			dsl: `services {
				TestService {
					domains: Test
					deployment: canary(invalid config)
				}
			}`,
		},
		{
			name: "Invalid percentage in deployment",
			dsl: `services {
				TestService {
					domains: Test
					deployment: canary(invalid% -> target)
				}
			}`,
		},
	}

	parser := NewParser()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := parser.ParseString(tc.dsl)

			if err == nil {
				t.Errorf("Expected parse error for '%s', got nil", tc.name)
			}

			if model != nil {
				t.Errorf("Expected nil model for '%s', got non-nil model", tc.name)
			}

			t.Logf("Successfully caught error for '%s': %v", tc.name, err)
		})
	}
}

// Helper function to find service by name
func findServiceByName(services []Service, name string) *Service {
	for _, service := range services {
		if service.Name == name {
			return &service
		}
	}
	return nil
}

// Test Enhanced Connector Words
func TestParser_EnhancedConnectorWords(t *testing.T) {
	dsl := `use_case "Enhanced Connectors Test" {
		when user updates profile
			validation asks database to verify data
			profile marks the user as verified
			notification sends an email to user
			audit logs with detailed information
			cache stores from database
			security checks in the system
			monitor records on the dashboard
			backup saves at regular intervals
			analytics tracks for reporting
			logger writes by timestamp
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]
	actions := scenario.Actions

	if len(actions) != 10 {
		t.Errorf("Expected 10 actions, got %d", len(actions))
		return
	}

	expectedConnectors := []string{"to", "the", "an", "with", "from", "in", "on", "at", "for", "by"}
	expectedTypes := []ActionType{
		ActionTypeSync,     // asks ... to
		ActionTypeInternal, // marks the
		ActionTypeInternal, // sends an
		ActionTypeInternal, // logs with
		ActionTypeInternal, // stores from
		ActionTypeInternal, // checks in
		ActionTypeInternal, // records on
		ActionTypeInternal, // saves at
		ActionTypeInternal, // tracks for
		ActionTypeInternal, // writes by
	}

	for i, action := range actions {
		if action.Type != expectedTypes[i] {
			t.Errorf("Action %d: Expected type %s, got %s", i, expectedTypes[i], action.Type)
		}

		// Skip sync action connector check as it has different structure
		if action.Type == ActionTypeSync {
			continue
		}

		if action.Connector != expectedConnectors[i] {
			t.Errorf("Action %d: Expected connector '%s', got '%s'", i, expectedConnectors[i], action.Connector)
		}
	}
}

// Test Mixed Architecture and Use Case
func TestParser_ArchitectureWithUseCases(t *testing.T) {
	dsl := `arch WebArch {
		presentation:
			ReactApp[spa] > CDN[cache:aggressive]
		gateway:
			NginxProxy > APIGateway[rate_limit:1000]
	}

	use_case "User Authentication" {
		when user logs in
			AuthService validates credentials
			SessionService creates session
			NotificationService notifies "User Logged In"
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate both architecture and use case are parsed
	if len(model.Architectures) != 1 {
		t.Errorf("Expected 1 architecture, got %d", len(model.Architectures))
	}

	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	// Validate architecture details
	arch := model.Architectures[0]
	if arch.Name != "WebArch" {
		t.Errorf("Expected architecture name 'WebArch', got '%s'", arch.Name)
	}

	// Validate use case is properly parsed
	useCase := model.UseCases[0]
	if useCase.Name != "User Authentication" {
		t.Errorf("Expected use case name 'User Authentication', got '%s'", useCase.Name)
	}

	if len(useCase.Scenarios) != 1 {
		t.Errorf("Expected 1 scenario, got %d", len(useCase.Scenarios))
	}

	scenario := useCase.Scenarios[0]
	if len(scenario.Actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(scenario.Actions))
	}
}

// Benchmark new features
func BenchmarkParser_ArchitectureDefinition(b *testing.B) {
	dsl := `arch BenchmarkArch {
		presentation:
			WebApp[ssl,cache] > APIGateway[auth:jwt] > ServiceMesh[tracing]
		gateway:
			LoadBalancer[algorithm:round_robin] > ReverseProxy[timeout:30s]
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

// Test edge cases and complex scenarios
func TestParser_MultipleArchitectures(t *testing.T) {
	dsl := `arch WebArch {
		presentation:
			WebApp
		gateway:
			APIGateway
	}

	arch MobileArch {
		presentation:
			MobileApp[native]
		gateway:
			MobileGateway[protocol:grpc]
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Architectures) != 2 {
		t.Errorf("Expected 2 architectures, got %d", len(model.Architectures))
	}

	archNames := []string{"WebArch", "MobileArch"}
	for i, arch := range model.Architectures {
		if arch.Name != archNames[i] {
			t.Errorf("Expected architecture name '%s', got '%s'", archNames[i], arch.Name)
		}
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

func TestParser_ComplexComponentChains(t *testing.T) {
	dsl := `arch ComplexChainArch {
		presentation:
			WebClient[ssl:tls1.3] > CDN[cache:aggressive,geo:distributed] > LoadBalancer[algorithm:weighted] > APIGateway[auth:oauth2,rate_limit:5000] > ServiceMesh[tracing:jaeger,metrics:prometheus]
		gateway:
			EdgeProxy[ddos_protection] > AuthService[provider:auth0] > RateLimiter[burst:100] > Router[strategy:path_based]
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	arch := model.Architectures[0]

	// Validate presentation chain
	presentationFlow := arch.Presentation[0]
	if len(presentationFlow.Chain) != 5 {
		t.Errorf("Expected 5 components in presentation chain, got %d", len(presentationFlow.Chain))
	}

	expectedPresentationComponents := []struct {
		name      string
		modifiers int
	}{
		{"WebClient", 1},
		{"CDN", 2},
		{"LoadBalancer", 1},
		{"APIGateway", 2},
		{"ServiceMesh", 2},
	}

	for i, expected := range expectedPresentationComponents {
		component := presentationFlow.Chain[i]
		if component.Name != expected.name {
			t.Errorf("Expected component name '%s', got '%s'", expected.name, component.Name)
		}

		if len(component.Modifiers) != expected.modifiers {
			t.Errorf("Expected %d modifiers for %s, got %d", expected.modifiers, expected.name, len(component.Modifiers))
		}
	}

	// Validate gateway chain
	gatewayFlow := arch.Gateway[0]
	if len(gatewayFlow.Chain) != 4 {
		t.Errorf("Expected 4 components in gateway chain, got %d", len(gatewayFlow.Chain))
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

func TestParser_ConnectorWordsInPhrases(t *testing.T) {
	dsl := `use_case "Connector Words Test" {
		when user submits form
			validation checks the data for accuracy
			database stores the record in the table
			notification sends an email to the user with confirmation
			audit logs the action by the user at current time
			cache updates from the database on successful validation
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]
	actions := scenario.Actions

	if len(actions) != 5 {
		t.Errorf("Expected 5 actions, got %d", len(actions))
		return
	}

	expectedPhrases := []string{
		"data for accuracy",
		"record in the table",
		"email to the user with confirmation",
		"action by the user at current time",
		"the database on successful validation",
	}

	for i, action := range actions {
		if action.Phrase != expectedPhrases[i] {
			t.Errorf("Action %d: Expected phrase '%s', got '%s'", i, expectedPhrases[i], action.Phrase)
		}
	}
}

func TestParser_SyncActionVariations(t *testing.T) {
	dsl := `use_case "Sync Action Variations" {
		when user initiates process
			ServiceA asks ServiceB to process request
			ServiceC asks ServiceD validate data
			ServiceE asks ServiceF store information safely
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]
	actions := scenario.Actions

	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
		return
	}

	// All should be sync actions
	for i, action := range actions {
		if action.Type != ActionTypeSync {
			t.Errorf("Action %d: Expected sync action, got %s", i, action.Type)
		}
	}

	// First action: with "to" connector
	if actions[0].Connector != "to" {
		t.Errorf("Action 0: Expected connector 'to', got '%s'", actions[0].Connector)
	}

	if actions[0].Phrase != "process request" {
		t.Errorf("Action 0: Expected phrase 'process request', got '%s'", actions[0].Phrase)
	}

	// Second action: no connector (direct phrase)
	if actions[1].Connector != "" {
		t.Errorf("Action 1: Expected no connector, got '%s'", actions[1].Connector)
	}

	if actions[1].Phrase != "validate data" {
		t.Errorf("Action 1: Expected phrase 'validate data', got '%s'", actions[1].Phrase)
	}

	// Third action: connector word within phrase
	if actions[2].Phrase != "store information safely" {
		t.Errorf("Action 2: Expected phrase 'store information safely', got '%s'", actions[2].Phrase)
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

// =============================================================================
// Domain Definition Tests
// =============================================================================

// Test single domain definition
func TestParser_SingleDomainDefinition(t *testing.T) {
	dsl := `domain ECommerce {
		User
		Product
		Order
		Payment
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate domain structure
	if len(model.Domains) != 1 {
		t.Errorf("Expected 1 domain, got %d", len(model.Domains))
	}

	domain := model.Domains[0]
	if domain.Name != "ECommerce" {
		t.Errorf("Expected domain name 'ECommerce', got '%s'", domain.Name)
	}

	expectedSubDomains := []string{"User", "Product", "Order", "Payment"}
	if len(domain.SubDomains) != len(expectedSubDomains) {
		t.Errorf("Expected %d subdomains, got %d", len(expectedSubDomains), len(domain.SubDomains))
	}

	// Check that all expected subdomains are present (order may vary due to map iteration)
	subdomainMap := make(map[string]bool)
	for _, subDomain := range domain.SubDomains {
		subdomainMap[subDomain] = true
	}
	for _, expectedSubDomain := range expectedSubDomains {
		if !subdomainMap[expectedSubDomain] {
			t.Errorf("Expected subdomain '%s' not found", expectedSubDomain)
		}
	}
}

// Test multiple domains definition
func TestParser_MultipleDomainDefinition(t *testing.T) {
	dsl := `domains {
		ECommerce {
			User
			Product
			Order
		}

		Analytics {
			Reporting
			Metrics
			Dashboard
		}

		Security {
			Authentication
			Authorization
		}
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate domains structure
	if len(model.Domains) != 3 {
		t.Errorf("Expected 3 domains, got %d", len(model.Domains))
	}

	expectedDomains := []struct {
		name       string
		subdomains []string
	}{
		{"ECommerce", []string{"User", "Product", "Order"}},
		{"Analytics", []string{"Reporting", "Metrics", "Dashboard"}},
		{"Security", []string{"Authentication", "Authorization"}},
	}

	for i, expected := range expectedDomains {
		domain := model.Domains[i]
		if domain.Name != expected.name {
			t.Errorf("Expected domain name '%s', got '%s'", expected.name, domain.Name)
		}

		if len(domain.SubDomains) != len(expected.subdomains) {
			t.Errorf("Expected %d subdomains for domain '%s', got %d",
				len(expected.subdomains), expected.name, len(domain.SubDomains))
		}

		// Check that all expected subdomains are present (order may vary due to map iteration)
		subdomainMap := make(map[string]bool)
		for _, subDomain := range domain.SubDomains {
			subdomainMap[subDomain] = true
		}
		for _, expectedSubdomain := range expected.subdomains {
			if !subdomainMap[expectedSubdomain] {
				t.Errorf("Expected subdomain '%s' for domain '%s' not found", expectedSubdomain, expected.name)
			}
		}
	}
}

// Test mixed single and multiple domains
func TestParser_MixedDomainDefinitions(t *testing.T) {
	dsl := `domain SingleDomain {
		SubDomain1
		SubDomain2
	}

	domains {
		MultipleDomain1 {
			Sub1
			Sub2
			Sub3
		}

		MultipleDomain2 {
			SubA
			SubB
		}
	}

	domain AnotherSingleDomain {
		OnlyOne
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 4 total domains
	if len(model.Domains) != 4 {
		t.Errorf("Expected 4 domains, got %d", len(model.Domains))
	}

	expectedDomainNames := []string{
		"SingleDomain", "MultipleDomain1", "MultipleDomain2", "AnotherSingleDomain",
	}

	for i, domain := range model.Domains {
		if domain.Name != expectedDomainNames[i] {
			t.Errorf("Expected domain name '%s', got '%s'", expectedDomainNames[i], domain.Name)
		}
	}

	// Validate specific domains
	singleDomain := model.Domains[0]
	if len(singleDomain.SubDomains) != 2 {
		t.Errorf("Expected 2 subdomains for SingleDomain, got %d", len(singleDomain.SubDomains))
	}

	multipleDomain1 := model.Domains[1]
	if len(multipleDomain1.SubDomains) != 3 {
		t.Errorf("Expected 3 subdomains for MultipleDomain1, got %d", len(multipleDomain1.SubDomains))
	}

	anotherSingle := model.Domains[3]
	if len(anotherSingle.SubDomains) != 1 || anotherSingle.SubDomains[0] != "OnlyOne" {
		t.Errorf("Expected 1 subdomain 'OnlyOne' for AnotherSingleDomain, got %v", anotherSingle.SubDomains)
	}
}

// Test domains with use cases
func TestParser_DomainsWithUseCases(t *testing.T) {
	dsl := `domain ECommerce {
		User
		Product
		Order
	}

	use_case "Product Purchase" {
		when customer buys product
			User validates customer
			Product checks availability
			Order processes purchase
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate both domains and use cases are present
	if len(model.Domains) != 1 {
		t.Errorf("Expected 1 domain, got %d", len(model.Domains))
	}

	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	// Validate domain
	domain := model.Domains[0]
	if domain.Name != "ECommerce" {
		t.Errorf("Expected domain name 'ECommerce', got '%s'", domain.Name)
	}

	expectedSubDomains := []string{"User", "Product", "Order"}
	if len(domain.SubDomains) != len(expectedSubDomains) {
		t.Errorf("Expected %d subdomains, got %d", len(expectedSubDomains), len(domain.SubDomains))
	}

	// Validate use case
	useCase := model.UseCases[0]
	if useCase.Name != "Product Purchase" {
		t.Errorf("Expected use case name 'Product Purchase', got '%s'", useCase.Name)
	}

	// Validate that use case actions reference subdomains from the domain
	scenario := useCase.Scenarios[0]
	if len(scenario.Actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(scenario.Actions))
	}

	expectedActionDomains := []string{"User", "Product", "Order"}
	for i, action := range scenario.Actions {
		if action.Domain != expectedActionDomains[i] {
			t.Errorf("Expected action domain '%s', got '%s'", expectedActionDomains[i], action.Domain)
		}
	}
}

// Test domains with services
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

// Test complete DSL with all features including domains
func TestParser_CompleteDSLWithDomains(t *testing.T) {
	dsl := `domains {
		ECommerce {
			User
			Product
			Order
			Payment
		}

		Analytics {
			Reporting
			Metrics
		}
	}

	arch ECommerceArch {
		presentation:
			WebApp[spa] > CDN[cache]
		gateway:
			APIGateway[auth:jwt] > LoadBalancer
	}

	services {
		UserService {
			domains: User
			data-stores: user_db
			language: golang
			deployment: canary(20% -> staging, 80% -> production)
		}
		ProductService {
			domains: Product
			data-stores: product_db, product_cache
			language: java
		}
	}

	exposure PublicAPI {
		to: external_clients
		of: UserService, ProductService
		through: APIGateway
	}

	use_case "User Registration" {
		when user creates account
			User validates user data
			User notifies "User Created"
	}

	use_case "Product Search" {
		when customer searches products
			Product asks Analytics to log search
			Product returns search results
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate all sections are present
	if len(model.Domains) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(model.Domains))
	}

	if len(model.Architectures) != 1 {
		t.Errorf("Expected 1 architecture, got %d", len(model.Architectures))
	}

	if len(model.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(model.Services))
	}

	if len(model.Exposures) != 1 {
		t.Errorf("Expected 1 exposure, got %d", len(model.Exposures))
	}

	if len(model.UseCases) != 2 {
		t.Errorf("Expected 2 use cases, got %d", len(model.UseCases))
	}

	// Validate domain structure
	ecommerce := model.Domains[0]
	if ecommerce.Name != "ECommerce" {
		t.Errorf("Expected domain name 'ECommerce', got '%s'", ecommerce.Name)
	}

	expectedECommerceSubDomains := []string{"User", "Product", "Order", "Payment"}
	if len(ecommerce.SubDomains) != len(expectedECommerceSubDomains) {
		t.Errorf("Expected %d subdomains for ECommerce, got %d",
			len(expectedECommerceSubDomains), len(ecommerce.SubDomains))
	}

	// Validate use case references domains correctly
	productSearch := model.UseCases[1]
	if productSearch.Name != "Product Search" {
		t.Errorf("Expected use case name 'Product Search', got '%s'", productSearch.Name)
	}

	scenario := productSearch.Scenarios[0]
	if len(scenario.Actions) != 2 {
		t.Errorf("Expected 2 actions in Product Search, got %d", len(scenario.Actions))
	}

	// Validate cross-domain interaction (Product asks Analytics)
	crossDomainAction := scenario.Actions[0]
	if crossDomainAction.Type != ActionTypeSync {
		t.Errorf("Expected sync action for cross-domain call, got %s", crossDomainAction.Type)
	}

	if crossDomainAction.Domain != "Product" {
		t.Errorf("Expected source domain 'Product', got '%s'", crossDomainAction.Domain)
	}

	if crossDomainAction.TargetDomain != "Analytics" {
		t.Errorf("Expected target domain 'Analytics', got '%s'", crossDomainAction.TargetDomain)
	}
}

// Test domain definition error cases
func TestParser_InvalidDomainDefinitions(t *testing.T) {
	testCases := []struct {
		name string
		dsl  string
	}{
		{
			name: "Domain without subdomains",
			dsl: `domain EmptyDomain {
			}`,
		},
		{
			name: "Domain without name",
			dsl: `domain {
				SubDomain1
			}`,
		},
		{
			name: "Domains block without content",
			dsl: `domains {
			}`,
		},
		{
			name: "Malformed domain syntax",
			dsl: `domain MalformedDomain
				SubDomain1
				SubDomain2`,
		},
		{
			name: "Mixed incorrect syntax",
			dsl: `domains {
				ValidDomain {
					Sub1
				}
				InvalidDomain
					Sub2
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := ParseDSLToModel(tc.dsl)

			if err == nil {
				t.Errorf("Expected parse error for '%s', got nil", tc.name)
			}

			if model != nil {
				t.Errorf("Expected nil model for '%s', got non-nil model", tc.name)
			}

			t.Logf("Successfully caught error for '%s': %v", tc.name, err)
		})
	}
}

// Test empty domain scenarios
func TestParser_EmptyDomainScenarios(t *testing.T) {
	// Test that empty domain sections should fail according to grammar
	dsl := `domains {
	}

	use_case "Test Case" {
		when user acts
			SomeDomain processes action
	}`

	model, err := ParseDSLToModel(dsl)

	// Empty domains block should fail - grammar requires at least one domain block
	if err == nil {
		t.Fatalf("Expected error for empty domains block, got nil")
	}

	if model != nil {
		t.Errorf("Expected nil model for empty domains block, got non-nil model")
	}
}

// Test domain naming edge cases
func TestParser_DomainNamingEdgeCases(t *testing.T) {
	dsl := `domains {
		Domain_With_Underscores {
			SubDomain_1
			SubDomain_2
		}

		Domain-With-Hyphens {
			Sub-Domain-A
			Sub-Domain-B
		}

		DomainWithNumbers123 {
			SubDomain456
			Another789Sub
		}

		SimpleA {
			B
		}
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error for domain naming variations, got: %v", err)
	}

	if len(model.Domains) != 4 {
		t.Errorf("Expected 4 domains, got %d", len(model.Domains))
	}

	expectedDomainNames := []string{
		"Domain_With_Underscores",
		"Domain-With-Hyphens",
		"DomainWithNumbers123",
		"SimpleA",
	}

	// Check that all expected domain names are present
	domainNameMap := make(map[string]bool)
	for _, domain := range model.Domains {
		domainNameMap[domain.Name] = true
	}
	for _, expectedName := range expectedDomainNames {
		if !domainNameMap[expectedName] {
			t.Errorf("Expected domain name '%s' not found", expectedName)
		}
	}

	// Validate specific subdomain patterns - find domains by name
	var underscoreDomain, hyphenDomain *Domain
	for i := range model.Domains {
		if model.Domains[i].Name == "Domain_With_Underscores" {
			underscoreDomain = &model.Domains[i]
		} else if model.Domains[i].Name == "Domain-With-Hyphens" {
			hyphenDomain = &model.Domains[i]
		}
	}

	if underscoreDomain == nil {
		t.Fatal("Domain_With_Underscores not found")
	}
	expectedUnderscoreSubs := []string{"SubDomain_1", "SubDomain_2"}
	underscoreSubMap := make(map[string]bool)
	for _, sub := range underscoreDomain.SubDomains {
		underscoreSubMap[sub] = true
	}
	for _, expectedSub := range expectedUnderscoreSubs {
		if !underscoreSubMap[expectedSub] {
			t.Errorf("Expected underscore subdomain '%s' not found", expectedSub)
		}
	}

	if hyphenDomain == nil {
		t.Fatal("Domain-With-Hyphens not found")
	}
	expectedHyphenSubs := []string{"Sub-Domain-A", "Sub-Domain-B"}
	hyphenSubMap := make(map[string]bool)
	for _, sub := range hyphenDomain.SubDomains {
		hyphenSubMap[sub] = true
	}
	for _, expectedSub := range expectedHyphenSubs {
		if !hyphenSubMap[expectedSub] {
			t.Errorf("Expected hyphen subdomain '%s' not found", expectedSub)
		}
	}
}

// Benchmark domain parsing
func BenchmarkParser_SingleDomainDefinition(b *testing.B) {
	dsl := `domain BenchmarkDomain {
		SubDomain1
		SubDomain2
		SubDomain3
		SubDomain4
		SubDomain5
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDSLToModel(dsl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser_MultipleDomainDefinition(b *testing.B) {
	dsl := `domains {
		Domain1 {
			Sub1A
			Sub1B
			Sub1C
		}

		Domain2 {
			Sub2A
			Sub2B
		}

		Domain3 {
			Sub3A
			Sub3B
			Sub3C
			Sub3D
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDSLToModel(dsl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser_ComplexDSLWithDomains(b *testing.B) {
	dsl := `domains {
		ECommerce {
			User
			Product
			Order
			Payment
			Inventory
		}

		Analytics {
			Reporting
			Metrics
			Dashboard
		}
	}

	arch ComplexArch {
		presentation:
			WebApp[spa,cache] > CDN[geo:distributed] > APIGateway[auth:oauth2]
		gateway:
			LoadBalancer[algorithm:round_robin] > ServiceMesh[tracing:jaeger]
	}

	services {
		UserService {
			domains: User, Product
			data-stores: user_db, user_cache
			language: golang
			deployment: canary(10% -> staging, 90% -> production)
		},
		OrderService {
			domains: Order, Payment, Inventory
			data-stores: order_db, payment_db
			language: java
			deployment: blue_green
		}
	}

	use_case "Complex Purchase Flow" {
		when customer purchases product
			User validates customer credentials
			Product checks availability
			Inventory reserves items
			Order creates purchase order
			Payment processes payment
			Order notifies "Order Completed"
			Analytics notifies "Purchase Tracked"
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseDSLToModel(dsl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Test duplicate domain definitions are merged
func TestParser_DuplicateDomainMerging(t *testing.T) {
	dsl := `domain Payment {
		ProcessPayment
		ValidateCard
	}

	domains {
		Payment {
			RefundPayment
			CancelPayment
		}

		User {
			CreateAccount
			UpdateProfile
		}
	}

	domain Payment {
		ChargeFee
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 2 unique domains (Payment and User)
	if len(model.Domains) != 2 {
		t.Fatalf("Expected 2 domains, got %d", len(model.Domains))
	}

	// Find Payment domain
	var paymentDomain *Domain
	var userDomain *Domain
	for i := range model.Domains {
		if model.Domains[i].Name == "Payment" {
			paymentDomain = &model.Domains[i]
		} else if model.Domains[i].Name == "User" {
			userDomain = &model.Domains[i]
		}
	}

	if paymentDomain == nil {
		t.Fatal("Payment domain not found")
	}
	if userDomain == nil {
		t.Fatal("User domain not found")
	}

	// Payment domain should have all subdomains merged (5 total)
	expectedPaymentSubs := []string{"ProcessPayment", "ValidateCard", "RefundPayment", "CancelPayment", "ChargeFee"}
	if len(paymentDomain.SubDomains) != len(expectedPaymentSubs) {
		t.Errorf("Expected %d Payment subdomains, got %d", len(expectedPaymentSubs), len(paymentDomain.SubDomains))
	}

	// Check that all expected subdomains are present (order may vary due to map iteration)
	subdomainMap := make(map[string]bool)
	for _, sub := range paymentDomain.SubDomains {
		subdomainMap[sub] = true
	}
	for _, expectedSub := range expectedPaymentSubs {
		if !subdomainMap[expectedSub] {
			t.Errorf("Expected Payment subdomain '%s' not found", expectedSub)
		}
	}

	// User domain should have 2 subdomains
	expectedUserSubs := []string{"CreateAccount", "UpdateProfile"}
	if len(userDomain.SubDomains) != len(expectedUserSubs) {
		t.Errorf("Expected %d User subdomains, got %d", len(expectedUserSubs), len(userDomain.SubDomains))
	}
}

// Test duplicate subdomains within same domain definition
func TestParser_DuplicateSubdomainMerging(t *testing.T) {
	dsl := `domain Inventory {
		AddItem
		RemoveItem
		AddItem
		UpdateItem
		RemoveItem
	}`

	model, err := ParseDSLToModel(dsl)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(model.Domains) != 1 {
		t.Fatalf("Expected 1 domain, got %d", len(model.Domains))
	}

	domain := model.Domains[0]
	if domain.Name != "Inventory" {
		t.Errorf("Expected domain name 'Inventory', got '%s'", domain.Name)
	}

	// Should have 3 unique subdomains (duplicates removed)
	expectedSubs := []string{"AddItem", "RemoveItem", "UpdateItem"}
	if len(domain.SubDomains) != len(expectedSubs) {
		t.Errorf("Expected %d unique subdomains, got %d", len(expectedSubs), len(domain.SubDomains))
	}

	// Check that all expected subdomains are present
	subdomainMap := make(map[string]bool)
	for _, sub := range domain.SubDomains {
		subdomainMap[sub] = true
	}
	for _, expectedSub := range expectedSubs {
		if !subdomainMap[expectedSub] {
			t.Errorf("Expected subdomain '%s' not found", expectedSub)
		}
	}
}

func TestParser_IndividualActor(t *testing.T) {
	dsl := `actor user Customer_Support`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate actors
	if len(model.Actors) != 1 {
		t.Errorf("Expected 1 actor, got %d", len(model.Actors))
	}

	actor := model.Actors[0]
	if actor.Name != "Customer_Support" {
		t.Errorf("Expected actor name 'Customer_Support', got '%s'", actor.Name)
	}

	if actor.Type != ActorTypeUser {
		t.Errorf("Expected actor type 'user', got '%s'", actor.Type)
	}
}

func TestParser_ActorsBlock(t *testing.T) {
	dsl := `actors {
		user Business_User
		system CronA
		service Database
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate actors
	if len(model.Actors) != 3 {
		t.Errorf("Expected 3 actors, got %d", len(model.Actors))
	}

	// Check each actor
	expectedActors := map[string]ActorType{
		"Business_User": ActorTypeUser,
		"CronA":         ActorTypeSystem,
		"Database":      ActorTypeService,
	}

	actorMap := make(map[string]ActorType)
	for _, actor := range model.Actors {
		actorMap[actor.Name] = actor.Type
	}

	for expectedName, expectedType := range expectedActors {
		actualType, exists := actorMap[expectedName]
		if !exists {
			t.Errorf("Expected actor '%s' not found", expectedName)
		}
		if actualType != expectedType {
			t.Errorf("Expected actor '%s' to be type '%s', got '%s'", expectedName, expectedType, actualType)
		}
	}
}

func TestParser_MixedActorDefinitions(t *testing.T) {
	dsl := `actors {
		user Business_User
		system CronA
	}

	actor service Database`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate actors
	if len(model.Actors) != 3 {
		t.Errorf("Expected 3 actors, got %d", len(model.Actors))
	}

	// Check each actor type is parsed correctly
	expectedActors := map[string]ActorType{
		"Business_User": ActorTypeUser,
		"CronA":         ActorTypeSystem,
		"Database":      ActorTypeService,
	}

	actorMap := make(map[string]ActorType)
	for _, actor := range model.Actors {
		actorMap[actor.Name] = actor.Type
	}

	for expectedName, expectedType := range expectedActors {
		actualType, exists := actorMap[expectedName]
		if !exists {
			t.Errorf("Expected actor '%s' not found", expectedName)
		}
		if actualType != expectedType {
			t.Errorf("Expected actor '%s' to be type '%s', got '%s'", expectedName, expectedType, actualType)
		}
	}
}

func TestParser_ReturnAction(t *testing.T) {
	dsl := `use_case "Payment Processing with Returns" {
		when User submits payment request
			PaymentService validates payment data
			PaymentService asks BankGateway to process payment
			BankGateway returns payment result to PaymentService
			PaymentService returns confirmation status
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate structure
	if len(model.UseCases) != 1 {
		t.Errorf("Expected 1 use case, got %d", len(model.UseCases))
	}

	useCase := model.UseCases[0]
	if useCase.Name != "Payment Processing with Returns" {
		t.Errorf("Expected use case name 'Payment Processing with Returns', got '%s'", useCase.Name)
	}

	if len(useCase.Scenarios) != 1 {
		t.Errorf("Expected 1 scenario, got %d", len(useCase.Scenarios))
	}

	scenario := useCase.Scenarios[0]

	// Validate actions - should have 4 actions
	if len(scenario.Actions) != 4 {
		t.Errorf("Expected 4 actions, got %d", len(scenario.Actions))
	}

	// Third action - return action with target domain
	action3 := scenario.Actions[2]
	if action3.Type != ActionTypeReturn {
		t.Errorf("Expected return action, got %s", action3.Type)
	}

	if action3.Domain != "BankGateway" {
		t.Errorf("Expected domain 'BankGateway', got '%s'", action3.Domain)
	}

	if action3.Phrase != "payment result" {
		t.Errorf("Expected phrase 'payment result', got '%s'", action3.Phrase)
	}

	if action3.TargetDomain != "PaymentService" {
		t.Errorf("Expected target domain 'PaymentService', got '%s'", action3.TargetDomain)
	}

	// Fourth action - return action without target domain
	action4 := scenario.Actions[3]
	if action4.Type != ActionTypeReturn {
		t.Errorf("Expected return action, got %s", action4.Type)
	}

	if action4.Domain != "PaymentService" {
		t.Errorf("Expected domain 'PaymentService', got '%s'", action4.Domain)
	}

	if action4.Phrase != "confirmation status" {
		t.Errorf("Expected phrase 'confirmation status', got '%s'", action4.Phrase)
	}

	if action4.TargetDomain != "" {
		t.Errorf("Expected empty target domain, got '%s'", action4.TargetDomain)
	}

	// Test action descriptions
	expectedDescription3 := "BankGateway returns payment result to PaymentService"
	if action3.Description != expectedDescription3 {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription3, action3.Description)
	}

	expectedDescription4 := "PaymentService returns confirmation status"
	if action4.Description != expectedDescription4 {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription4, action4.Description)
	}

	fmt.Printf("Test passed: TestParser_ReturnAction\n")
}

func TestParser_ReturnActionWithConnector(t *testing.T) {
	dsl := `use_case "Return with Connector" {
		when User requests data
			DataService returns user information to User
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	scenario := model.UseCases[0].Scenarios[0]
	action := scenario.Actions[0]

	if action.Type != ActionTypeReturn {
		t.Errorf("Expected return action, got %s", action.Type)
	}

	if action.Domain != "DataService" {
		t.Errorf("Expected domain 'DataService', got '%s'", action.Domain)
	}

	if action.Phrase != "user information" {
		t.Errorf("Expected phrase 'user information', got '%s'", action.Phrase)
	}

	if action.Connector != "to" {
		t.Errorf("Expected connector 'to', got '%s'", action.Connector)
	}

	if action.TargetDomain != "User" {
		t.Errorf("Expected target domain 'User', got '%s'", action.TargetDomain)
	}

	expectedDescription := "DataService returns user information to User"
	if action.Description != expectedDescription {
		t.Errorf("Expected description '%s', got '%s'", expectedDescription, action.Description)
	}

	fmt.Printf("Test passed: TestParser_ReturnActionWithConnector\n")
}

func TestParser_ReturnActionCallStack(t *testing.T) {
	dsl := `use_case "Payment Flow with Call Stack" {
		when User submits payment
			PaymentService validates data
			PaymentService asks BankGateway to process payment
			BankGateway returns payment result
			PaymentService returns confirmation
	}`

	parser := NewParser()
	model, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Validate that actions are parsed correctly
	scenario := model.UseCases[0].Scenarios[0]
	if len(scenario.Actions) != 4 {
		t.Errorf("Expected 4 actions, got %d", len(scenario.Actions))
	}

	// Check that the third action (BankGateway returns payment result) has no explicit target
	action3 := scenario.Actions[2]
	if action3.Type != ActionTypeReturn {
		t.Errorf("Expected return action, got %s", action3.Type)
	}

	if action3.Domain != "BankGateway" {
		t.Errorf("Expected domain 'BankGateway', got '%s'", action3.Domain)
	}

	if action3.TargetDomain != "" {
		t.Errorf("Expected empty target domain (should use call stack), got '%s'", action3.TargetDomain)
	}

	// Check that the fourth action also has no explicit target
	action4 := scenario.Actions[3]
	if action4.Type != ActionTypeReturn {
		t.Errorf("Expected return action, got %s", action4.Type)
	}

	if action4.Domain != "PaymentService" {
		t.Errorf("Expected domain 'PaymentService', got '%s'", action4.Domain)
	}

	if action4.TargetDomain != "" {
		t.Errorf("Expected empty target domain (should use call stack), got '%s'", action4.TargetDomain)
	}

	fmt.Printf("Test passed: TestParser_ReturnActionCallStack\n")
}

func TestParser_ReturnActionDD(t *testing.T) {
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
    Authentication returns hello
    Authentication asks Database to check email uniqueness
    Profile creates user profile
    Authentication notifies "User Registered"


  when Profile listens "User Registered"
    Profile asks Database to store profile data
    Profile asks Notifier to send welcome email

}`

	parser := NewParser()
	_, err := parser.ParseString(dsl)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}
