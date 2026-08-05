// Package sema — lint rules for Craft DSL (v2 typed views, B5).
// These rules mirror the heuristics in internal/linter/ but operate on
// the syntax tree typed views (internal/syntax/ast.go) and the workspace
// symbol table rather than the ANTLR parser.DSLModel. The old linter
// package remains intact until ANTLR is deleted in S12.
package sema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// LintWorkspace runs style and consistency checks across all workspace files.
// It accepts the per-file syntax tree map and the merged workspace symbol table.
// An optional perFileLineIndices map (uri→LineIndex) enables accurate position reporting.
// Returns diagnostics with SourceURI populated so callers can route each
// finding to the correct file.
func LintWorkspace(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, perFileLineIndices ...map[string]green.LineIndex) []model.Diagnostic {
	var lis map[string]green.LineIndex
	if len(perFileLineIndices) > 0 {
		lis = perFileLineIndices[0]
	}
	var diags []model.Diagnostic
	diags = append(diags, lintDeadEvents(perFileTrees, lis)...)
	diags = append(diags, lintUnusedActors(perFileTrees, ws)...)
	diags = append(diags, lintEventPastTense(perFileTrees, lis)...)
	diags = append(diags, lintContextMapConsistency(perFileTrees, ws, lis)...)
	return diags
}

// ── dead-event ────────────────────────────────────────────────────────────────
// Warning: an event published via `notifies` but never consumed by any
// `when … listens` or `when … "event"` trigger in any workspace file.

// eventTokenLen returns the source length of an event token: len(name)+2 for
// quoted strings (column starts at the opening `"`), len(name) for bare idents.
func eventTokenLen(name string, isString bool) int {
	if isString {
		return len(name) + 2
	}
	return len(name)
}

func lintDeadEvents(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	type pubSite struct {
		uri, event string
		line, col  int
		tokenLen   int
	}
	published := map[string]pubSite{}
	consumed := map[string]bool{}

	for uri, tree := range perFileTrees {
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				if trigger.Kind() == "event" || trigger.Kind() == "domain_listen" {
					if ev := trigger.EventValue(); ev != "" {
						consumed[ev] = true
					}
				}
				for _, action := range sc.Actions() {
					if action.Kind() == "async_action" {
						ev := action.EventValue()
						if ev != "" {
							if _, seen := published[ev]; !seen {
								published[ev] = pubSite{
									uri:      uri,
									event:    ev,
									line:     action.Line(li),
									col:      action.EventCol(li),
									tokenLen: eventTokenLen(ev, action.EventIsString()),
								}
							}
						}
					}
				}
			}
		}
	}

	var diags []model.Diagnostic
	for event, site := range published {
		if !consumed[event] {
			startChar := colToLSP(site.col)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/lint/dead-event",
				Message:  fmt.Sprintf("event %q is published but never consumed", event),
				Severity: model.SeverityWarning,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(site.line), Character: startChar},
					End:   model.Position{Line: lineToLSP(site.line), Character: startChar + site.tokenLen},
				},
				SourceURI: site.uri,
			})
		}
	}
	return diags
}

// ── unused-actor ──────────────────────────────────────────────────────────────
// Warning: an actor is declared but never appears as the subject of any
// external trigger (`when <Actor> …`) in any workspace file.

func lintUnusedActors(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols) []model.Diagnostic {
	if len(ws.Actors) == 0 {
		return nil
	}

	used := map[string]bool{}
	for _, tree := range perFileTrees {
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				switch trigger.Kind() {
				case "external":
					if actor := trigger.ActorName(); actor != "" {
						used[actor] = true
					}
				case "domain_listen":
					// An actor can be the subject of a listens trigger; the parser
					// stores the subject in Context regardless of whether it is an
					// actor or a bounded context.
					if ctx := trigger.ContextName(); ctx != "" {
						used[ctx] = true
					}
				}
			}
		}
	}

	var diags []model.Diagnostic
	for name, sym := range ws.Actors {
		if !used[name] {
			startChar := colToLSP(sym.Column)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/lint/unused-actor",
				Message:  fmt.Sprintf("actor %q is defined but never used as a trigger subject", name),
				Severity: model.SeverityWarning,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(sym.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(sym.Line), Character: startChar + lspLen(name)},
				},
				SourceURI: sym.URI,
			})
		}
	}
	return diags
}

