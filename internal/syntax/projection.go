package syntax

import (
	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/pkg/craft"
)

// Project converts an *ast.File to a *craft.CraftDoc.
// The projection is the public contract; AST shapes are internal.
func Project(f *ast.File) *craft.CraftDoc {
	doc := &craft.CraftDoc{
		UseCases: []craft.UseCase{},
	}
	for _, a := range f.Actors {
		doc.Actors = append(doc.Actors, craft.Actor{
			Name: a.Name,
			Type: craft.ActorType(a.Type),
			Line: a.Line,
		})
	}
	return doc
}
