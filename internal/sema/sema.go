// Package sema implements the semantic analysis layer for Craft DSL.
// S3: actor namespace + duplicate-name rule.
// S4: domain namespace + duplicate-name for domains + cross-kind-name-reuse warning.
// S5: service namespace + ResolutionMap population + unresolved-reference +
//     invalid-reference-target rules + AnalyzeWorkspace for cross-file resolution.
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
	// BoundedContexts lists the bounded context names within the domain.
	BoundedContexts []string
	// Line is the 1-based source line of the domain name.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// ServiceSymbol holds collected information about a service declaration.
type ServiceSymbol struct {
	Name       string
	Contexts   []string
	DataStores []string
	Language   string
	// Line is the 1-based source line of the service name.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// Symbols is the output of the symbol-collection pass for a single file.
type Symbols struct {
	Actors   []ActorSymbol
	Domains  []DomainSymbol
	Services []ServiceSymbol
}

// WorkspaceSymbols merges per-file symbol tables for cross-file resolution.
type WorkspaceSymbols struct {
	// Actors maps name → symbol across all workspace files.
	Actors map[string]ActorSymbol
	// Domains maps name → symbol across all workspace files.
	Domains map[string]DomainSymbol
	// BoundedContexts maps bounded-context name → owning DomainSymbol.
	BoundedContexts map[string]DomainSymbol
	// Services maps name → symbol across all workspace files.
	Services map[string]ServiceSymbol
}

// RefSite records a single reference site used in ResolutionMap.
type RefSite struct {
	// URI is the file containing the reference.
	URI string
	// Line is the 1-based source line of the reference.
	Line int
	// Name is the referenced identifier text.
	Name string
}

// ResolutionMap maps reference sites to their resolved declaration symbols.
// Populated by AnalyzeWorkspace (S5).
type ResolutionMap struct {
	// ServiceContexts maps a (uri, serviceName, contextName) reference to the
	// DomainSymbol of the bounded-context owner.
	ServiceContexts map[serviceContextKey]DomainSymbol
}

type serviceContextKey struct {
	ServiceURI  string
	ServiceName string
	ContextName string
}

// AnalyzeFile collects symbols from a single file's AST and runs validation
// rules for constructs present through S5 (actors + domains + services).
// Returns the symbol table and any semantic diagnostics.
func AnalyzeFile(uri string, f *ast.File) (syms Symbols, diags []craft.Diagnostic) {
	defer func() {
		if r := recover(); r != nil {
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

	seenActors := make(map[string]ActorSymbol)
	seenDomains := make(map[string]DomainSymbol)
	seenServices := make(map[string]ServiceSymbol)

	for _, a := range f.Actors {
		sym := ActorSymbol{Name: a.Name, Type: a.Type, Line: a.Line, URI: uri}
		if prev, dup := seenActors[a.Name]; dup {
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
		sym := DomainSymbol{Name: d.Name, BoundedContexts: d.BoundedContexts, Line: d.Line, URI: uri}
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

	for _, s := range f.Services {
		sym := ServiceSymbol{
			Name:       s.Name,
			Contexts:   s.Contexts,
			DataStores: s.DataStores,
			Language:   s.Language,
			Line:       s.Line,
			URI:        uri,
		}
		if prev, dup := seenServices[s.Name]; dup {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("service %q already declared (first seen at line %d)", s.Name, prev.Line),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(s.Line)},
					End:   craft.Position{Line: ast.LineToLSP(s.Line), Character: len(s.Name)},
				},
			})
			continue
		}
		seenServices[s.Name] = sym
		syms.Services = append(syms.Services, sym)
	}

	// Cross-kind name reuse check (Q4b, Q23).
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

// MergeWorkspaceSymbols merges per-file symbol tables into a workspace-level
// index. Cross-file duplicate service declarations emit a warning diagnostic
// with SourceURI set to the second file that declares the same name.
func MergeWorkspaceSymbols(perFile map[string]Symbols) (WorkspaceSymbols, []craft.Diagnostic) {
	ws := WorkspaceSymbols{
		Actors:          make(map[string]ActorSymbol),
		Domains:         make(map[string]DomainSymbol),
		BoundedContexts: make(map[string]DomainSymbol),
		Services:        make(map[string]ServiceSymbol),
	}
	var diags []craft.Diagnostic
	for _, syms := range perFile {
		for _, a := range syms.Actors {
			ws.Actors[a.Name] = a
		}
		for _, d := range syms.Domains {
			ws.Domains[d.Name] = d
			for _, bc := range d.BoundedContexts {
				ws.BoundedContexts[bc] = d
			}
		}
		for _, s := range syms.Services {
			if prev, dup := ws.Services[s.Name]; dup && prev.URI != s.URI {
				diags = append(diags, craft.Diagnostic{
					Code: "craft/sema/duplicate-name",
					Message: fmt.Sprintf(
						"service %q already declared in %s (line %d)",
						s.Name, prev.URI, prev.Line,
					),
					Severity:  craft.SeverityWarning,
					SourceURI: s.URI,
					Range: craft.Range{
						Start: craft.Position{Line: ast.LineToLSP(s.Line)},
						End:   craft.Position{Line: ast.LineToLSP(s.Line), Character: len(s.Name)},
					},
				})
				// Keep the first declaration in the map so resolution is stable.
				continue
			}
			ws.Services[s.Name] = s
		}
	}
	return ws, diags
}

// AnalyzeWorkspace runs cross-file resolution for all files. It populates a
// ResolutionMap and returns workspace-level diagnostics (unresolved references,
// etc.). The caller should first call MergeWorkspaceSymbols and pass its
// outputs; merge-phase diagnostics (cross-file duplicates) are returned there.
func AnalyzeWorkspace(perFile map[string]Symbols, ws WorkspaceSymbols) (ResolutionMap, []craft.Diagnostic) {
	rm := ResolutionMap{
		ServiceContexts: make(map[serviceContextKey]DomainSymbol),
	}
	var diags []craft.Diagnostic

	for uri, syms := range perFile {
		for _, svc := range syms.Services {
			for _, ctxName := range svc.Contexts {
				key := serviceContextKey{ServiceURI: uri, ServiceName: svc.Name, ContextName: ctxName}

				domSym, resolved := ws.BoundedContexts[ctxName]
				if !resolved {
					// Check if it's actually a domain name (not a bounded context name)
					if domByName, isDomain := ws.Domains[ctxName]; isDomain {
						// It names a domain directly — treat as valid (resolves to domain)
						rm.ServiceContexts[key] = domByName
						continue
					}
				diags = append(diags, craft.Diagnostic{
					Code: "craft/sema/unresolved-reference",
					Message: fmt.Sprintf(
						"service %q references context %q which is not declared in any domain",
						svc.Name, ctxName,
					),
					Severity:  craft.SeverityError,
					SourceURI: uri,
					Range: craft.Range{
						Start: craft.Position{Line: ast.LineToLSP(svc.Line)},
						End:   craft.Position{Line: ast.LineToLSP(svc.Line), Character: len(svc.Name)},
					},
				})
					continue
				}
				rm.ServiceContexts[key] = domSym
			}
		}
	}

	return rm, diags
}

// ResolveServiceContext looks up where a service's context reference resolves.
// Returns the owning DomainSymbol and true if found.
func ResolveServiceContext(rm ResolutionMap, uri, serviceName, contextName string) (DomainSymbol, bool) {
	key := serviceContextKey{ServiceURI: uri, ServiceName: serviceName, ContextName: contextName}
	sym, ok := rm.ServiceContexts[key]
	return sym, ok
}
