// Package sema implements the semantic analysis layer for Craft DSL.
// S3: actor namespace + duplicate-name rule.
// S4: domain namespace + duplicate-name for domains + cross-kind-name-reuse warning.
package sema

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/pkg/craft"
)


// ActorSymbol holds collected information about an actor declaration.
type ActorSymbol struct {
	Name string
	Type ast.ActorType
	// Line is the 1-based source line of the actor name.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// DomainSymbol holds collected information about a domain declaration.
type DomainSymbol struct {
	Name string
	// Line is the 1-based source line of the domain name.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// Symbols is the output of the symbol-collection pass for a single file.
type Symbols struct {
	Actors  []ActorSymbol
	Domains []DomainSymbol
}

// ResolutionMap is the output of the name-resolution pass.
// S3: not yet populated — exists so S5 can extend it without retrofitting.
type ResolutionMap struct {
	// Future slices add reference → declaration mappings here.
}

// AnalyzeFile collects symbols from a single file's AST and runs validation
// rules for constructs present through S4 (actors + domains).
// Returns the symbol table and any semantic diagnostics.
func AnalyzeFile(uri string, f *ast.File) (syms Symbols, diags []craft.Diagnostic) {
	defer func() {
		if r := recover(); r != nil {
			// Sema-tier panic recovery (Q17): log the panic with structured fields
			// so it surfaces in $/logTrace during dogfooding, then emit a stable
			// diagnostic code so the client knows sema analysis was skipped.
			slog.Error("craft sema: panic recovered",
				"tier", "sema",
				"uri", uri,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/sema/sema-panic",
				Message:  fmt.Sprintf("internal sema error: %v", r),
				Severity: craft.SeverityError,
			})
		}
	}()

	// Per-kind seen maps (Q4b: per-kind namespaces).
	seenActors := make(map[string]ActorSymbol)
	seenDomains := make(map[string]DomainSymbol)

	for _, a := range f.Actors {
		sym := ActorSymbol{
			Name: a.Name,
			Type: a.Type,
			Line: a.Line,
			URI:  uri,
		}

		if prev, dup := seenActors[a.Name]; dup {
			// Individual `actor` statements have Line=0 in the AST (ANTLR compat
			// — VisitActor_def does not record line). Block-entry actors carry
			// a correct Line value.
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("actor %q already declared (first seen at line %d)", a.Name, prev.Line),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(a.Line)},
					End:   craft.Position{Line: ast.LineToLSP(a.Line), Character: len(a.Name)},
				},
			})
			continue
		}
		seenActors[a.Name] = sym
		syms.Actors = append(syms.Actors, sym)
	}

	for _, d := range f.Domains {
		sym := DomainSymbol{
			Name: d.Name,
			Line: d.Line,
			URI:  uri,
		}

		if prev, dup := seenDomains[d.Name]; dup {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("domain %q already declared (first seen at line %d)", d.Name, prev.Line),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(d.Line)},
					End:   craft.Position{Line: ast.LineToLSP(d.Line), Character: len(d.Name)},
				},
			})
			continue
		}
		seenDomains[d.Name] = sym
		syms.Domains = append(syms.Domains, sym)
	}

	// Cross-kind name reuse check (Q4b, Q23): warn when the same identifier
	// appears in two different kind namespaces (e.g. actor User + domain User).
	for name, actorSym := range seenActors {
		if domSym, clash := seenDomains[name]; clash {
			diags = append(diags, craft.Diagnostic{
				Code: "craft/sema/cross-kind-name-reuse",
				Message: fmt.Sprintf(
					"%q is declared as both an actor (line %d) and a domain (line %d); "+
						"consider renaming to avoid confusion",
					name, actorSym.Line, domSym.Line,
				),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(domSym.Line)},
					End:   craft.Position{Line: ast.LineToLSP(domSym.Line), Character: len(name)},
				},
			})
		}
	}

	return syms, diags
}

