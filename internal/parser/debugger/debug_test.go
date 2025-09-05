package parser

import "testing"

// Test specifically for exposure debugging
func TestDebugExposureOnly(t *testing.T) {
	dsl := `exposure PublicAPI {
		to: external_clients, mobile_apps
		of: UserService, OrderService
		through: APIGateway, LoadBalancer
	}`

	debugParseDSL(t, dsl)
}

func TestDebugSimpleExposure(t *testing.T) {
	dsl := `exposure TestExposure {
		to: clients
	}`

	debugParseDSL(t, dsl)
}

func TestDebugArchOnly(t *testing.T) {
	dsl := `arch hello {
		presentation:
			Frontend
		gateway:
			APIGateway
	}`

	debugParseDSL(t, dsl)
}

func TestDebugMinimalExposure(t *testing.T) {
	dsl := `exposure MinimalAPI {
		to: clients
	}`

	debugParseDSL(t, dsl)
}

func TestDebugExposureWithOnlyOf(t *testing.T) {
	dsl := `exposure OnlyOfAPI {
		of: UserService
	}`

	debugParseDSL(t, dsl)
}

func TestDebugExposureWithOnlyThrough(t *testing.T) {
	dsl := `exposure OnlyThroughAPI {
		through: APIGateway
	}`

	debugParseDSL(t, dsl)
}

func TestDebugComplexExposure(t *testing.T) {
	dsl := `exposure ComplexAPI {
		to: external_clients, mobile_apps, third_party
		of: UserService, OrderService, PaymentService
		through: APIGateway, LoadBalancer, CDN
	}`

	debugParseDSL(t, dsl)
}