// ── event-past-tense ──────────────────────────────────────────────────────────
// Warning: an event name (published or consumed) does not appear to use
// past tense. Mirrors internal/linter's eventPastTenseRule.

var pastTenseRe = regexp.MustCompile(`(?i)\b\w+(ed|en)\b`)

func lintEventPastTense(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	reported := map[string]bool{}
	var diags []model.Diagnostic

	check := func(event, uri string, line, col int, isString bool) {
		if event == "" || reported[event] {
			return
		}
		reported[event] = true
		if !pastTenseRe.MatchString(strings.ToLower(event)) {
			startChar := colToLSP(col)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/lint/event-not-past-tense",
				Message:  fmt.Sprintf("event %q does not appear to use past tense", event),
				Severity: model.SeverityWarning,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(line), Character: startChar},
					End:   model.Position{Line: lineToLSP(line), Character: startChar + eventTokenLen(event, isString)},
				},
				SourceURI: uri,
			})
		}
	}

	for uri, tree := range perFileTrees {
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				whenTok := sc.When()
				triggerLine := 0
				if whenTok != nil {
					triggerLine, _ = li.LineCol(whenTok.Offset())
				}
				if k := trigger.Kind(); k == "event" || k == "domain_listen" {
					check(trigger.EventValue(), uri, triggerLine, trigger.EventCol(li), trigger.EventIsString())
				}
				for _, action := range sc.Actions() {
					if action.Kind() == "async_action" {
						check(action.EventValue(), uri, action.Line(li), action.EventCol(li), action.EventIsString())
					}
				}
			}
		}
	}
	return diags
}

// ── context-map-consistency ───────────────────────────────────────────────────
// Cross-validates context_map relationships against the communication that
// actually happens between bounded contexts in use-case scenarios.
//
// Both communication primitives reduce to one directed DEPENDENCY EDGE
// `D -> U` ("downstream depends on upstream"):
//   - sync `X asks Y`            -> edge `X -> Y` (asker depends on asked)
//   - async `Y notifies E` +
//     `X listens E`              -> edge `X -> Y` (listener depends on publisher)
//
// The dependency adjacency built here is PARTIAL: it only records what the
// use-case scenarios actually say. Absence of a recorded dependency NEVER by
// itself produces a warning — only a contradicting context_map edge does.
// This structure (build adjacency, then walk context_map edges checking it)
// is designed so later rules (B2 direction-inverted/bidirectional, B3
// unclassified-communication) can extend the same adjacency without
// rebuilding it.

// pairKey is an unordered communicating BC pair, canonicalised by sorting the
// two resolved "domain/name" identities so `a < b` lexically. fwd/rev on the
// associated depInfo record which direction(s) of dependency were observed
// relative to that sorted order.
type pairKey struct {
	a, b string
}

// depInfo records the directions of dependency observed for a pairKey, plus
// the deterministic-minimum communicating-action's position (used by R3 to
// anchor the unclassified-communication diagnostic; unused by R1).
type depInfo struct {
	fwd, rev            bool
	firstLine, firstCol int
	firstLen            int
	firstURI            string
}

// newPairKey canonicalises a directed pair (from depends on to) into a sorted
// pairKey plus whether that direction runs "forward" (a->b) relative to the
// sort.
func newPairKey(from, to string) (key pairKey, fwd bool) {
	if from <= to {
		return pairKey{a: from, b: to}, true
	}
	return pairKey{a: to, b: from}, false
}

// recordDependency adds one observed directed dependency (from depends on to)
// to deps. Self-pairs and empty identities are ignored. The recorded site
// (uri/line/col) is kept as the DETERMINISTIC MINIMUM (by URI, then line,
// then col) across all observed sites for the pair — buildDependencyEdges
// iterates perFileTrees, a Go map, so encounter order is nondeterministic;
// keeping the minimum instead of the "first observed" one makes the site
// (and therefore R3's anchored diagnostic range) stable across runs.
func recordDependency(deps map[pairKey]depInfo, from, to, uri string, line, col, nameLen int) {
	if from == "" || to == "" || from == to {
		return
	}
	key, fwd := newPairKey(from, to)
	info := deps[key]
	if fwd {
		info.fwd = true
	} else {
		info.rev = true
	}
	if info.firstURI == "" || siteLess(uri, line, col, info.firstURI, info.firstLine, info.firstCol) {
		info.firstLine, info.firstCol, info.firstLen, info.firstURI = line, col, nameLen, uri
	}
	deps[key] = info
}

