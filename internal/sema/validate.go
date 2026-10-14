// Task 7 (Slice E): local well-formedness validation — slug shape,
// context_map edge endpoint kinds, deprecated quoted notifies/listens forms,
// and duplicate service catalog_ref:/repo: anchors. craft validates SHAPE only;
// the hub is authoritative for cross-context / cross-repo resolution. See
// docs/superpowers/specs/2026-07-14-craft-dsl-vnext-design.md §6.
//
// These checks are wired into AnalyzeFile (per-file, appended to its
// returned diagnostics) and AnalyzeWorkspace (cross-file best-effort local
// resolution of collected SlugRefSite entries — unresolved-ref-local).
package sema

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// SlugRefSite records a single kind-prefixed node-slug reference (domain:,
// bc:, or service: — term: is skipped, since craft tracks no term
// declarations at all; that resolution is hub-only) for cross-file
// "declared in the file-set" resolution in AnalyzeWorkspace
// (craft/sema/unresolved-ref-local).
type SlugRefSite struct {
	// Kind is "domain", "bc", or "service".
	Kind string
	// Name is the final name segment (after the last '/', or the whole
	// remainder for a service: alias, which has no namespace).
	Name string
	// Text is the full ref source text, for the diagnostic message/range.
	Text string
	URI  string
	Line int
	Col  int
}

// isRecognisedSlugKind reports whether kind is one of the four node-slug kind
// words (domain/bc/term/service).
func isRecognisedSlugKind(kind string) bool {
	switch kind {
	case "domain", "bc", "term", "service":
		return true
	}
	return false
}

// refKindWord returns the leading `kind:` word of a ref's raw text if present
// and recognised, else "". Operates directly on RefText() rather than
// RefDecl.RefKind() so that an UNRECOGNISED kind word (e.g. "foo:re/x") can
// still be identified as an attempted-but-invalid kind prefix — RefKind()
// deliberately returns "" for both "no prefix" and "invalid prefix", which
// this rule needs to distinguish.
func refKindWord(text string) string {
	idx := strings.IndexByte(text, ':')
	if idx < 0 {
		return ""
	}
	kind := text[:idx]
	if isRecognisedSlugKind(kind) {
		return kind
	}
	return ""
}

// slugShapeError checks a node-slug ref's text against its per-kind
// namespace rule (Task 7, design §6): domain:re/<name>, bc:<domain>/<name>,
// term:<bc>/<name>, service:<alias> (no namespace/slash). Returns "" if the
// text is not a kind-prefixed slug at all (bare short form / dotted event
// ref — nothing to check here) or if it is well-formed; else a
// human-readable reason for the malformed-slug diagnostic.
func slugShapeError(text string) string {
	idx := strings.IndexByte(text, ':')
	if idx < 0 {
		// No `kind:` prefix. A '/' makes this the qualified `<domain>/<name>`
		// form, which must be exactly two non-empty segments. Without a '/'
		// the text is a bare short form or a dotted event ref and imposes no
		// shape at all.
		//
		// This branch used to return "" unconditionally, which meant a
		// slash-shaped name skipped shape validation entirely: "re/" and
		// "re//billing" were accepted in silence in every ref slot.
		if strings.ContainsRune(text, '/') {
			parts := strings.Split(text, "/")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return `a qualified reference must be exactly two non-empty segments, "<domain>/<name>"`
			}
		}
		return ""
	}
	kind := text[:idx]
	rest := text[idx+1:]
	if !isRecognisedSlugKind(kind) {
		return fmt.Sprintf("%q is not a recognised slug kind (must be one of domain, bc, term, service)", kind)
	}
	parts := strings.Split(rest, "/")
	switch kind {
	case "domain":
		if len(parts) != 2 || parts[0] != "re" || parts[1] == "" {
			return fmt.Sprintf(`domain slug must be shaped "domain:re/<name>", got %q`, text)
		}
	case "bc":
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Sprintf(`bc slug must be shaped "bc:<domain>/<name>", got %q`, text)
		}
	case "term":
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Sprintf(`term slug must be shaped "term:<bc>/<name>", got %q`, text)
		}
	case "service":
		if len(parts) != 1 || parts[0] == "" {
			return fmt.Sprintf(`service slug must be shaped "service:<alias>" (no namespace), got %q`, text)
		}
	}
	return ""
}

