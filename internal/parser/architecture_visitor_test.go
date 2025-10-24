package parser

import (	"testing"
)

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

