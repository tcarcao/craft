// Package ast defines the v2 hand-written parser's abstract syntax tree nodes.
// All types here are private to the parser implementation; the public contract
// is pkg/craft.CraftDoc. AST shapes change freely; CraftDoc is frozen at v0.1.
package ast

// LineToLSP converts a 1-based source line (as recorded in AST nodes) to a
// 0-based LSP line number (as required by the LSP protocol).
// Returns 0 for any non-positive input.
func LineToLSP(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
}

// File is the root AST node for a parsed .craft file.
type File struct {
	Actors   []*ActorDecl   `json:"actors,omitempty"`
	Domains  []*DomainDecl  `json:"domains,omitempty"`
	Services []*ServiceDecl `json:"services,omitempty"`
}

// ServiceDecl represents a service declaration inside a services { ... } block.
type ServiceDecl struct {
	Name       string   `json:"name"`
	Contexts   []string `json:"contexts,omitempty"`
	DataStores []string `json:"dataStores,omitempty"`
	Language   string   `json:"language,omitempty"`
	// Line is the 1-based source line where the service name appears.
	Line int `json:"line,omitempty"`
	// ContextLines holds the 1-based source line for each entry in Contexts
	// (parallel slice). Used for go-to-definition cursor matching.
	ContextLines []int `json:"contextLines,omitempty"`
}

// DomainDecl represents either an individual domain declaration
// (domain Name { BoundedContext... }) or an entry inside a domains block.
type DomainDecl struct {
	Name            string `json:"name"`
	BoundedContexts []string `json:"boundedContexts,omitempty"`
	// Line is the 1-based source line where the domain name appears.
	Line int `json:"line,omitempty"`
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
