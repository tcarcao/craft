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
  TestService: {
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
  WalletService: {
    domains: Wallet, WalletItemPurchase
    data-stores: wallet_db
  },
  "Order Service": {
    domains: OrderManagement
    data-stores: order_db
  },
  service-re-go-vas: {
    domains: VASScheduling, VASProcessing
    data-stores: vas_db, vas_cache
  },
  "complex-service name": {
    domains: ComplexDomain
    data-stores: complex_db
  },
  simple_underscore_service: {
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
  service-123-test: {
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
  service_test_123: {
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
  "Service with spaces & symbols": {
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
  A: {
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
  TestService: {
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