// slugName returns the final name segment of a well-shaped node-slug's text
// (after the last '/', or the whole remainder for a service: alias). Callers
// must have already confirmed the shape is valid via slugShapeError.
func slugName(text string) string {
	idx := strings.IndexByte(text, ':')
	if idx < 0 {
		return text
	}
	rest := text[idx+1:]
	if i := strings.LastIndexByte(rest, '/'); i >= 0 {
		return rest[i+1:]
	}
	return rest
}

// checkSlugRef validates one ref's shape, appending a malformed-slug
// diagnostic if it is an invalid kind-prefixed slug. If it is a well-formed
// domain:/bc:/service: slug, it also appends a SlugRefSite for later
// cross-file resolution (term: is skipped — see SlugRefSite). text == ""
// (no ref present) is a no-op.
func checkSlugRef(uri, text string, line, col int, diags *[]model.Diagnostic, refs *[]SlugRefSite) {
	if text == "" {
		return
	}
	if reason := slugShapeError(text); reason != "" {
		*diags = append(*diags, malformedSlugDiag(uri, text, reason, line, col))
		return
	}
	kind := refKindWord(text)
	if kind == "" || kind == "term" {
		return // bare short form / event ref / term (hub-only) — not locally resolvable
	}
	*refs = append(*refs, SlugRefSite{
		Kind: kind,
		Name: slugName(text),
		Text: text,
		URI:  uri,
		Line: line,
		Col:  col,
	})
}

func malformedSlugDiag(uri, text, reason string, line, col int) model.Diagnostic {
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/malformed-slug",
		Message:   fmt.Sprintf("malformed node slug %q: %s", text, reason),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(text)},
		},
	}
}

// validateUseCaseRefs walks every use-case trigger/action ref (domain_listen
// event, sync_action target, async_action event): checks slug shape
// (malformed-slug) and flags legacy quoted notifies/listens strings
// (deprecated-string-ref). Also returns well-formed slug refs collected for
// cross-file resolution (AnalyzeWorkspace).
func validateUseCaseRefs(uri string, file syntax.File, li green.LineIndex, hasLI bool) ([]model.Diagnostic, []SlugRefSite) {
	var diags []model.Diagnostic
	var refs []SlugRefSite

	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			trigger := sc.Trigger()
			triggerLine := 0
			if hasLI {
				if whenTok := sc.When(); whenTok != nil {
					triggerLine, _ = li.LineCol(whenTok.Offset())
				}
			}

			// Trigger subject: the actor of an external trigger or the context
			// of a domain_listen one. Both read the same slot, and since
			// Task 6b both accept the qualified <domain>/<name> form, so both
			// need the same shape check. Checking only the domain_listen
			// branch would miss the malformed cases: a stray '/' swallows the
			// verb, which re-classifies the trigger as external.
			subject := trigger.ContextName()
			if subject == "" {
				subject = trigger.ActorName()
			}
			if subject != "" {
				col := 0
				if hasLI {
					col = trigger.ActorCol(li)
				}
				checkSlugRef(uri, subject, triggerLine, col, &diags, &refs)
			}

			if trigger.Kind() == lang.TriggerDomainListen {
				col := 0
				if hasLI {
					col = trigger.EventCol(li)
				}
				if trigger.EventIsString() {
					diags = append(diags, deprecatedStringRefDiag(uri, "listens", trigger.EventValue(), triggerLine, col))
				} else {
					checkSlugRef(uri, trigger.EventValue(), triggerLine, col, &diags, &refs)
				}
			}

			for _, action := range sc.Actions() {
				line := 0
				if hasLI {
					line = action.Line(li)
				}
				// The subject slot exists for all four action kinds and, since
				// Task 6b, accepts a qualified ref in each of them.
				if subj := action.SubjectName(); subj != "" {
					col := 0
					if hasLI {
						col = action.SubjectCol(li)
					}
					checkSlugRef(uri, subj, line, col, &diags, &refs)
				}
				switch action.Kind() {
				case lang.ActionAsync:
					col := 0
					if hasLI {
						col = action.EventCol(li)
					}
					if action.EventIsString() {
						diags = append(diags, deprecatedStringRefDiag(uri, "notifies", action.EventValue(), line, col))
					} else {
						checkSlugRef(uri, action.EventValue(), line, col, &diags, &refs)
					}
				case lang.ActionSync, lang.ActionReturn:
					// return_action's `to <target>` became a ref slot in
					// Task 6b, so it is validated alongside the asks target.
					col := 0
					if hasLI {
						col = action.TargetCol(li)
					}
					checkSlugRef(uri, action.TargetName(), line, col, &diags, &refs)
				}
			}
		}
	}
	return diags, refs
}

