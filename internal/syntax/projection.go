package syntax

import (
	"fmt"

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

	for _, uc := range f.UseCases {
		doc.UseCases = append(doc.UseCases, projectUseCase(uc))
	}

	for _, a := range f.Archs {
		doc.Architectures = append(doc.Architectures, projectArch(a))
	}

	return doc
}

// projectArch converts an AST ArchDecl to a craft.ArchBlock.
// Presentation and Gateway are always initialised as slices (never nil) to
// match the ANTLR adapter's behaviour.
func projectArch(a *ast.ArchDecl) craft.ArchBlock {
	ab := craft.ArchBlock{
		Name:         a.Name,
		Presentation: []craft.Component{},
		Gateway:      []craft.Component{},
	}
	for _, c := range a.Presentation {
		ab.Presentation = append(ab.Presentation, projectArchComponent(c))
	}
	for _, c := range a.Gateway {
		ab.Gateway = append(ab.Gateway, projectArchComponent(c))
	}
	return ab
}

// projectArchComponent converts an AST ArchComponent to a craft.Component.
func projectArchComponent(c *ast.ArchComponent) craft.Component {
	comp := craft.Component{
		Name: c.Name,
		Type: craft.ComponentType(c.Type),
	}
	for _, m := range c.Modifiers {
		comp.Modifiers = append(comp.Modifiers, craft.ComponentModifier{Key: m.Key, Value: m.Value})
	}
	for _, ch := range c.Chain {
		comp.Chain = append(comp.Chain, projectArchComponent(ch))
	}
	return comp
}

// projectUseCase converts a single AST UseCaseDecl to a craft.UseCase.
// IDs (scenario_N, action_N) are already assigned by the parser.
func projectUseCase(uc *ast.UseCaseDecl) craft.UseCase {
	out := craft.UseCase{
		Name:      uc.Name,
		Scenarios: []craft.Scenario{},
	}
	for _, sc := range uc.Scenarios {
		out.Scenarios = append(out.Scenarios, projectScenario(sc))
	}
	return out
}

// projectScenario converts a single AST ScenarioDecl to a craft.Scenario.
func projectScenario(sc *ast.ScenarioDecl) craft.Scenario {
	out := craft.Scenario{
		ID:      sc.ID,
		Trigger: projectTrigger(sc.Trigger),
		Actions: []craft.Action{},
	}
	for i, a := range sc.Actions {
		out.Actions = append(out.Actions, projectAction(a, i))
	}
	return out
}

// projectTrigger converts an AST TriggerDecl to a craft.Trigger.
func projectTrigger(t ast.TriggerDecl) craft.Trigger {
	return craft.Trigger{
		Type:        craft.TriggerType(t.TriggerType),
		Actor:       t.Actor,
		Verb:        t.Verb,
		Phrase:      t.Phrase,
		Domain:      t.Domain,
		Event:       t.Event,
		Description: t.Description,
	}
}

// projectAction converts an AST ActionDecl to a craft.Action.
// The action's ID is already embedded in a.ActionID (set by the parser).
func projectAction(a *ast.ActionDecl, _ int) craft.Action {
	return craft.Action{
		ID:           fmt.Sprintf("action_%d", a.ActionID),
		Type:         craft.ActionType(a.ActionType),
		Domain:       a.Domain,
		Verb:         a.Verb,
		TargetDomain: a.TargetDomain,
		Event:        a.Event,
		Connector:    a.Connector,
		Phrase:       a.Phrase,
		Description:  a.Description,
		Line:         a.Line,
	}
}
