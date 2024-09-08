package parser

import (
	"github.com/tcarcao/archdsl/pkg/parser"
)

type architectureListener struct {
	*parser.BaseArchDSLListener

	architecture   *Architecture
	currentSystem  *System
	currentContext *Context
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
	l.currentContext.Components = append(l.currentContext.Components, component)
}

func (l *architectureListener) EnterService(ctx *parser.ServiceContext) {
	service := &Service{
		Name: ctx.IDENT().GetText(),
	}
	l.currentContext.Services = append(l.currentContext.Services, service)
}

func (l *architectureListener) getArchitecture() *Architecture {
	return l.architecture
}