func deprecatedStringRefDiag(uri, verb, event string, line, col int) model.Diagnostic {
	startChar := colToLSP(col)
	tokLen := eventTokenLen(event, true)
	return model.Diagnostic{
		Code:      "craft/lint/deprecated-string-ref",
		Message:   fmt.Sprintf("%s %q uses the legacy quoted-string form; migrate to a typed ref", verb, event),
		Severity:  model.SeverityWarning,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + tokLen},
		},
	}
}

// validateContextMapEdges checks each context_map edge for slug-shape
// validity (malformed-slug) and verb recognition. Endpoint-kind validation
// and cross-file resolution are DEFERRED to a later task. Also returns
// well-formed slug refs collected for resolution.
func validateContextMapEdges(uri string, file syntax.File, li green.LineIndex, hasLI bool) ([]model.Diagnostic, []SlugRefSite) {
	var diags []model.Diagnostic
	var refs []SlugRefSite

	for _, cm := range file.ContextMaps() {
		for _, edge := range cm.Edges() {
			left, right, verb := edge.Left(), edge.Right(), edge.Verb()

			leftLine, leftCol := 0, 0
			if hasLI {
				if lr := edge.LeftRef(); lr != nil {
					leftLine, leftCol = lr.Line(li), lr.Col(li)
				}
			}
			rightLine, rightCol := 0, 0
			if hasLI {
				if rr := edge.RightRef(); rr != nil {
					rightLine, rightCol = rr.Line(li), rr.Col(li)
				}
			}

			checkSlugRef(uri, left, leftLine, leftCol, &diags, &refs)
			checkSlugRef(uri, right, rightLine, rightCol, &diags, &refs)

			diags = append(diags, classifyEdgeVerb(uri, verb, left, leftLine, leftCol)...)
		}
	}
	return diags, refs
}

// classifyEdgeVerb recognises one context_map edge verb. A verb in
// lang's edge verbs (the 8 DDD patterns) are accepted with no diagnostic. The
// parser accepts exactly the same table, so the error branch is unreachable
// from .craft source; it stays as a loud internal error and is unit-tested
// directly.
func classifyEdgeVerb(uri, verb, left string, leftLine, leftCol int) []model.Diagnostic {
	if lang.IsEdgeVerb(verb) {
		return nil
	}
	startChar := colToLSP(leftCol)
	return []model.Diagnostic{{
		Code:      "craft/sema/unrecognised-edge-verb",
		Message:   fmt.Sprintf("edge verb %q is not a recognised relationship pattern (internal error, not a user error)", verb),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
			End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + lspLen(left)},
		},
	}}
}

// ContextMapEdgeSite records one context_map edge for cross-file endpoint
// resolution in AnalyzeWorkspace (Task 4). Mirrors SlugRefSite: collected
// per-file in AnalyzeFile (which has no WorkspaceSymbols) and resolved in the
// workspace pass, so a BC declared in another file is not false-flagged.
type ContextMapEdgeSite struct {
	// ScopeDomain is the owning block's domain scope (ContextMapDecl.Domain(),
	// "" if the block is unscoped).
	ScopeDomain string
	// Left, Right are the raw endpoint texts (bare `name` or qualified `domain/name`).
	Left, Right         string
	Verb                string
	URI                 string
	LeftLine, LeftCol   int
	RightLine, RightCol int
}

// collectContextMapEdgeSites gathers one ContextMapEdgeSite per edge, using the
// same position-extraction idiom as validateContextMapEdges. Resolution is
// deferred to AnalyzeWorkspace (endpoints are cross-domain / cross-file).
func collectContextMapEdgeSites(uri string, file syntax.File, li green.LineIndex, hasLI bool) []ContextMapEdgeSite {
	var sites []ContextMapEdgeSite
	for _, cm := range file.ContextMaps() {
		scope := cm.Domain()
		for _, edge := range cm.Edges() {
			leftLine, leftCol := 0, 0
			if hasLI {
				if lr := edge.LeftRef(); lr != nil {
					leftLine, leftCol = lr.Line(li), lr.Col(li)
				}
			}
			rightLine, rightCol := 0, 0
			if hasLI {
				if rr := edge.RightRef(); rr != nil {
					rightLine, rightCol = rr.Line(li), rr.Col(li)
				}
			}
			sites = append(sites, ContextMapEdgeSite{
				ScopeDomain: scope,
				Left:        edge.Left(),
				Right:       edge.Right(),
				Verb:        edge.Verb(),
				URI:         uri,
				LeftLine:    leftLine,
				LeftCol:     leftCol,
				RightLine:   rightLine,
				RightCol:    rightCol,
			})
		}
	}
	return sites
}

