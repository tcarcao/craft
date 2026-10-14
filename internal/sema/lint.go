// Package sema — lint rules for Craft DSL. They read the syntax tree's typed
// views (internal/syntax/ast.go) and the workspace symbol table;
// LintCommunication additionally reads the interactions derived from the
// resolved model.
package sema

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// sortedTreeURIs returns the file URIs of perFileTrees in ascending order.
//
// Every rule below walks the workspace file-by-file while accumulating into a
// map keyed by something COARSER than the file — an event name, an actor name,
// a publisher/listener pair. The first file to reach a given key is the one that
// gets recorded, and every later file with the same key is suppressed as a
// duplicate. Ranging over the map directly therefore does not merely shuffle the
// output: it changes WHICH file and line a finding is reported against, because
// a different file wins the race on each run. The finding count stays identical,
// which is exactly what makes the bug easy to miss.
//
// Walking in sorted URI order makes "first file wins" a deterministic statement
// about the corpus instead of a statement about Go's map seed.
func sortedTreeURIs(perFileTrees map[string]syntax.SyntaxNode) []string {
	uris := make([]string, 0, len(perFileTrees))
	for uri := range perFileTrees {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}

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
	diags = append(diags, lintIndistinguishableScenarios(perFileTrees, lis)...)
	diags = append(diags, lintAmbiguousUseCaseRefs(perFileTrees, ws, lis)...)
	diags = append(diags, lintSingleBranchPar(perFileTrees, lis)...)
	diags = append(diags, lintReceivesOnActor(perFileTrees, ws, lis)...)
	diags = append(diags, lintStoreWithoutChange(perFileTrees, lis)...)
	diags = append(diags, lintOperationNotInSurface(perFileTrees, ws, lis)...)
	diags = append(diags, lintRepeatedRequests(perFileTrees, ws, lis)...)
	diags = append(diags, lintDuplicateEventInList(perFileTrees, lis)...)
	diags = append(diags, lintOneOfSingleEvent(perFileTrees, lis)...)
	diags = append(diags, lintDuplicateOperationInList(perFileTrees, lis)...)
	diags = append(diags, lintEgressOnInternalStep(perFileTrees, lis)...)
	diags = append(diags, lintUnquotedAddress(perFileTrees, lis)...)
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

	for _, uri := range sortedTreeURIs(perFileTrees) {
		tree := perFileTrees[uri]
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				if trigger.Kind() == lang.TriggerEvent {
					if ev := trigger.EventValue(); ev != "" {
						consumed[ev] = true
					}
				}
				for _, it := range trigger.EventItems() {
					consumed[it.Value] = true
				}
				// Every item of a `notifies one of` list is published, each
				// at its own position.
				for _, action := range sc.Actions() {
					for _, it := range action.EventItems() {
						if _, seen := published[it.Value]; it.Value == "" || seen {
							continue
						}
						line, col := li.LineCol16(it.Offset)
						published[it.Value] = pubSite{
							uri:      uri,
							event:    it.Value,
							line:     line,
							col:      col,
							tokenLen: eventTokenLen(it.Value, it.IsString),
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
	for _, uri := range sortedTreeURIs(perFileTrees) {
		file := syntax.AsFile(perFileTrees[uri])
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				switch trigger.Kind() {
				case lang.TriggerExternal:
					if _, _, ok := trigger.Schedule(); ok {
						// A scheduled trigger runs as the scheduler actor.
						used[lang.SchedulerActor] = true
					} else if actor := trigger.ActorName(); actor != "" {
						used[actor] = true
					}
				case lang.TriggerDomainListen, lang.TriggerReceive:
					// An actor can be the subject of a listens (or, wrongly, a
					// receives) trigger; the parser stores the subject in Context
					// regardless of whether it is an actor or a bounded context.
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

	for _, uri := range sortedTreeURIs(perFileTrees) {
		tree := perFileTrees[uri]
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
				if trigger.Kind() == lang.TriggerEvent {
					check(trigger.EventValue(), uri, triggerLine, trigger.EventCol(li), trigger.EventIsString())
				}
				// Each item of a listens list, on its own line: a
				// continuation item reports where it is written.
				for _, it := range trigger.EventItems() {
					line, col := li.LineCol16(it.Offset)
					check(it.Value, uri, line, col, it.IsString)
				}
				for _, action := range sc.Actions() {
					for _, it := range action.EventItems() {
						line, col := li.LineCol16(it.Offset)
						check(it.Value, uri, line, col, it.IsString)
					}
				}
			}
		}
	}
	return diags
}

// ── indistinguishable-scenarios ───────────────────────────────────────────────
// Warning: within one use case, a scenario repeats the trigger of an earlier
// scenario and either of them lacks a `scenario "name"`, so a reader cannot
// tell the two apart. One warning per repeating scenario, on its `when`.

func lintIndistinguishableScenarios(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		file := syntax.AsFile(perFileTrees[uri])
		for _, uc := range file.UseCases() {
			if len(uc.Scenarios()) < 2 {
				continue
			}
			// seen maps a normalised trigger to "some earlier scenario with it is unnamed".
			seen := map[string]bool{}
			for _, sc := range uc.Scenarios() {
				when := sc.When()
				if when == nil {
					continue // recovered orphan
				}
				key := strings.Join(strings.Fields(sc.Trigger().SourceText()), " ")
				unnamed := sc.Name() == ""
				earlierUnnamed, dup := seen[key]
				if dup {
					if unnamed || earlierUnnamed {
						line, col := li.LineCol16(when.Offset())
						start := colToLSP(col)
						diags = append(diags, model.Diagnostic{
							Code:     "craft/lint/indistinguishable-scenarios",
							Message:  fmt.Sprintf("scenario repeats the trigger \"when %s\"; use alt \"…\" for different outcomes, or scenario \"…\" names for different ways in", key),
							Severity: model.SeverityWarning,
							Range: model.Range{
								Start: model.Position{Line: lineToLSP(line), Character: start},
								End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(when.Text())},
							},
							SourceURI: uri,
						})
					}
				}
				seen[key] = earlierUnnamed || unnamed
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

// dependenciesFrom builds the dependency adjacency from derived interactions:
// a request makes its subject depend on its target, and a delivered event
// makes the listener depend on the publisher. Only interactions between two
// resolved bounded contexts count.
func dependenciesFrom(its []interaction.Interaction) map[pairKey]depInfo {
	deps := map[pairKey]depInfo{}
	bc := func(e interaction.Endpoint) bool { return e.Kind == model.ParticipantBoundedContext }
	for _, it := range its {
		site := it.Site
		switch it.Kind {
		case interaction.Request:
			if bc(it.From) && bc(it.To) {
				recordDependency(deps, it.From.Key, it.To.Key, site.URI, site.Line, site.Column, site.Length)
			}
		case interaction.Listen:
			if bc(it.From) && bc(it.To) {
				recordDependency(deps, it.To.Key, it.From.Key, site.URI, site.Line, site.Column, site.Length)
			}
		}
	}
	return deps
}

// lintAmbiguousUseCaseRefs reports bare bounded-context names in use-case
// participant slots that resolve to two or more domains. ResolveParticipant
// never guesses an owner, so such a name resolves to nothing in the model —
// it would silently drop out of every diagram without this diagnostic.
//
// Every participant slot is checked: each action's subject, the target of a
// sync or return action, and the subject of an external or domain-listen
// trigger, and the callee of a receive trigger. Each slot is visited once, so one ambiguous name cannot produce two
// diagnostics at the same position.
func lintAmbiguousUseCaseRefs(
	perFileTrees map[string]syntax.SyntaxNode,
	ws WorkspaceSymbols,
	lis map[string]green.LineIndex,
) []model.Diagnostic {
	var diags []model.Diagnostic
	check := func(uri, name string, line, col int) {
		if name == "" {
			return
		}
		if _, ambiguous := ResolveParticipant(ws, "", name); ambiguous {
			diags = append(diags, ambiguousUseCaseRefDiag(uri, name, line, col))
		}
	}
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		file := syntax.AsFile(perFileTrees[uri])
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				line := 0
				if whenTok := sc.When(); whenTok != nil {
					line, _ = li.LineCol(whenTok.Offset())
				}
				switch trigger.Kind() {
				case lang.TriggerExternal:
					check(uri, trigger.ActorName(), line, trigger.ActorCol(li))
				case lang.TriggerDomainListen, lang.TriggerReceive:
					check(uri, trigger.ContextName(), line, trigger.ActorCol(li))
				}

				for _, action := range sc.Actions() {
					check(uri, action.SubjectName(), action.Line(li), action.SubjectCol(li))
					switch action.Kind() {
					case lang.ActionSync, lang.ActionReturn:
						check(uri, action.TargetName(), action.Line(li), action.TargetCol(li))
					}
				}
			}
		}
	}
	return diags
}

// ambiguousUseCaseRefDiag mirrors the ambiguous-bc branch of endpointDiag
// (validate.go:734-741) so the code and wording stay identical across the
// context_map and use_case sites.
func ambiguousUseCaseRefDiag(uri, ref string, line, col int) model.Diagnostic {
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code: "craft/sema/ambiguous-bc",
		Message: fmt.Sprintf(
			"bounded context %q is declared in multiple domains; qualify it as <domain>/%s", ref, ref),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(ref)},
		},
	}
}

