// Package ast defines the v2 hand-written parser's abstract syntax tree nodes.
// All types here are private to the parser implementation; the public contract
// is pkg/craft.CraftDoc. AST shapes change freely; CraftDoc is frozen at v0.1.
package ast

// File is the root AST node for a parsed .craft file.
type File struct {
	Actors []*ActorDecl `json:"actors,omitempty"`
}

// ActorDecl represents either a block entry (actors { user Foo }) or an
// individual declaration (actor user Foo).
type ActorDecl struct {
	Name string    `json:"name"`
	Type ActorType `json:"type"`
	// Line is the 1-based source line where the actor name appears.
	Line int `json:"line,omitempty"`
}

// ActorType classifies an actor.
type ActorType string

const (
	ActorTypeUser    ActorType = "user"
	ActorTypeSystem  ActorType = "system"
	ActorTypeService ActorType = "service"
)