func domainHasBC(dom DomainSymbol, name string) bool {
	for _, bc := range dom.BoundedContexts {
		if bc == name {
			return true
		}
	}
	return false
}

// relationshipEdgeDiags resolves both endpoints of a collected context_map edge
// and emits the endpoint-kind and self-relationship diagnostics (Task 4).
func relationshipEdgeDiags(ws WorkspaceSymbols, site ContextMapEdgeSite) []model.Diagnostic {
	var diags []model.Diagnostic

	leftResolved, leftKind, leftAmbig := resolveBCRef(ws, site.ScopeDomain, site.Left)
	rightResolved, rightKind, rightAmbig := resolveBCRef(ws, site.ScopeDomain, site.Right)

	diags = append(diags, endpointDiag(site.URI, site.Left, leftKind, leftAmbig, site.LeftLine, site.LeftCol)...)
	diags = append(diags, endpointDiag(site.URI, site.Right, rightKind, rightAmbig, site.RightLine, site.RightCol)...)

	// A relationship must not connect a bounded context to itself. Only fires
	// when BOTH endpoints resolved unambiguously to the SAME bc.
	if leftKind == "bc" && !leftAmbig && rightKind == "bc" && !rightAmbig &&
		leftResolved != "" && leftResolved == rightResolved {
		startChar := colToLSP(site.LeftCol)
		diags = append(diags, model.Diagnostic{
			Code:      "craft/sema/self-relationship",
			Message:   fmt.Sprintf("relationship connects bounded context %q to itself", leftResolved),
			Severity:  model.SeverityError,
			SourceURI: site.URI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(site.LeftLine), Character: startChar},
				End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + lspLen(site.Left)},
			},
		})
	}
	return diags
}

// GlossaryRelationSite records one glossary term relation for cross-file
// endpoint resolution in AnalyzeWorkspace (Task A3). Mirrors
// ContextMapEdgeSite: collected per-file in AnalyzeFile (which has no
// WorkspaceSymbols) and resolved in the workspace pass, so a BC declared in
// another file is not false-flagged.
type GlossaryRelationSite struct {
	// ScopeDomain is the owning block's domain scope (GlossaryDecl.Domain(),
	// "" if the block is unscoped).
	ScopeDomain string
	// Left, Right are the raw term-node texts (e.g. "billing/Invoice").
	Left, Right         string
	Verb                string
	URI                 string
	LeftLine, LeftCol   int
	RightLine, RightCol int
}

// collectGlossaryRelationSites gathers one GlossaryRelationSite per relation,
// using the same position-extraction idiom as collectContextMapEdgeSites.
// Resolution is deferred to AnalyzeWorkspace (endpoints are cross-domain /
// cross-file).
func collectGlossaryRelationSites(uri string, file syntax.File, li green.LineIndex, hasLI bool) []GlossaryRelationSite {
	var sites []GlossaryRelationSite
	for _, gd := range file.Glossaries() {
		scope := gd.Domain()
		for _, rel := range gd.Relations() {
			leftLine, leftCol := 0, 0
			if hasLI {
				if lr := rel.LeftRef(); lr != nil {
					leftLine, leftCol = lr.Line(li), lr.Col(li)
				}
			}
			rightLine, rightCol := 0, 0
			if hasLI {
				if rr := rel.RightRef(); rr != nil {
					rightLine, rightCol = rr.Line(li), rr.Col(li)
				}
			}
			sites = append(sites, GlossaryRelationSite{
				ScopeDomain: scope,
				Left:        rel.Left(),
				Right:       rel.Right(),
				Verb:        rel.Verb(),
				URI:         uri,
				LeftLine:    leftLine,
				LeftCol:     leftCol,
				RightLine:   rightLine,
				RightCol:    rightCol,
			})
		}
	}
	return sites
}