// LintCommunication cross-checks the declared context_map edges against the
// communication derived from use cases (its). It runs after the model is
// resolved, which is why it is separate from LintWorkspace:
//   - separate-ways-violation: a `separate_ways` pair that communicates;
//   - relationship-direction-inverted / relationship-bidirectional: traffic
//     against, or in both directions of, an asymmetric pattern;
//   - unclassified-communication: a communicating pair with no context_map
//     edge at all.
func LintCommunication(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex, its []interaction.Interaction) []model.Diagnostic {
	deps := dependenciesFrom(its)

	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		tree := perFileTrees[uri]
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
				if v, ok := lang.LookupEdgeVerb(verb); !ok || v.Symmetric() {
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
	for _, uri := range sortedTreeURIs(perFileTrees) {
		file := syntax.AsFile(perFileTrees[uri])
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

// ── single-branch-par ─────────────────────────────────────────────────────────
// Warning: a par without `and` whose body has exactly one step that is not a
// note, or only notes: nothing in it runs concurrently.

func lintSingleBranchPar(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		var walk func([]syntax.StepDecl)
		walk = func(steps []syntax.StepDecl) {
			for _, st := range steps {
				b, ok := st.Block()
				if !ok {
					continue
				}
				brs := b.Branches()
				// One step, or only notes: either way nothing runs alongside.
				// An empty body is the parser's empty-branch error instead.
				if n := countNonNotes(brs[0].Steps()); b.IsFanOut() && n <= 1 && len(brs[0].Steps()) > 0 {
					msg := "this `par` has one step, so nothing in it runs concurrently; drop the `par` or add the steps that run alongside it"
					if n == 0 {
						msg = "this `par` holds only notes, so nothing in it runs concurrently; drop the `par` or add the steps that run alongside each other"
					}
					if tok := brs[0].HeaderToken(); tok != nil {
						line, col := li.LineCol16(tok.Offset())
						start := colToLSP(col)
						diags = append(diags, model.Diagnostic{
							Code:      "craft/lint/single-branch-par",
							Message:   msg,
							Severity:  model.SeverityWarning,
							SourceURI: uri,
							Range: model.Range{
								Start: model.Position{Line: lineToLSP(line), Character: start},
								End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(tok.Text())},
							},
						})
					}
				}
				for _, br := range brs {
					walk(br.Steps())
				}
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				walk(sc.Steps())
			}
		}
	}
	return diags
}

// countNonNotes counts the steps that are not notes: notes do not make a
// fan-out par branch (spec §4.1 rule 2).
func countNonNotes(steps []syntax.StepDecl) int {
	n := 0
	for _, st := range steps {
		if _, isNote := st.Note(); !isNote {
			n++
		}
	}
	return n
}

// ── receives-on-actor ─────────────────────────────────────────────────────────
// Warning: a `receives` trigger whose subject is an actor.

func lintReceivesOnActor(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				tr := sc.Trigger()
				if tr.Kind() != lang.TriggerReceive || sc.When() == nil {
					continue
				}
				name := tr.ContextName()
				if p, _ := ResolveParticipant(ws, "", name); p.Kind != model.ParticipantActor {
					continue
				}
				line, _ := li.LineCol(sc.When().Offset())
				start := colToLSP(tr.ActorCol(li))
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/receives-on-actor",
					Message:   "an actor starts work with `when <Actor> <verb> …`; `receives` is for a boundary whose caller is not observed",
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(name)},
					},
				})
			}
		}
	}
	return diags
}

