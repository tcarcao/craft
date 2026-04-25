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
	doc.Services = mergeServices(f.Services)

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

	for _, e := range f.Exposures {
		doc.Exposures = append(doc.Exposures, projectExposure(e))
	}

	return doc
}

// projectExposure converts an AST ExposureDecl to a craft.Exposure.
func projectExposure(e *ast.ExposureDecl) craft.Exposure {
	exp := craft.Exposure{
		Name:    e.Name,
		To:      e.To,
		Through: e.Through,
	}
	if len(e.Contexts) > 0 {
		exp.Contexts = e.Contexts
	}
	return exp
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

// mergeServices collapses multiple AST ServiceDecls that share the same name
// into a single craft.Service, matching the ANTLR parser's service-merger
// behaviour (internal/parser/service_merger.go). Merge rules:
//   - Contexts and DataStores are unioned (duplicates removed, first-seen order).
//   - Language and Deployment are taken from the first declaration that sets them.
//   - Line is taken from the first declaration.
func mergeServices(svcs []*ast.ServiceDecl) []craft.Service {
	type entry struct {
		svc    craft.Service
		ctxSet map[string]bool
		dsSet  map[string]bool
	}
	var order []string
	byName := map[string]*entry{}

	for _, s := range svcs {
		e, exists := byName[s.Name]
		if !exists {
			e = &entry{
				svc: craft.Service{
					Name:       s.Name,
					Deployment: craft.DeploymentStrategy{Type: s.DeploymentType},
					Line:       s.Line,
				},
				ctxSet: map[string]bool{},
				dsSet:  map[string]bool{},
			}
			byName[s.Name] = e
			order = append(order, s.Name)
		}
		for _, c := range s.Contexts {
			if !e.ctxSet[c] {
				e.ctxSet[c] = true
				e.svc.Contexts = append(e.svc.Contexts, c)
			}
		}
		for _, d := range s.DataStores {
			if !e.dsSet[d] {
				e.dsSet[d] = true
				e.svc.DataStores = append(e.svc.DataStores, d)
			}
		}
		if e.svc.Language == "" && s.Language != "" {
			e.svc.Language = s.Language
		}
		if e.svc.Deployment.Type == "" && s.DeploymentType != "" {
			e.svc.Deployment.Type = s.DeploymentType
		}
		if len(e.svc.Deployment.Rules) == 0 && len(s.DeploymentRules) > 0 {
			for _, r := range s.DeploymentRules {
				e.svc.Deployment.Rules = append(e.svc.Deployment.Rules, craft.DeploymentRule{
					Percentage: r.Percentage,
					Target:     r.Target,
				})
			}
		}
	}

	out := make([]craft.Service, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name].svc)
	}
	return out
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
