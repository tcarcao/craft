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
	for _, s := range f.Services {
		svc := craft.Service{
			Name:       s.Name,
			Contexts:   s.Contexts,
			DataStores: s.DataStores,
			Language:   s.Language,
			Deployment: craft.DeploymentStrategy{},
			Line:       s.Line,
		}
		doc.Services = append(doc.Services, svc)
	}
	for _, a := range f.Actors {
		doc.Actors = append(doc.Actors, craft.Actor{
			Name: a.Name,
			Type: craft.ActorType(a.Type),
			Line: a.Line,
		})
	}
	for _, d := range f.Domains {
		contexts := d.BoundedContexts
		if contexts == nil {
			contexts = []string{}
		}
		doc.Domains = append(doc.Domains, craft.Domain{
			Name:            d.Name,
			BoundedContexts: contexts,
		})
	}
	return doc
}