// ── store-without-change, store-without-name ──────────────────────────────────
// Warning: a store ref (store.<name>[.<change>]) whose last segment is not a
// change, in a `notifies` or a `listens` trigger (store-without-change); or a
// ref that starts with `store.` but names no store, such as `store.inserted`,
// which falls back to a plain event (store-without-name).

func lintStoreWithoutChange(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		check := func(ref string, line, col int) {
			_, change, ok := lang.ParseStoreRef(ref)
			var code, msg string
			switch {
			case !ok && strings.HasPrefix(ref, lang.StoreTransport+"."):
				// `store.inserted` or a bare `store.`: it reads as a hand-off
				// but parses as a plain event.
				code = "craft/lint/store-without-name"
				msg = fmt.Sprintf("store ref `%s` names no store, so it is drawn as a plain event; write `store.<name>.<change>`, e.g. `store.sms_queue.inserted`", ref)
			case ok && change == "":
				code = "craft/lint/store-without-change"
				msg = fmt.Sprintf("store ref `%s` names no change; end it with `.inserted`, `.updated` or `.deleted`", ref)
			default:
				return
			}
			start := colToLSP(col)
			diags = append(diags, model.Diagnostic{
				Code:      code,
				Message:   msg,
				Severity:  model.SeverityWarning,
				SourceURI: uri,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(line), Character: start},
					End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(ref)},
				},
			})
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				if sc.When() != nil {
					for _, it := range sc.Trigger().EventItems() {
						line, col := li.LineCol16(it.Offset)
						check(it.Ref, line, col)
					}
				}
				for _, a := range sc.Actions() {
					for _, it := range a.EventItems() {
						line, col := li.LineCol16(it.Offset)
						check(it.Ref, line, col)
					}
				}
			}
		}
	}
	return diags
}

