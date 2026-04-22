package parser_antlr_adapter_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	internalparser "github.com/tcarcao/craft/internal/parser"
	. "github.com/tcarcao/craft/internal/parser_antlr_adapter"
)

func TestFromDSLModel_RoundTripWithANTLR(t *testing.T) {
	content, err := os.ReadFile("../../examples/user-management.craft")
	if err != nil {
		t.Fatalf("failed to read user-management.craft: %v", err)
	}

	model, err := internalparser.NewParser().ParseString(string(content))
	if err != nil {
		t.Fatalf("failed to parse user-management.craft: %v", err)
	}

	result := FromDSLModel(model)
	if result == nil {
		t.Fatal("FromDSLModel returned nil for a non-nil input")
	}

	// Assert top-level field lengths match for three non-empty fields
	if len(result.UseCases) != len(model.UseCases) {
		t.Errorf("UseCases length mismatch: got %d, want %d", len(result.UseCases), len(model.UseCases))
	}
	if len(result.Actors) != len(model.Actors) {
		t.Errorf("Actors length mismatch: got %d, want %d", len(result.Actors), len(model.Actors))
	}
	if len(result.Services) != len(model.Services) {
		t.Errorf("Services length mismatch: got %d, want %d", len(result.Services), len(model.Services))
	}

	// JSON round-trip: marshal to JSON and back, assert DeepEqual
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundTripped interface{}
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal (to interface{}) failed: %v", err)
	}

	// Re-marshal and unmarshal into the same concrete type for DeepEqual
	data2, err := json.Marshal(roundTripped)
	if err != nil {
		t.Fatalf("second json.Marshal failed: %v", err)
	}

	var result2 interface{}
	if err := json.Unmarshal(data2, &result2); err != nil {
		t.Fatalf("second json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(roundTripped, result2) {
		t.Errorf("JSON round-trip produced different documents")
	}
}

func TestFromDSLModel_NilInput(t *testing.T) {
	result := FromDSLModel(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %+v", result)
	}
}
