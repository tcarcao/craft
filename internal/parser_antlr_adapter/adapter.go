package parser_antlr_adapter

import (
	"github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/pkg/craft"
)

// FromDSLModel converts the internal parser's DSLModel to the canonical CraftDoc type.
// Returns nil if src is nil.
func FromDSLModel(src *parser.DSLModel) *craft.CraftDoc {
	if src == nil {
		return nil
	}

	doc := &craft.CraftDoc{
		UseCases: make([]craft.UseCase, len(src.UseCases)),
	}

	for _, a := range src.Architectures {
		doc.Architectures = append(doc.Architectures, archBlock(a))
	}

	for _, e := range src.Exposures {
		doc.Exposures = append(doc.Exposures, exposure(e))
	}

	for _, s := range src.Services {
		doc.Services = append(doc.Services, service(s))
	}

	for i, u := range src.UseCases {
		doc.UseCases[i] = useCase(u)
	}

	for _, d := range src.Domains {
		doc.Domains = append(doc.Domains, domain(d))
	}

	for _, a := range src.Actors {
		doc.Actors = append(doc.Actors, actor(a))
	}

	return doc
}

func archBlock(a parser.Architecture) craft.ArchBlock {
	ab := craft.ArchBlock{
		Name: a.Name,
	}

	for _, c := range a.Presentation {
		ab.Presentation = append(ab.Presentation, component(c))
	}

	for _, c := range a.Gateway {
		ab.Gateway = append(ab.Gateway, component(c))
	}

	return ab
}

func component(c parser.Component) craft.Component {
	comp := craft.Component{
		Name: c.Name,
		Type: craft.ComponentType(c.Type),
	}

	for _, m := range c.Modifiers {
		comp.Modifiers = append(comp.Modifiers, componentModifier(m))
	}

	for _, ch := range c.Chain {
		comp.Chain = append(comp.Chain, component(ch))
	}

	return comp
}

func componentModifier(m parser.ComponentModifier) craft.ComponentModifier {
	return craft.ComponentModifier{
		Key:   m.Key,
		Value: m.Value,
	}
}

func exposure(e parser.Exposure) craft.Exposure {
	return craft.Exposure{
		Name:     e.Name,
		To:       e.To,
		Contexts: e.Contexts,
		Through:  e.Through,
	}
}

func service(s parser.Service) craft.Service {
	return craft.Service{
		Name:       s.Name,
		Contexts:   s.Contexts,
		DataStores: s.DataStores,
		Language:   s.Language,
		Deployment: deploymentStrategy(s.Deployment),
		Line:       s.Line,
	}
}

func deploymentStrategy(d parser.DeploymentStrategy) craft.DeploymentStrategy {
	ds := craft.DeploymentStrategy{
		Type: d.Type,
	}

	for _, r := range d.Rules {
		ds.Rules = append(ds.Rules, deploymentRule(r))
	}

	return ds
}

func deploymentRule(r parser.DeploymentRule) craft.DeploymentRule {
	return craft.DeploymentRule{
		Percentage: r.Percentage,
		Target:     r.Target,
	}
}

func useCase(u parser.UseCase) craft.UseCase {
	uc := craft.UseCase{
		Name:      u.Name,
		Scenarios: make([]craft.Scenario, len(u.Scenarios)),
	}

	for i, s := range u.Scenarios {
		uc.Scenarios[i] = scenario(s)
	}

	return uc
}

func scenario(s parser.Scenario) craft.Scenario {
	sc := craft.Scenario{
		ID:      s.ID,
		Trigger: trigger(s.Trigger),
		Actions: make([]craft.Action, len(s.Actions)),
	}

	for i, a := range s.Actions {
		sc.Actions[i] = action(a)
	}

	return sc
}

func trigger(t parser.Trigger) craft.Trigger {
	return craft.Trigger{
		Type:        craft.TriggerType(t.Type),
		Actor:       t.Actor,
		Verb:        t.Verb,
		Phrase:      t.Phrase,
		Domain:      t.Domain,
		Event:       t.Event,
		Description: t.Description,
	}
}

func action(a parser.Action) craft.Action {
	return craft.Action{
		ID:           a.ID,
		Type:         craft.ActionType(a.Type),
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

func domain(d parser.Domain) craft.Domain {
	return craft.Domain{
		Name:            d.Name,
		BoundedContexts: d.BoundedContexts,
	}
}

func actor(a parser.Actor) craft.Actor {
	return craft.Actor{
		Name: a.Name,
		Type: craft.ActorType(a.Type),
		Line: a.Line,
	}
}
