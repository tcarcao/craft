package sema

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/model"
)

// ResolveParticipant resolves a name written in a participant slot — a
// use-case subject or target, a context_map endpoint, a service's contexts
// entry — against the whole workspace. It is the only resolver for that
// grammar production, so every consumer agrees on what a name means.
//
// scopeDomain is the domain of the enclosing block, or "" when there is none
// (use cases have none). The order, first match wins:
//
//  1. qualified "<domain>/<bc>" names that bounded context, or nothing;
//  2. a bounded context of scopeDomain;
//  3. a bounded context declared in exactly one domain;
//  4. a bounded context declared in two or more domains is ambiguous: the
//     result is the zero Participant with ambiguous set, never a guess;
//  5. a domain, then a service, then an actor, then a data store.
//
// The zero Participant (Kind "") means the name does not resolve.
func ResolveParticipant(ws WorkspaceSymbols, scopeDomain, ref string) (p model.Participant, ambiguous bool) {
	if idx := strings.IndexByte(ref, '/'); idx >= 0 {
		domain, name := ref[:idx], ref[idx+1:]
		if dom, ok := ws.Domains[domain]; ok && domainHasBC(dom, name) {
			return ws.boundedContext(domain, name), false
		}
		return model.Participant{}, false
	}

	if scopeDomain != "" {
		if dom, ok := ws.Domains[scopeDomain]; ok && domainHasBC(dom, ref) {
			return ws.boundedContext(scopeDomain, ref), false
		}
	}

	// Count owners across all domains: ws.BoundedContexts maps each name to a
	// single owner and would hide the ambiguity.
	owner, count := "", 0
	for domName, dom := range ws.Domains {
		if domainHasBC(dom, ref) {
			if count == 0 || domName < owner {
				owner = domName
			}
			count++
		}
	}
	switch {
	case count == 1:
		return ws.boundedContext(owner, ref), false
	case count >= 2:
		return model.Participant{}, true
	}

	if _, ok := ws.Domains[ref]; ok {
		return model.Participant{Kind: model.ParticipantDomain, ID: ref}, false
	}
	if _, ok := ws.Services[ref]; ok {
		return model.Participant{Kind: model.ParticipantService, ID: ref, Service: ref}, false
	}
	if _, ok := ws.Actors[ref]; ok {
		return model.Participant{Kind: model.ParticipantActor, ID: ref}, false
	}
	if svc, ok := ws.DataStores[ref]; ok {
		return model.Participant{Kind: model.ParticipantDataStore, ID: ref, Service: svc}, false
	}
	return model.Participant{}, false
}

func (ws WorkspaceSymbols) boundedContext(domain, name string) model.Participant {
	id := domain + "/" + name
	return model.Participant{Kind: model.ParticipantBoundedContext, ID: id, Service: ws.ContextOwners[id]}
}

// resolveBCRef is ResolveParticipant in the vocabulary of the context_map and
// glossary checks: kind is "bc", "domain", "service", "actor", or "" when the
// name does not resolve (a data store is not a valid endpoint, so it reads as
// unresolved), and resolved is the canonical name.
func resolveBCRef(ws WorkspaceSymbols, scopeDomain, ref string) (resolved, kind string, ambiguous bool) {
	p, ambiguous := ResolveParticipant(ws, scopeDomain, ref)
	if ambiguous {
		return "", "bc", true
	}
	switch p.Kind {
	case model.ParticipantBoundedContext:
		return p.ID, "bc", false
	case model.ParticipantDomain, model.ParticipantService, model.ParticipantActor:
		return p.ID, string(p.Kind), false
	}
	return "", "", false
}

// useCaseRefTarget converts a resolved participant into the symbol the LSP
// navigates to. A data store has no declaration of its own to jump to.
func (ws WorkspaceSymbols) useCaseRefTarget(p model.Participant) (UseCaseRefTarget, bool) {
	switch p.Kind {
	case model.ParticipantActor:
		a := ws.Actors[p.ID]
		return UseCaseRefTarget{Kind: "actor", Actor: &a}, true
	case model.ParticipantDomain:
		d := ws.Domains[p.ID]
		return UseCaseRefTarget{Kind: "domain", Domain: &d}, true
	case model.ParticipantService:
		s := ws.Services[p.ID]
		return UseCaseRefTarget{Kind: "service", Service: &s}, true
	case model.ParticipantBoundedContext:
		domain, name, _ := strings.Cut(p.ID, "/")
		d := ws.Domains[domain]
		return ws.bcTarget(d, name), true
	}
	return UseCaseRefTarget{}, false
}

func (ws WorkspaceSymbols) bcTarget(d DomainSymbol, name string) UseCaseRefTarget {
	t := UseCaseRefTarget{Kind: "bounded_context", Domain: &d}
	// Positions are indexed by bare BC name.
	if pos, ok := ws.BCPositions[name]; ok {
		t.BCLine = pos.Line
		t.BCColumn = pos.Column
		t.BCURI = pos.URI
	}
	return t
}

// indexServiceMembers records which service deploys each bounded context and
// owns each data store, visiting files in sorted order. A service declared in
// several files counts only where it is first declared, matching ws.Services.
// A bounded context listed by two services keeps the first and reports the
// second: a bounded context belongs to exactly one service.
func indexServiceMembers(ws *WorkspaceSymbols, perFile map[string]Symbols, uris []string) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range uris {
		for _, s := range perFile[uri].Services {
			if ws.Services[s.Name].URI != uri {
				continue
			}
			for _, store := range s.DataStores {
				if _, dup := ws.DataStores[store]; !dup {
					ws.DataStores[store] = s.Name
				}
			}
			for _, ctx := range s.Contexts {
				p, _ := ResolveParticipant(*ws, "", ctx)
				if p.Kind != model.ParticipantBoundedContext {
					continue
				}
				prev, dup := ws.ContextOwners[p.ID]
				if !dup {
					ws.ContextOwners[p.ID] = s.Name
					continue
				}
				if prev == s.Name {
					continue
				}
				startChar := colToLSP(s.Column)
				diags = append(diags, model.Diagnostic{
					Code: "craft/sema/bc-in-multiple-services",
					Message: fmt.Sprintf(
						"bounded context %q is already deployed by service %q; a bounded context belongs to one service, so %q is ignored here",
						p.ID, prev, s.Name,
					),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(s.Line), Character: startChar},
						End:   model.Position{Line: lineToLSP(s.Line), Character: startChar + lspLen(s.Name)},
					},
				})
			}
		}
	}
	return diags
}