// siteLess reports whether site (uri1, line1, col1) sorts before site (uri2,
// line2, col2), comparing URI first, then line, then column.
func siteLess(uri1 string, line1, col1 int, uri2 string, line2, col2 int) bool {
	if uri1 != uri2 {
		return uri1 < uri2
	}
	if line1 != line2 {
		return line1 < line2
	}
	return col1 < col2
}

// asyncSite records one resolved async publisher or listener site, keyed
// externally by event value, for cross-file publisher/listener pairing.
type asyncSite struct {
	bc, uri   string
	line, col int
	nameLen   int
}

// buildDependencyEdges walks every use-case scenario in perFileTrees and
// builds the dependency adjacency described above: sync `asks` edges direct,
// async `notifies`/`listens` edges paired by equal event value across the
// whole workspace (mirroring how lintDeadEvents matches publish/consume
// workspace-wide).
func buildDependencyEdges(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) map[pairKey]depInfo {
	deps := map[pairKey]depInfo{}
	publishersByEvent := map[string][]asyncSite{}
	listenersByEvent := map[string][]asyncSite{}

	for uri, tree := range perFileTrees {
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				if trigger.Kind() == "domain_listen" {
					if ev := trigger.EventValue(); ev != "" {
						if ctx := trigger.ContextName(); ctx != "" {
							resolved, kind, ambiguous := resolveBCRef(ws, "", ctx)
							if kind == "bc" && !ambiguous && resolved != "" {
								whenTok := sc.When()
								line := 0
								if whenTok != nil {
									line, _ = li.LineCol(whenTok.Offset())
								}
								listenersByEvent[ev] = append(listenersByEvent[ev], asyncSite{
									bc: resolved, uri: uri, line: line, col: trigger.ActorCol(li), nameLen: lspLen(ctx),
								})
							}
						}
					}
				}

				for _, action := range sc.Actions() {
					switch action.Kind() {
					case "sync_action":
						subject, subjectKind, subjectAmbig := resolveBCRef(ws, "", action.SubjectName())
						target, targetKind, targetAmbig := resolveBCRef(ws, "", action.TargetName())
						if subjectKind == "bc" && !subjectAmbig && targetKind == "bc" && !targetAmbig {
							recordDependency(deps, subject, target, uri, action.Line(li), action.SubjectCol(li), lspLen(action.SubjectName()))
						}
					case "async_action":
						if ev := action.EventValue(); ev != "" {
							resolved, kind, ambiguous := resolveBCRef(ws, "", action.SubjectName())
							if kind == "bc" && !ambiguous && resolved != "" {
								publishersByEvent[ev] = append(publishersByEvent[ev], asyncSite{
									bc: resolved, uri: uri, line: action.Line(li), col: action.SubjectCol(li), nameLen: lspLen(action.SubjectName()),
								})
							}
						}
					}
				}
			}
		}
	}

	// Pair publishers and listeners of the same event across the whole
	// workspace: a listener depends on every bc that publishes that event.
	for event, listeners := range listenersByEvent {
		for _, publisher := range publishersByEvent[event] {
			for _, listener := range listeners {
				recordDependency(deps, listener.bc, publisher.bc, listener.uri, listener.line, listener.col, listener.nameLen)
			}
		}
	}

	return deps
}