// ── repeated-request ──────────────────────────────────────────────────────────
// Warning: two asks in one step list (a scenario body or a branch body, at any
// distance) from different subjects to the same resolved target with the same
// operation. Usually two clients making one request: write it once. A subject
// that was the target of an earlier ask in the list is relaying the request
// (a call chain), and each subject is reported once per request.

func lintRepeatedRequests(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	id := func(name string) string {
		if p, _ := ResolveParticipant(ws, "", name); p.Kind != "" {
			return p.ID
		}
		return name
	}
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		var visit func([]syntax.StepDecl)
		visit = func(steps []syntax.StepDecl) {
			type first struct{ subject, subjectID string }
			seen := map[string]first{}
			asked := map[string]bool{}    // targets asked earlier in this list
			reported := map[string]bool{} // key + subject already warned
			for _, st := range steps {
				if b, ok := st.Block(); ok {
					for _, br := range b.Branches() {
						visit(br.Steps())
					}
					continue
				}
				a, ok := st.Action()
				if !ok || a.Kind() != lang.ActionSync || a.TargetName() == "" || a.SubjectName() == "" {
					continue
				}
				subj := id(a.SubjectName())
				relays := asked[subj]
				asked[id(a.TargetName())] = true
				op := strings.Join(strings.Fields(a.OpText()), " ")
				if op == "" {
					continue
				}
				key := id(a.TargetName()) + "\x00" + op
				f, dup := seen[key]
				if !dup {
					seen[key] = first{a.SubjectName(), subj}
					continue
				}
				// The same subject again, a subject that was itself asked
				// earlier (a call chain relaying the request), or a pair
				// already reported: not two clients making one request.
				if f.subjectID == subj || relays || reported[key+"\x00"+subj] {
					continue
				}
				reported[key+"\x00"+subj] = true
				line, start := a.Line(li), colToLSP(a.SubjectCol(li))
				diags = append(diags, model.Diagnostic{
					Code: "craft/lint/repeated-request",
					Message: fmt.Sprintf("`%s` and `%s` make the same request `[%s]` to `%s`; if they are alternatives, write it once with a `channels` tag, or open the scenario with `alt \"on the web\"` / `else \"in the app\"`",
						a.SubjectName(), f.subject, op, a.TargetName()),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(a.SubjectName())},
					},
				})
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				visit(sc.Steps())
			}
		}
	}
	return diags
}

// ── duplicate-event-in-list ───────────────────────────────────────────────────
// Warning: the same item (exact text) twice in one event list. The range is
// the second one.

func lintDuplicateEventInList(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		check := func(items []syntax.EventItem) {
			seen := map[string]bool{}
			for _, it := range items {
				if !seen[it.Value] {
					seen[it.Value] = true
					continue
				}
				line, col := li.LineCol16(it.Offset)
				start := colToLSP(col)
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/duplicate-event-in-list",
					Message:   fmt.Sprintf("event %q is already in this list", it.Value),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + eventTokenLen(it.Value, it.IsString)},
					},
				})
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				check(sc.Trigger().EventItems())
				for _, a := range sc.Actions() {
					if a.IsOneOf() {
						check(a.EventItems())
					}
				}
			}
		}
	}
	return diags
}

