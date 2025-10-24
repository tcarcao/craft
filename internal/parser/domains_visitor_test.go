package parser

import (	"testing"
)

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

