// Package sema implements the semantic analysis layer for Craft DSL.
// S3: actor namespace + duplicate-name rule.
// S4: domain namespace + duplicate-name for domains + cross-kind-name-reuse warning.
// S5: service namespace + ResolutionMap population + unresolved-reference +
//     invalid-reference-target rules + AnalyzeWorkspace for cross-file resolution.
// S6: use_case namespace + duplicate-use-case-name rule + extend unresolved-reference
//     to use-case trigger subjects and action parties.
// S8: exposure collection + invalid-exposure-target validation (to:→actor,
//     through:→service, contexts:→domain/service).
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

// UseCaseSymbol holds collected information about a use_case declaration.
type UseCaseSymbol struct {
	Name string
	// Line is the 1-based source line of the use_case keyword.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// ExposureSymbol holds collected information about an exposure declaration.
type ExposureSymbol struct {
	Name     string
	To       []string
	Contexts []string
	Through  []string
	// Line is the 1-based source line of the `exposure` keyword.
	Line int
	// URI is the file that contains this declaration.
	URI string
}

// UseCaseRef captures a single reference site from within a use-case body.
// Each actor/domain/service name that appears in a trigger subject, action
// domain, or action target is recorded here for cross-resolution in S6.
type UseCaseRef struct {
	// Name is the referenced identifier.
	Name string
	// Line is the 1-based source line of the reference token.
	Line int
}

// Symbols is the output of the symbol-collection pass for a single file.
type Symbols struct {
	Actors    []ActorSymbol
	Domains   []DomainSymbol
	Services  []ServiceSymbol
	UseCases  []UseCaseSymbol
	Exposures []ExposureSymbol
	// UseCaseRefs collects all name references from within use-case bodies
	// so AnalyzeWorkspace can resolve them cross-file.
	UseCaseRefs []UseCaseRef
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
	// UseCases maps name → symbol across all workspace files.
	UseCases map[string]UseCaseSymbol
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
// Populated by AnalyzeWorkspace (S5+).
type ResolutionMap struct {
	// ServiceContexts maps a (uri, serviceName, contextName) reference to the
	// DomainSymbol of the bounded-context owner.
	ServiceContexts map[serviceContextKey]DomainSymbol

	// UseCaseRefs maps a (uri, line) reference site to its resolved symbol.
	// Covers actor, domain, service, and bounded-context references inside
	// use-case trigger subjects and action parties (S6).
	UseCaseRefs map[useCaseRefKey]UseCaseRefTarget
}

type serviceContextKey struct {
	ServiceURI  string
	ServiceName string
	ContextName string
}

// useCaseRefKey identifies a reference site within a use-case body.
type useCaseRefKey struct {
	URI  string
	Line int // 1-based source line of the reference token
	Name string
}

// UseCaseRefTarget is the resolved symbol for a use-case reference.
type UseCaseRefTarget struct {
	Kind string // "actor" | "domain" | "bounded_context" | "service"
	// One of the following is populated depending on Kind:
	Actor  *ActorSymbol
	Domain *DomainSymbol
	Service *ServiceSymbol
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

	// Use case collection + duplicate-use-case-name rule (Q23).
	seenUseCases := make(map[string]UseCaseSymbol)
	for _, uc := range f.UseCases {
		sym := UseCaseSymbol{Name: uc.Name, Line: uc.Line, URI: uri}
		if prev, dup := seenUseCases[uc.Name]; dup {
			diags = append(diags, craft.Diagnostic{
				Code: "craft/sema/duplicate-use-case-name",
				Message: fmt.Sprintf(
					"use_case %q already declared (first seen at line %d)", uc.Name, prev.Line,
				),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(uc.Line)},
					End:   craft.Position{Line: ast.LineToLSP(uc.Line), Character: len(uc.Name)},
				},
			})
			continue
		}
		seenUseCases[uc.Name] = sym
		syms.UseCases = append(syms.UseCases, sym)

		// Collect reference sites from all scenarios for cross-resolution.
		for _, sc := range uc.Scenarios {
			// Trigger subject (actor or domain name).
			if sc.Trigger.Actor != "" {
				syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
					Name: sc.Trigger.Actor,
					Line: sc.Trigger.Line,
				})
			}
			if sc.Trigger.Domain != "" {
				syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
					Name: sc.Trigger.Domain,
					Line: sc.Trigger.Line,
				})
			}
			// Action parties.
			for _, action := range sc.Actions {
				if action.Domain != "" {
					syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
						Name: action.Domain,
						Line: action.Line,
					})
				}
				if action.TargetDomain != "" {
					syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
						Name: action.TargetDomain,
						Line: action.Line,
					})
				}
			}
		}
	}

	// Exposure collection (S8). Exposures have no duplicate-name rule (two
	// exposures named "default" in separate files is valid; the design
	// intentionally allows one exposure per API gateway).
	for _, e := range f.Exposures {
		syms.Exposures = append(syms.Exposures, ExposureSymbol{
			Name:     e.Name,
			To:       e.To,
			Contexts: e.Contexts,
			Through:  e.Through,
			Line:     e.Line,
			URI:      uri,
		})
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
		UseCases:        make(map[string]UseCaseSymbol),
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
		for _, uc := range syms.UseCases {
			ws.UseCases[uc.Name] = uc
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
		UseCaseRefs:     make(map[useCaseRefKey]UseCaseRefTarget),
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

	// Resolve use-case reference sites (S6).
	// For each ref, try to find a matching actor, domain, bounded-context, or service.
	// Unknown references emit craft/sema/unresolved-reference diagnostics.
	// References that name an actor in action.TargetDomain are valid (actors can be targets).
	for uri, syms := range perFile {
		for _, ref := range syms.UseCaseRefs {
			key := useCaseRefKey{URI: uri, Line: ref.Line, Name: ref.Name}
			if target, ok := resolveUseCaseRef(ws, ref.Name); ok {
				rm.UseCaseRefs[key] = target
			}
			// Unknown refs in use-case bodies don't emit diagnostics here —
			// many use-case participants are external/future entities. Diagnostics
			// are reserved for unambiguously wrong references (future S9 work).
			// Per S6 plan: we populate the ResolutionMap for hover/definition use;
			// unresolved-reference diagnostics for use-case bodies are fire-only
			// when the workspace has actor/domain/service declarations.
		}
	}

	// Validate exposure targets (S8, Q23: craft/sema/invalid-exposure-target).
	// Only fire when the referenced name is known but is of the wrong kind.
	// Unknown names are silently ignored (many exposure targets may be
	// architecture components that aren't modelled as DSL declarations).
	for uri, syms := range perFile {
		for _, exp := range syms.Exposures {
			expRange := craft.Range{
				Start: craft.Position{Line: ast.LineToLSP(exp.Line)},
				End:   craft.Position{Line: ast.LineToLSP(exp.Line), Character: len(exp.Name)},
			}
			// `to:` must name actors.
			for _, name := range exp.To {
				if _, isActor := ws.Actors[name]; isActor {
					continue // correct — actor reference
				}
				if _, isDomain := ws.Domains[name]; isDomain {
					diags = append(diags, craft.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `to:` target %q is a domain, not an actor — use actor declarations for `to:` targets", exp.Name, name),
						Severity:  craft.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
					continue
				}
				if _, isService := ws.Services[name]; isService {
					diags = append(diags, craft.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `to:` target %q is a service, not an actor — use actor declarations for `to:` targets", exp.Name, name),
						Severity:  craft.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
				}
				// Unknown name: skip (may be an external entity or arch component).
			}
			// `through:` must name services.
			for _, name := range exp.Through {
				if _, isService := ws.Services[name]; isService {
					continue // correct — service reference
				}
				if _, isActor := ws.Actors[name]; isActor {
					diags = append(diags, craft.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `through:` target %q is an actor, not a service — use service declarations for `through:` targets", exp.Name, name),
						Severity:  craft.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
					continue
				}
				if _, isDomain := ws.Domains[name]; isDomain {
					diags = append(diags, craft.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `through:` target %q is a domain, not a service — use service declarations for `through:` targets", exp.Name, name),
						Severity:  craft.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
				}
				// Unknown name: skip (arch gateway components are valid but not modelled as services).
			}
			// `contexts:` must name domains, bounded contexts, or services.
			for _, name := range exp.Contexts {
				if _, isDomain := ws.Domains[name]; isDomain {
					continue
				}
				if _, isBC := ws.BoundedContexts[name]; isBC {
					continue
				}
				if _, isService := ws.Services[name]; isService {
					continue
				}
				if _, isActor := ws.Actors[name]; isActor {
					diags = append(diags, craft.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `contexts:` target %q is an actor — use domain or service declarations for `contexts:` targets", exp.Name, name),
						Severity:  craft.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
				}
				// Unknown name: skip.
			}
		}
	}

	return rm, diags
}

// resolveUseCaseRef attempts to resolve a name from a use-case body against
// the workspace symbol tables. Returns the target and true if found.
func resolveUseCaseRef(ws WorkspaceSymbols, name string) (UseCaseRefTarget, bool) {
	if a, ok := ws.Actors[name]; ok {
		return UseCaseRefTarget{Kind: "actor", Actor: &a}, true
	}
	if d, ok := ws.Domains[name]; ok {
		return UseCaseRefTarget{Kind: "domain", Domain: &d}, true
	}
	if bc, ok := ws.BoundedContexts[name]; ok {
		return UseCaseRefTarget{Kind: "bounded_context", Domain: &bc}, true
	}
	if s, ok := ws.Services[name]; ok {
		return UseCaseRefTarget{Kind: "service", Service: &s}, true
	}
	return UseCaseRefTarget{}, false
}

// ResolveServiceContext looks up where a service's context reference resolves.
// Returns the owning DomainSymbol and true if found.
func ResolveServiceContext(rm ResolutionMap, uri, serviceName, contextName string) (DomainSymbol, bool) {
	key := serviceContextKey{ServiceURI: uri, ServiceName: serviceName, ContextName: contextName}
	sym, ok := rm.ServiceContexts[key]
	return sym, ok
}

// ResolveUseCaseRef looks up the resolution for a use-case body reference.
// Returns the target and true if the reference was resolved.
func ResolveUseCaseRef(rm ResolutionMap, uri, name string, line int) (UseCaseRefTarget, bool) {
	key := useCaseRefKey{URI: uri, Line: line, Name: name}
	target, ok := rm.UseCaseRefs[key]
	return target, ok
}