// lintContextMapConsistency cross-checks context_map edges against the
// dependency adjacency built by buildDependencyEdges. R1 (this task):
// separate-ways-violation — a `separate_ways` edge whose endpoints have ANY
// observed dependency (either direction) between them is contradictory,
// since separate_ways declares that no relationship should exist. Later
// tasks (B2, B3) add further rules over the same adjacency.
func lintContextMapConsistency(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	deps := buildDependencyEdges(perFileTrees, ws, lis)

	var diags []model.Diagnostic
	for uri, tree := range perFileTrees {
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, cm := range file.ContextMaps() {
			scope := cm.Domain()
			for _, edge := range cm.Edges() {
				verb := edge.Verb()
				leftResolved, leftKind, leftAmbig := resolveBCRef(ws, scope, edge.Left())
				rightResolved, rightKind, rightAmbig := resolveBCRef(ws, scope, edge.Right())
				if leftKind != "bc" || leftAmbig || rightKind != "bc" || rightAmbig {
					continue
				}
				if leftResolved == "" || rightResolved == "" || leftResolved == rightResolved {
					continue
				}

				leftLine, leftCol := 0, 0
				if lr := edge.LeftRef(); lr != nil {
					leftLine, leftCol = lr.Line(li), lr.Col(li)
				}
				startChar := colToLSP(leftCol)
				edgeRange := model.Range{
					Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
					End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + lspLen(edge.Left())},
				}

				if verb == "separate_ways" {
					key, _ := newPairKey(leftResolved, rightResolved)
					info, hasDep := deps[key]
					if !hasDep || !(info.fwd || info.rev) {
						continue
					}
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/separate-ways-violation",
						Message:   fmt.Sprintf("%q and %q are declared separate_ways but communicate in a use case", leftResolved, rightResolved),
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range:     edgeRange,
					})
					continue
				}

				// R2a/R2b: direction-inverted / bidirectional, directional
				// verbs only (symmetric verbs like separate_ways/partnership/
				// shared_kernel have no upstream/downstream distinction).
				if _, ok := syntax.LookupEdgeVerbMeta(verb); !ok || syntax.EdgeVerbSymmetric(verb) {
					continue
				}

				key, _ := newPairKey(leftResolved, rightResolved)
				info := deps[key]
				// hasDep(from, to): was a dependency edge from->to observed
				// for this pair? Given the sorted key [a,b] (a<b): fwd means
				// a->b, rev means b->a.
				hasDep := func(from, to string) bool {
					if from < to {
						return info.fwd
					}
					return info.rev
				}

				correctPresent := hasDep(rightResolved, leftResolved)  // downstream depends on upstream
				invertedPresent := hasDep(leftResolved, rightResolved) // upstream depends on downstream

				switch {
				case invertedPresent && !correctPresent:
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/relationship-direction-inverted",
						Message:   fmt.Sprintf("%q depends on %q, but %s is upstream: dependency direction is inverted for this %s relationship", leftResolved, rightResolved, leftResolved, verb),
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range:     edgeRange,
					})
				case correctPresent && invertedPresent:
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/relationship-bidirectional",
						Message:   fmt.Sprintf("%q and %q show bidirectional traffic under an asymmetric %s pattern, consider partnership?", leftResolved, rightResolved, verb),
						Severity:  model.SeverityHint,
						SourceURI: uri,
						Range:     edgeRange,
					})
				}
			}
		}
	}

	// R3: unclassified-communication — a communicating BC pair (present in
	// the dependency adjacency) that has NO context_map edge of any verb
	// between its two endpoints anywhere in the workspace. Build the set of
	// "classified" pairs first (any edge, any verb, resolved via
	// resolveBCRef), then flag every communicating pair not in that set.
	// The adjacency already holds one entry per unordered pair, so iterating
	// it fires each pair at most once.
	classified := map[pairKey]bool{}
	for _, tree := range perFileTrees {
		file := syntax.AsFile(tree)
		for _, cm := range file.ContextMaps() {
			scope := cm.Domain()
			for _, edge := range cm.Edges() {
				leftResolved, leftKind, leftAmbig := resolveBCRef(ws, scope, edge.Left())
				rightResolved, rightKind, rightAmbig := resolveBCRef(ws, scope, edge.Right())
				if leftKind != "bc" || leftAmbig || rightKind != "bc" || rightAmbig {
					continue
				}
				if leftResolved == "" || rightResolved == "" || leftResolved == rightResolved {
					continue
				}
				key, _ := newPairKey(leftResolved, rightResolved)
				classified[key] = true
			}
		}
	}

	for key, info := range deps {
		if !(info.fwd || info.rev) {
			continue
		}
		if classified[key] {
			continue
		}
		startChar := colToLSP(info.firstCol)
		length := info.firstLen
		if length <= 0 {
			length = 1
		}
		diags = append(diags, model.Diagnostic{
			Code:      "craft/lint/unclassified-communication",
			Message:   fmt.Sprintf("%q and %q communicate in a use case but have no context_map relationship", key.a, key.b),
			Severity:  model.SeverityHint,
			SourceURI: info.firstURI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(info.firstLine), Character: startChar},
				End:   model.Position{Line: lineToLSP(info.firstLine), Character: startChar + length},
			},
		})
	}

	return diags
}