// ── duplicate-operation-in-list ───────────────────────────────────────────────
// Warning: the same item twice in one operation bracket (`[opA, opA, opB]`),
// by exact text. The range is the repeated item.

func lintDuplicateOperationInList(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li, hasLI := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					items := lang.SplitOperationItems(a.OpText())
					if len(items) < 2 {
						continue
					}
					seen := map[string]bool{}
					for _, it := range items {
						if it.Text == "" || !seen[it.Text] {
							seen[it.Text] = true
							continue
						}
						var r model.Range
						if hasLI {
							r = opItemRange(a.OpAnnotation(), it, li)
						}
						diags = append(diags, model.Diagnostic{
							Code:      "craft/lint/duplicate-operation-in-list",
							Message:   fmt.Sprintf("operation %q is already in this bracket; drop the repeat", it.Text),
							Severity:  model.SeverityWarning,
							SourceURI: uri,
							Range:     r,
						})
					}
				}
			}
		}
	}
	return diags
}

// ── one-of-single-event ───────────────────────────────────────────────────────
// Warning: `notifies one of` with a single event. The range is `one of`.

func lintOneOfSingleEvent(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if !a.IsOneOf() || len(a.EventItems()) != 1 {
						continue
					}
					one, of := a.OneOfTokens()
					line, start := li.LineCol16(one.Offset())
					_, end := li.LineCol16(of.Offset() + green.TextSize(len(of.Text())))
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/one-of-single-event",
						Message:   "`notifies one of` names a single event; drop `one of`",
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(line), Character: colToLSP(start)},
							End:   model.Position{Line: lineToLSP(line), Character: colToLSP(end)},
						},
					})
				}
			}
		}
	}
	return diags
}

// ── egress-on-internal-step ───────────────────────────────────────────────────
// Warning: an internal or return action whose bracket has an ADDR or SQL
// item. Both name a call to another party, so the step should be an asks
// (spec §9.3).

func lintEgressOnInternalStep(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li, hasLI := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if k := a.Kind(); k != lang.ActionInternal && k != lang.ActionReturn {
						continue
					}
					for _, it := range lang.SplitOperationItems(a.OpText()) {
						if verb, _ := lang.ParseOperationItem(it.Text); verb == lang.OpVerbAddr || verb == lang.OpVerbSQL {
							diags = append(diags, model.Diagnostic{
								Code:      "craft/lint/egress-on-internal-step",
								Message:   fmt.Sprintf("`[%s …]` names a call to another party; write `%s asks <target> … [%s …]`, declaring the target as a `system` actor if it is outside the estate", verb, a.SubjectName(), verb),
								Severity:  model.SeverityWarning,
								SourceURI: uri,
								Range:     annotationRange(a.OpAnnotation(), li, hasLI),
							})
							break
						}
					}
				}
			}
		}
	}
	return diags
}

// ── unquoted-address ──────────────────────────────────────────────────────────
// Warning: an action whose phrase ends in `[ADDR` because a `//` after a
// space opened a comment and cut the bracket (spec §9.3; plan correction 2).

func lintUnquotedAddress(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li, hasLI := lis[uri]
		if !hasLI {
			continue // the check reads the comment, which only the source holds
		}
		src := li.Src()
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if a.OpAnnotation() != nil || !strings.HasSuffix(strings.TrimSpace(a.PhraseText()), "[ADDR") {
						continue
					}
					line := a.Line(li)
					start := int(li.OffsetFromUTF16(line, 0))
					end := strings.IndexByte(src[start:], '\n')
					if end < 0 {
						end = len(src) - start
					}
					text := src[start : start+end]
					at := strings.LastIndex(text, "[ADDR")
					if at < 0 || !strings.Contains(text[at:], "//") {
						continue
					}
					col := lspLen(text[:at])
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/unquoted-address",
						Message:   "the address was cut by a `//` comment; quote it: `[ADDR \"//…\"]`",
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(line), Character: col},
							End:   model.Position{Line: lineToLSP(line), Character: col + len("[ADDR")},
						},
					})
				}
			}
		}
	}
	return diags
}