// resolveTermNode resolves a glossary term node (e.g. "billing/Invoice",
// "re/billing/Invoice", or a bare "Invoice") against the workspace symbol
// table. The LAST slash-separated segment is the term name — a free string,
// never validated. The remaining segments (if any) are reconstructed into a
// BC reference and resolved via resolveBCRef: zero remaining segments (a bare
// one-segment node) yields an empty BC ref, one remaining segment yields a
// bare `bc` ref, two or more remaining segments are joined back with "/"
// (the common case being `domain/bc`). Returns the canonical identity
// (resolvedBC + "/" + term, used for self-relation comparison), the BC kind
// ("bc"/"domain"/"service"/"actor"/"" for fully unresolved), ambiguity, and
// the extracted term name.
func resolveTermNode(ws WorkspaceSymbols, scopeDomain, node string) (canonical, bcKind string, ambiguous bool, term string) {
	segments := strings.Split(node, "/")
	term = segments[len(segments)-1]
	bcRef := strings.Join(segments[:len(segments)-1], "/")

	resolvedBC, kind, ambig := resolveBCRef(ws, scopeDomain, bcRef)
	return resolvedBC + "/" + term, kind, ambig, term
}

// glossaryEndpointDiag emits at most one diagnostic for a single resolved
// glossary term-node endpoint: glossary-ambiguous-bc (error),
// glossary-endpoint-not-bc (error), or glossary-unresolved-bc (warning).
// Mirrors endpointDiag.
func glossaryEndpointDiag(uri, ref, kind string, ambiguous bool, line, col int) []model.Diagnostic {
	startChar := colToLSP(col)
	rng := model.Range{
		Start: model.Position{Line: lineToLSP(line), Character: startChar},
		End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(ref)},
	}
	switch {
	case ambiguous:
		return []model.Diagnostic{{
			Code:      "craft/sema/glossary-ambiguous-bc",
			Message:   fmt.Sprintf("bounded context in term node %q is declared in multiple domains; qualify it as <domain>/%s", ref, ref),
			Severity:  model.SeverityError,
			SourceURI: uri,
			Range:     rng,
		}}
	case kind != "" && kind != "bc":
		return []model.Diagnostic{{
			Code:      "craft/sema/glossary-endpoint-not-bc",
			Message:   fmt.Sprintf("glossary term node %q resolves to a %s, not a bounded context", ref, kind),
			Severity:  model.SeverityError,
			SourceURI: uri,
			Range:     rng,
		}}
	case kind == "":
		return []model.Diagnostic{{
			Code:      "craft/sema/glossary-unresolved-bc",
			Message:   fmt.Sprintf("glossary term node %q does not resolve to any bounded context", ref),
			Severity:  model.SeverityWarning,
			SourceURI: uri,
			Range:     rng,
		}}
	}
	return nil
}

// glossaryRelationDiags resolves both term-node endpoints of a collected
// glossary relation and emits the endpoint-kind and self-relation
// diagnostics (Task A3). Mirrors relationshipEdgeDiags.
func glossaryRelationDiags(ws WorkspaceSymbols, site GlossaryRelationSite) []model.Diagnostic {
	var diags []model.Diagnostic

	leftCanonical, leftKind, leftAmbig, _ := resolveTermNode(ws, site.ScopeDomain, site.Left)
	rightCanonical, rightKind, rightAmbig, _ := resolveTermNode(ws, site.ScopeDomain, site.Right)

	diags = append(diags, glossaryEndpointDiag(site.URI, site.Left, leftKind, leftAmbig, site.LeftLine, site.LeftCol)...)
	diags = append(diags, glossaryEndpointDiag(site.URI, site.Right, rightKind, rightAmbig, site.RightLine, site.RightCol)...)

	// A glossary relation must not connect a term node to itself. Only fires
	// when BOTH endpoints resolved unambiguously to the SAME bc and the same
	// canonical term identity.
	if leftKind == "bc" && !leftAmbig && rightKind == "bc" && !rightAmbig &&
		leftCanonical != "" && leftCanonical == rightCanonical {
		startChar := colToLSP(site.LeftCol)
		diags = append(diags, model.Diagnostic{
			Code:      "craft/sema/glossary-self-relation",
			Message:   fmt.Sprintf("glossary relation connects term node %q to itself", leftCanonical),
			Severity:  model.SeverityError,
			SourceURI: site.URI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(site.LeftLine), Character: startChar},
				End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + lspLen(site.Left)},
			},
		})
	}
	return diags
}

