// Package sema implements the semantic analysis layer for Craft DSL.
// S3: symbol collection in the actor namespace + duplicate-name validation rule.
// Future slices add domain/service/use-case namespaces and cross-kind rules.
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

// Symbols is the output of the symbol-collection pass for a single file.
type Symbols struct {
	Actors []ActorSymbol
}

// ResolutionMap is the output of the name-resolution pass.
// S3: not yet populated — exists so S5 can extend it without retrofitting.
type ResolutionMap struct {
	// Future slices add reference → declaration mappings here.
}

// AnalyzeFile collects symbols from a single file's AST and runs validation
// rules for constructs present in S3 (actors only).
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

	seen := make(map[string]ActorSymbol)

	for _, a := range f.Actors {
		sym := ActorSymbol{
			Name: a.Name,
			Type: a.Type,
			Line: a.Line,
			URI:  uri,
		}

		if prev, dup := seen[a.Name]; dup {
			// Individual `actor` statements have Line=0 in the AST (ANTLR compat
			// — VisitActor_def does not record line). The range will point to
			// line 0 for such duplicates until per-token position tracking is
			// added in S4+. Block-entry actors carry a correct Line value.
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("actor %q already declared (first seen at line %d)", a.Name, prev.Line),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: lineToLSP(a.Line)},
					End:   craft.Position{Line: lineToLSP(a.Line), Character: len(a.Name)},
				},
			})
			continue
		}
		seen[a.Name] = sym
		syms.Actors = append(syms.Actors, sym)
	}

	return syms, diags
}

// lineToLSP converts a 1-based source line to a 0-based LSP line.
func lineToLSP(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
}
