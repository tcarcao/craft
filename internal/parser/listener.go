package parser

import (
	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

type architectureListener struct {
	*parser.BaseArchDSLListener

	architecture   *Architecture
	currentSystem  *System
	currentContext *Context
	currentFlow    *Flow
}

func newArchitectureListener() *architectureListener {
	return &architectureListener{
		architecture: &Architecture{},
	}
}

func (l *architectureListener) EnterSystem(ctx *parser.SystemContext) {
	system := &System{
		Name: ctx.IDENT().GetText(),
	}
	l.architecture.Systems = append(l.architecture.Systems, system)
	l.currentSystem = system
}

func (l *architectureListener) ExitSystem(ctx *parser.SystemContext) {
	l.currentSystem = nil
}

func (l *architectureListener) EnterContext(ctx *parser.ContextContext) {
	context := &Context{
		Name: ctx.IDENT().GetText(),
	}
	l.currentSystem.Contexts = append(l.currentSystem.Contexts, context)
	l.currentContext = context
}

func (l *architectureListener) ExitContext(ctx *parser.ContextContext) {
	l.currentContext = nil
}

func (l *architectureListener) EnterAggregate(ctx *parser.AggregateContext) {
	l.currentContext.Aggregates = append(l.currentContext.Aggregates, ctx.IDENT().GetText())
}

func (l *architectureListener) EnterComponent(ctx *parser.ComponentContext) {
	component := &Component{
		Name: ctx.IDENT().GetText(),
	}
	if ctx.Tech() != nil {
		component.Tech = &Technology{
			Language: ctx.Tech().TECH().GetText(),
		}
	}
	l.currentContext.Components = append(l.currentContext.Components, component)
}

func (l *architectureListener) EnterService(ctx *parser.ServiceContext) {
	service := &Service{
		Name: ctx.IDENT().GetText(),
	}
	if ctx.Tech() != nil {
		service.Tech = &Technology{
			Language: ctx.Tech().TECH().GetText(),
		}
	}
	if ctx.Platform() != nil {
		service.Platform = ctx.Platform().PLATFORM().GetText()
	}
	l.currentContext.Services = append(l.currentContext.Services, service)
}

func (l *architectureListener) EnterEvent(ctx *parser.EventContext) {
	l.currentContext.Events = append(l.currentContext.Events, ctx.IDENT().GetText())
}

func (l *architectureListener) EnterRelation(ctx *parser.RelationContext) {
	relation := &Relation{
		Type:    ctx.GetChild(0).(antlr.TerminalNode).GetText(), // upstream or downstream
		Target:  ctx.IDENT().GetText(),
		Pattern: ctx.Pattern().GetText(),
	}
	l.currentContext.Relations = append(l.currentContext.Relations, relation)
}

func (l *architectureListener) EnterFlow(ctx *parser.FlowContext) {
	flow := &Flow{
		Source:    ctx.IDENT(0).GetText(),
		Operation: ctx.IDENT(1).GetText(),
	}

	if ctx.Args() != nil {
		for _, arg := range ctx.Args().AllIDENT() {
			flow.Args = append(flow.Args, arg.GetText())
		}
	}

	if ctx.Target() != nil {
		flow.Target = &FlowTarget{
			Context:   ctx.Target().IDENT(0).GetText(),
			Operation: ctx.Target().IDENT(1).GetText(),
		}
	}

	l.architecture.Flows = append(l.architecture.Flows, flow)
}

func (l *architectureListener) getArchitecture() *Architecture {
	return l.architecture
}