// glossaryLintDiags flags two glossary lints (Task A4) for a relation whose
// both term-node endpoints resolved unambiguously to a bc — mirroring
// redundantRelationshipDiag's gating and dedup strategy:
//
//   - craft/lint/glossary-redundant: the same unordered canonical term-node
//     pair declared with the same verb more than once anywhere in the
//     workspace.
//   - craft/lint/glossary-conflicting-relation: the same unordered pair
//     carries both same_as and (distinct_from or contrasts) — asserting
//     identity and difference at once. Fires once per pair, on whichever
//     relation completes the conflicting verb set.
//
// seenVerb, verbsForPair, and conflictReported are shared across the whole
// workspace resolution loop (like redundantRelationshipDiag's seen) so
// duplicates/conflicts split across files still dedupe/fire correctly.
func glossaryLintDiags(
	ws WorkspaceSymbols,
	site GlossaryRelationSite,
	seenVerb map[string]bool,
	verbsForPair map[string]map[string]bool,
	conflictReported map[string]bool,
) []model.Diagnostic {
	leftCanonical, leftKind, leftAmbig, _ := resolveTermNode(ws, site.ScopeDomain, site.Left)
	rightCanonical, rightKind, rightAmbig, _ := resolveTermNode(ws, site.ScopeDomain, site.Right)

	if leftKind != "bc" || leftAmbig || leftCanonical == "" ||
		rightKind != "bc" || rightAmbig || rightCanonical == "" {
		return nil
	}

	pair := [2]string{leftCanonical, rightCanonical}
	if pair[0] > pair[1] {
		pair[0], pair[1] = pair[1], pair[0]
	}
	pairKey := pair[0] + "\x00" + pair[1]
	verbKey := pairKey + "\x00" + site.Verb

	startChar := colToLSP(site.LeftCol)
	rng := model.Range{
		Start: model.Position{Line: lineToLSP(site.LeftLine), Character: startChar},
		End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + lspLen(site.Left)},
	}

	var diags []model.Diagnostic

	if seenVerb[verbKey] {
		diags = append(diags, model.Diagnostic{
			Code: "craft/lint/glossary-redundant",
			Message: fmt.Sprintf(
				"%s relation between %q and %q is already declared elsewhere in the workspace",
				site.Verb, leftCanonical, rightCanonical,
			),
			Severity:  model.SeverityWarning,
			SourceURI: site.URI,
			Range:     rng,
		})
	}
	seenVerb[verbKey] = true

	verbs := verbsForPair[pairKey]
	if verbs == nil {
		verbs = map[string]bool{}
		verbsForPair[pairKey] = verbs
	}
	verbs[site.Verb] = true

	if verbs["same_as"] && (verbs["distinct_from"] || verbs["contrasts"]) && !conflictReported[pairKey] {
		diags = append(diags, model.Diagnostic{
			Code: "craft/lint/glossary-conflicting-relation",
			Message: fmt.Sprintf(
				"%q and %q are declared same_as and also distinct/contrasting elsewhere in the workspace",
				leftCanonical, rightCanonical,
			),
			Severity:  model.SeverityWarning,
			SourceURI: site.URI,
			Range:     rng,
		})
		conflictReported[pairKey] = true
	}

	return diags
}

// redundantRelationshipDiag flags a symmetric context_map edge (partnership,
// shared_kernel, separate_ways) whose resolved BC pair + verb was already
// seen elsewhere in the workspace (Task 5). Symmetric verbs express an
// undirected fact, so "a partnership b" and "b partnership a" are the same
// declaration; the second (and any later) occurrence is redundant. Only
// fires when both endpoints resolved unambiguously to a bc — unresolved or
// ambiguous endpoints already get their own diagnostics from
// relationshipEdgeDiags and must not also trigger this lint. seen is shared
// across the whole workspace resolution loop so duplicates split across
// files still dedupe.
func redundantRelationshipDiag(ws WorkspaceSymbols, site ContextMapEdgeSite, seen map[string]bool) []model.Diagnostic {
	if v, _ := lang.LookupEdgeVerb(site.Verb); !v.Symmetric() {
		return nil
	}

	leftResolved, leftKind, leftAmbig := resolveBCRef(ws, site.ScopeDomain, site.Left)
	rightResolved, rightKind, rightAmbig := resolveBCRef(ws, site.ScopeDomain, site.Right)

	if leftKind != "bc" || leftAmbig || leftResolved == "" ||
		rightKind != "bc" || rightAmbig || rightResolved == "" {
		return nil
	}

	pair := [2]string{leftResolved, rightResolved}
	if pair[0] > pair[1] {
		pair[0], pair[1] = pair[1], pair[0]
	}
	key := pair[0] + "|" + pair[1] + "|" + site.Verb

	if seen[key] {
		startChar := colToLSP(site.LeftCol)
		return []model.Diagnostic{{
			Code: "craft/lint/redundant-relationship",
			Message: fmt.Sprintf(
				"%s relationship between %q and %q is already declared elsewhere in the workspace",
				site.Verb, leftResolved, rightResolved,
			),
			Severity:  model.SeverityWarning,
			SourceURI: site.URI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(site.LeftLine), Character: startChar},
				End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + lspLen(site.Left)},
			},
		}}
	}
	seen[key] = true
	return nil
}

