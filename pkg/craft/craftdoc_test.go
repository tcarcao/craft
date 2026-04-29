package craft_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

func TestCraftDocRoundTrip(t *testing.T) {
	original := craft.CraftDoc{
		Actors: []craft.Actor{
			{Name: "customer", Type: craft.ActorTypeUser, Line: 1},
		},
		Domains: []craft.Domain{
			{Name: "ordering", BoundedContexts: []string{"cart", "checkout"}},
		},
		Services: []craft.Service{
			{
				Name:       "order-service",
				Contexts:   []string{"checkout"},
				DataStores: []string{"postgres"},
				Language:   "go",
				Deployment: craft.DeploymentStrategy{
					Type: "rolling",
					Rules: []craft.DeploymentRule{
						{Percentage: "100%", Target: "production"},
					},
				},
				Line: 10,
			},
		},
		UseCases: []craft.UseCase{
			{
				Name: "PlaceOrder",
				Scenarios: []craft.Scenario{
					{
						ID: "happy-path",
						Trigger: craft.Trigger{
							Type:        craft.TriggerTypeExternal,
							Actor:       "customer",
							Verb:        "submits",
							Phrase:      "an order",
							Description: "Customer submits a new order",
						},
						Actions: []craft.Action{
							{
								ID:          "validate-cart",
								Type:        craft.ActionTypeSync,
								Context: "ordering",
								Verb:        "validates",
								Description: "Validate cart contents",
								Line:        20,
							},
							{
								ID:          "emit-order-placed",
								Type:        craft.ActionTypeAsync,
								Context: "ordering",
								Event:       "OrderPlaced",
								Description: "Emit OrderPlaced event",
								Line:        21,
							},
						},
					},
				},
			},
		},
		Architectures: []craft.ArchBlock{
			{
				Name: "main",
				Presentation: []craft.Component{
					{
						Name: "api-gateway",
						Type: craft.ComponentTypeSimple,
						Modifiers: []craft.ComponentModifier{
							{Key: "auth", Value: "jwt"},
						},
					},
				},
				Gateway: []craft.Component{
					{
						Name: "order-handler",
						Type: craft.ComponentTypeFlow,
						Chain: []craft.Component{
							{Name: "validator", Type: craft.ComponentTypeSimple},
						},
					},
				},
			},
		},
		Exposures: []craft.Exposure{
			{
				Name:     "ordering-api",
				To:       []string{"customer"},
				Contexts: []string{"checkout"},
				Through:  []string{"REST"},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped craft.CraftDoc
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(original, roundTripped) {
		t.Errorf("round-trip mismatch:\n  original:     %+v\n  round-tripped: %+v", original, roundTripped)
	}
}

func TestProjectFromTree_ParityWithProject(t *testing.T) {
	fixtures := []string{
		`actor user Alice`,
		`actor system BackendAPI`,
		`actors { user Alice  system Bob }`,
		`domain Ordering { Cart Checkout }`,
		`service order-service { contexts: [Cart] }`,
		"use_case \"Pay\" { when Customer submits PaymentForm\n    PaymentService asks Bank }",
	}
	for _, src := range fixtures {
		src := src
		t.Run(src[:min(len(src), 40)], func(t *testing.T) {
			astFile, _ := syntax.Parse(src)
			legacyDoc := syntax.Project(astFile)

			tree, _, _ := syntax.ParseTree(src)
			newDoc := syntax.ProjectFromTree(tree)

			legacyJSON, _ := json.Marshal(legacyDoc)
			newJSON, _ := json.Marshal(newDoc)
			if string(legacyJSON) != string(newJSON) {
				t.Errorf("parity failure:\nlegacy: %s\nnew:    %s", legacyJSON, newJSON)
			}
		})
	}
}
