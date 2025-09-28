package parser

import (
	"github.com/tcarcao/craft/pkg/parser"
)

// VisitActor_def handles individual actor definitions: actor user Name
func (b *DSLModelBuilder) VisitActor_def(ctx *parser.Actor_defContext) interface{} {
	actorType := b.extractActorType(ctx.ActorType().(*parser.ActorTypeContext))
	actorName := b.extractActorName(ctx.Actor_name())

	actor := Actor{
		Name: actorName,
		Type: actorType,
	}

	b.model.Actors = append(b.model.Actors, actor)
	return nil
}

// VisitActors_def handles actors block definitions: actors { ... }
func (b *DSLModelBuilder) VisitActors_def(ctx *parser.Actors_defContext) interface{} {
	if actorsList := ctx.Actor_definition_list(); actorsList != nil {
		b.VisitActor_definition_list(actorsList.(*parser.Actor_definition_listContext))
	}
	return nil
}

// VisitActor_definition_list processes the list of actor definitions in actors block
func (b *DSLModelBuilder) VisitActor_definition_list(ctx *parser.Actor_definition_listContext) interface{} {
	for _, actorDef := range ctx.AllActor_definition() {
		b.VisitActor_definition(actorDef.(*parser.Actor_definitionContext))
	}
	return nil
}

// VisitActor_definition processes a single actor definition within actors block: user Name
func (b *DSLModelBuilder) VisitActor_definition(ctx *parser.Actor_definitionContext) interface{} {
	actorType := b.extractActorType(ctx.ActorType().(*parser.ActorTypeContext))
	actorName := b.extractActorName(ctx.Actor_name())

	actor := Actor{
		Name: actorName,
		Type: actorType,
	}

	b.model.Actors = append(b.model.Actors, actor)
	return nil
}

// extractActorType extracts and validates the actor type from the ActorType context
func (b *DSLModelBuilder) extractActorType(ctx *parser.ActorTypeContext) ActorType {
	if ctx == nil {
		return ActorTypeUser // default
	}

	actorTypeText := ctx.GetText()
	switch actorTypeText {
	case "user":
		return ActorTypeUser
	case "system":
		return ActorTypeSystem
	case "service":
		return ActorTypeService
	default:
		// For now, default to user type if unrecognized
		// In the future, we could add validation warnings here
		return ActorTypeUser
	}
}

// extractActorName extracts the actor name from the actor name context
func (b *DSLModelBuilder) extractActorName(ctx parser.IActor_nameContext) string {
	if ctx == nil {
		return ""
	}
	
	// Actor name is defined as 'identifier' in the grammar
	actorNameCtx := ctx.(*parser.Actor_nameContext)
	
	// Look for the identifier child
	for i := 0; i < actorNameCtx.GetChildCount(); i++ {
		child := actorNameCtx.GetChild(i)
		if identifierCtx, ok := child.(*parser.IdentifierContext); ok {
			return identifierCtx.GetText()
		}
	}
	
	// Fallback
	return actorNameCtx.GetText()
}