// endpointDiag emits at most one diagnostic for a single resolved endpoint:
// ambiguous-bc (error), edge-endpoint-not-bc (error), or unresolved-bc (warning).
func endpointDiag(uri, ref, kind string, ambiguous bool, line, col int) []model.Diagnostic {
	startChar := colToLSP(col)
	rng := model.Range{
		Start: model.Position{Line: lineToLSP(line), Character: startChar},
		End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(ref)},
	}
	switch {
	case ambiguous:
		return []model.Diagnostic{{
			Code:      "craft/sema/ambiguous-bc",
			Message:   fmt.Sprintf("bounded context %q is declared in multiple domains; qualify it as <domain>/%s", ref, ref),
			Severity:  model.SeverityError,
			SourceURI: uri,
			Range:     rng,
		}}
	case kind != "" && kind != "bc":
		return []model.Diagnostic{{
			Code:      "craft/sema/edge-endpoint-not-bc",
			Message:   fmt.Sprintf("context_map endpoint %q resolves to a %s, not a bounded context", ref, kind),
			Severity:  model.SeverityError,
			SourceURI: uri,
			Range:     rng,
		}}
	case kind == "":
		return []model.Diagnostic{{
			Code:      "craft/sema/unresolved-bc",
			Message:   fmt.Sprintf("context_map endpoint %q does not resolve to any bounded context", ref),
			Severity:  model.SeverityWarning,
			SourceURI: uri,
			Range:     rng,
		}}
	}
	return nil
}

// validateServiceAnchors flags a repeated catalog_ref: or repo: field within a
// single service block (duplicate-service-anchor). Every occurrence after
// the first is reported.
func validateServiceAnchors(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, svc := range file.Services() {
		svcName := ""
		if nameTok := svc.Name(); nameTok != nil {
			svcName = syntax.StringAwareText(*nameTok)
		}
		seenCatalogRef, seenRepo := false, false
		for _, f := range svc.Fields() {
			switch {
			case f.IsCatalogRef():
				if seenCatalogRef {
					diags = append(diags, duplicateAnchorDiag(uri, "catalog_ref", svcName, f, li, hasLI))
				}
				seenCatalogRef = true
			case f.IsRepo():
				if seenRepo {
					diags = append(diags, duplicateAnchorDiag(uri, "repo", svcName, f, li, hasLI))
				}
				seenRepo = true
			}
		}
	}
	return diags
}

func duplicateAnchorDiag(uri, field, svcName string, f syntax.ServiceField, li green.LineIndex, hasLI bool) model.Diagnostic {
	line, col := 0, 0
	if hasLI {
		line, col = f.Line(li), f.Col(li)
	}
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/duplicate-service-anchor",
		Message:   fmt.Sprintf("service %q: %q is already declared; only one `%s:` is allowed per service", svcName, field, field),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(field)},
		},
	}
}

// validateUseCaseTags flags a repeated tag key within one `tags { }` block,
// or a second `tags { }` block in one use case (duplicate-tag, Task 4,
// Slice B). Every occurrence after the first is reported. Severity is
// WARNING, not error: Task 3's projection is last-write-wins for a repeated
// key, so this is legal input — the rule just flags the likely mistake.
func validateUseCaseTags(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range file.UseCases() {
		ucName := uc.Name()
		seenKeys := map[string]bool{}
		for i, tb := range uc.TagsBlocks() {
			if i > 0 {
				diags = append(diags, duplicateTagBlockDiag(uri, ucName, tb, li, hasLI))
			}
			for _, tag := range tb.Tags() {
				keyTok := tag.Key()
				if keyTok == nil {
					continue
				}
				key := keyTok.Text()
				if seenKeys[key] {
					diags = append(diags, duplicateTagKeyDiag(uri, ucName, key, keyTok, li, hasLI))
				}
				seenKeys[key] = true
			}
		}
	}
	return diags
}

