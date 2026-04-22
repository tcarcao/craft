package ast_test

import (
	"encoding/json"
	"testing"

	"github.com/tcarcao/craft/internal/ast"
)

func TestFile_JSONRoundTrip(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{
			{Name: "Alice", Type: ast.ActorTypeUser, Line: 1},
			{Name: "Bob", Type: ast.ActorTypeSystem, Line: 2},
			{Name: "DB", Type: ast.ActorTypeService},
		},
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var f2 ast.File
	if err := json.Unmarshal(data, &f2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(f2.Actors) != len(f.Actors) {
		t.Fatalf("actors len: got %d want %d", len(f2.Actors), len(f.Actors))
	}
	for i, a := range f2.Actors {
		orig := f.Actors[i]
		if a.Name != orig.Name || a.Type != orig.Type || a.Line != orig.Line {
			t.Errorf("actor[%d]: got %+v want %+v", i, a, orig)
		}
	}
}
