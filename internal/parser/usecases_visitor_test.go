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

// TestParser_KeywordsInPhrases tests that keywords like "when" can appear in phrases
// This was a parser bug - "when" would be treated as a keyword instead of a regular word
func TestParser_KeywordsInPhrases(t *testing.T) {
	dsl := `use_case "Scheduled VAS Applied" {
		when VASMgmt identifies a scheduled VAS to apply
			VASMgmt applies a VAS and stores details until when its valid
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
	if useCase.Name != "Scheduled VAS Applied" {
		t.Errorf("Expected use case name 'Scheduled VAS Applied', got '%s'", useCase.Name)
	}

	if len(useCase.Scenarios) != 1 {
		t.Errorf("Expected 1 scenario, got %d", len(useCase.Scenarios))
	}

	scenario := useCase.Scenarios[0]

	// Validate trigger phrase contains "to" (connector word in phrase)
	if scenario.Trigger.Type != TriggerTypeExternal {
		t.Errorf("Expected external trigger, got %s", scenario.Trigger.Type)
	}

	if scenario.Trigger.Phrase != "scheduled VAS to apply" {
		t.Errorf("Expected phrase 'scheduled VAS to apply', got '%s'", scenario.Trigger.Phrase)
	}

	// Validate action phrase contains "when" (keyword in phrase) - this is the key test
	if len(scenario.Actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(scenario.Actions))
	}

	action := scenario.Actions[0]
	expectedPhrase := "VAS and stores details until when its valid"
	if action.Phrase != expectedPhrase {
		t.Errorf("Expected phrase '%s', got '%s'", expectedPhrase, action.Phrase)
	}

	// The word "when" should be part of the phrase, not cause a parse error
	if action.Verb != "applies" {
		t.Errorf("Expected verb 'applies', got '%s'", action.Verb)
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

func TestParser_ReturnAction(t *testing.T) {
	dsl := `use_case "Payment Processing with Returns" {
		when User submits payment request
			PaymentService validates payment data
			PaymentService asks BankGateway to process payment
			BankGateway returns to PaymentService payment result
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
			DataService returns to User user information
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

	if action.Connector != "" {
		t.Errorf("Expected empty connector, got '%s'", action.Connector)
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