func duplicateTagKeyDiag(uri, ucName, key string, keyTok *syntax.SyntaxToken, li green.LineIndex, hasLI bool) model.Diagnostic {
	line, col := 0, 0
	if hasLI {
		line, col = li.LineCol16(keyTok.Offset())
	}
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/duplicate-tag",
		Message:   fmt.Sprintf("use case %q: tag %q is already set; last value wins", ucName, key),
		Severity:  model.SeverityWarning,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(key)},
		},
	}
}

func duplicateTagBlockDiag(uri, ucName string, tb syntax.TagsBlock, li green.LineIndex, hasLI bool) model.Diagnostic {
	line, col := 0, 0
	if hasLI {
		if kw := tb.Keyword(); kw != nil {
			line, col = li.LineCol16(kw.Offset())
		}
	}
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/duplicate-tag",
		Message:   fmt.Sprintf("use case %q: a second `tags` block is already declared; only one is allowed per use case", ucName),
		Severity:  model.SeverityWarning,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen("tags")},
		},
	}
}

// ── invalid-schedule ──────────────────────────────────────────────────────────

// cronMacros are the descriptors accepted in place of the fields. `@every` is
// deliberately absent: an interval is the `every` form.
var cronMacros = map[string]bool{
	"@yearly": true, "@annually": true, "@monthly": true, "@weekly": true,
	"@daily": true, "@midnight": true, "@hourly": true,
}

var cronFieldRe = regexp.MustCompile(`^[0-9*/,\-?LW#A-Za-z]+$`)

// scheduleError returns why a scheduled trigger's expression is malformed, or
// "" when it is well-formed. craft checks shape only, not whether a field's
// value is in range.
func scheduleError(kind lang.ScheduleKind, expr string) string {
	switch kind {
	case lang.ScheduleCron:
		if strings.HasPrefix(expr, "@") {
			if cronMacros[expr] {
				return ""
			}
			if strings.HasPrefix(expr, "@every") {
				return fmt.Sprintf("cron macro %q is not accepted; @every is not accepted; use when every \"5m\" for an interval", expr)
			}
			return fmt.Sprintf("unknown cron macro %q; use @yearly, @annually, @monthly, @weekly, @daily, @midnight or @hourly, or 5 or 6 fields like \"0 7 * * 1-5\"", expr)
		}
		fields := strings.Fields(expr)
		if len(fields) < 5 || len(fields) > 6 {
			return fmt.Sprintf("cron expression must have 5 or 6 fields like \"0 7 * * 1-5\" (or a macro such as @daily), got %q", expr)
		}
		for _, f := range fields {
			if !cronFieldRe.MatchString(f) {
				return fmt.Sprintf("cron field %q has characters outside 0-9 * / , - ? L W # and letters; expected a form like \"0 7 * * 1-5\"", f)
			}
		}
	case lang.ScheduleEvery:
		if !positiveDuration(expr) {
			return fmt.Sprintf("every interval must be a positive duration like \"5m\", \"1h30m\" or \"2d\", got %q", expr)
		}
	}
	return ""
}

// positiveDuration accepts a positive time.ParseDuration value, or `<n>d` for
// whole days.
func positiveDuration(s string) bool {
	if n, ok := strings.CutSuffix(s, "d"); ok {
		// Digits only: Atoi would accept a leading sign.
		if n == "" || strings.Trim(n, "0123456789") != "" {
			return false
		}
		days, err := strconv.Atoi(n)
		return err == nil && days > 0
	}
	d, err := time.ParseDuration(s)
	return err == nil && d > 0
}

// validateSchedules flags a `when cron "…"` / `when every "…"` trigger whose
// expression is not well-formed. The range is the string token.
func validateSchedules(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			trigger := sc.Trigger()
			kind, expr, ok := trigger.Schedule()
			if !ok {
				continue
			}
			tok := trigger.ScheduleToken()
			if tok == nil {
				continue // the parser already reported the missing string
			}
			msg := scheduleError(kind, expr)
			if msg == "" {
				continue
			}
			line, col := 0, 0
			if hasLI {
				line, col = li.LineCol16(tok.Offset())
			}
			start := colToLSP(col)
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/invalid-schedule",
				Message:   msg,
				Severity:  model.SeverityError,
				SourceURI: uri,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(line), Character: start},
					End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(tok.Text())},
				},
			})
		}
	}
	return diags
}
