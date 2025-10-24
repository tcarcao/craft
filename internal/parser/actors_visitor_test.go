package parser

import (	"testing"
)

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

