// Task 7 (Slice E): local well-formedness validation — slug shape,
// context_map edge endpoint kinds, deprecated quoted notifies/listens forms,
// and duplicate service opslevel:/repo: anchors. craft validates SHAPE only;
// the hub is authoritative for cross-context / cross-repo resolution. See
// docs/superpowers/specs/2026-07-14-craft-dsl-vnext-design.md §6.
//
// These checks are wired into AnalyzeFile (per-file, appended to its
// returned diagnostics) and AnalyzeWorkspace (cross-file best-effort local
// resolution of collected SlugRefSite entries — unresolved-ref-local).
package sema

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// edgeRelationshipVerbs is sema's recognition set for context_map edge verbs:
// the 8 DDD strategic context-mapping patterns. It MUST stay in sync with the
// parser's edgeKeywords (asserted by TestEdgeVerbVocabulariesInSync). Recognition
// only — endpoint resolution, endpoint-not-bc, self-relationship, and redundant
// symmetric lint are LATER tasks and are deliberately not implemented here.
var edgeRelationshipVerbs = map[string]bool{
	// directional
	"customer_supplier": true, "conformist": true, "anticorruption_layer": true,
	"open_host_service": true, "published_language": true,
	// symmetric
	"partnership": true, "shared_kernel": true, "separate_ways": true,
}

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
			End:   model.Position{Line: lineToLSP(line), Character: startChar + len(text)},
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
			if trigger.Kind() == "domain_listen" {
				line := 0
				if hasLI {
					if whenTok := sc.When(); whenTok != nil {
						line, _ = li.LineCol(whenTok.Offset())
					}
				}
				col := 0
				if hasLI {
					col = trigger.EventCol(li)
				}
				if trigger.EventIsString() {
					diags = append(diags, deprecatedStringRefDiag(uri, "listens", trigger.EventValue(), line, col))
				} else {
					checkSlugRef(uri, trigger.EventValue(), line, col, &diags, &refs)
				}
			}

			for _, action := range sc.Actions() {
				line := 0
				if hasLI {
					line = action.Line(li)
				}
				switch action.Kind() {
				case "async_action":
					col := 0
					if hasLI {
						col = action.EventCol(li)
					}
					if action.EventIsString() {
						diags = append(diags, deprecatedStringRefDiag(uri, "notifies", action.EventValue(), line, col))
					} else {
						checkSlugRef(uri, action.EventValue(), line, col, &diags, &refs)
					}
				case "sync_action":
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
// edgeRelationshipVerbs (the 8 DDD patterns) is accepted with NO diagnostic —
// endpoint resolution / endpoint-not-bc / self-relationship / redundant lint are
// LATER tasks. The default branch is unreachable via .craft source (the parser's
// isEdgeKeyword gates the same vocabulary, kept in sync by
// TestEdgeVerbVocabulariesInSync); it exists only to make parser/sema drift a
// loud internal error and is unit-tested directly.
func classifyEdgeVerb(uri, verb, left string, leftLine, leftCol int) []model.Diagnostic {
	if edgeRelationshipVerbs[verb] {
		return nil
	}
	startChar := colToLSP(leftCol)
	return []model.Diagnostic{{
		Code:      "craft/sema/unrecognised-edge-verb",
		Message:   fmt.Sprintf("edge verb %q is not a recognised relationship pattern (internal: parser accepted it via isEdgeKeyword but sema doesn't classify it — parser/sema drift, not a user error)", verb),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
			End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + len(left)},
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

// resolveBCRef resolves a context_map endpoint against the workspace symbol table.
// scopeDomain is the owning block's domain scope ("" if unscoped).
// Returns (canonical "domain/name" when a BC, kind, ambiguous).
// kind ∈ {"bc","domain","service","actor",""}; "" means fully unresolved.
func resolveBCRef(ws WorkspaceSymbols, scopeDomain, ref string) (resolved, kind string, ambiguous bool) {
	// 1. Qualified `domain/name`: names a specific BC. If that domain doesn't
	// declare it, it's fully unresolved (not endpoint-not-bc).
	if idx := strings.IndexByte(ref, '/'); idx >= 0 {
		domain, name := ref[:idx], ref[idx+1:]
		if dom, ok := ws.Domains[domain]; ok && domainHasBC(dom, name) {
			return domain + "/" + name, "bc", false
		}
		return "", "", false
	}

	// 2. Bare `name`, scope-first: a BC of the owning block's domain wins.
	if scopeDomain != "" {
		if dom, ok := ws.Domains[scopeDomain]; ok && domainHasBC(dom, ref) {
			return scopeDomain + "/" + ref, "bc", false
		}
	}

	// 3. Bare `name`, ambiguity count across all domains. (ws.BoundedContexts
	// maps to a single owner and hides ambiguity, so it is not used here.)
	owner, count := "", 0
	for domName, dom := range ws.Domains {
		if domainHasBC(dom, ref) {
			owner = domName
			count++
		}
	}
	if count == 1 {
		return owner + "/" + ref, "bc", false
	}
	if count >= 2 {
		return "", "bc", true
	}

	// 4. Bare `name`, not a BC: fall through to domain / service / actor.
	if _, ok := ws.Domains[ref]; ok {
		return ref, "domain", false
	}
	if _, ok := ws.Services[ref]; ok {
		return ref, "service", false
	}
	if _, ok := ws.Actors[ref]; ok {
		return ref, "actor", false
	}
	return "", "", false
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
				End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + len(site.Left)},
			},
		})
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
	if !syntax.EdgeVerbSymmetric(site.Verb) {
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
				End:   model.Position{Line: lineToLSP(site.LeftLine), Character: startChar + len(site.Left)},
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
		End:   model.Position{Line: lineToLSP(line), Character: startChar + len(ref)},
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

// validateServiceAnchors flags a repeated opslevel: or repo: field within a
// single service block (duplicate-service-anchor). Every occurrence after
// the first is reported.
func validateServiceAnchors(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, svc := range file.Services() {
		svcName := ""
		if nameTok := svc.Name(); nameTok != nil {
			svcName = syntax.StringAwareText(*nameTok)
		}
		seenOpsLevel, seenRepo := false, false
		for _, f := range svc.Fields() {
			switch {
			case f.IsOpsLevel():
				if seenOpsLevel {
					diags = append(diags, duplicateAnchorDiag(uri, "opslevel", svcName, f, li, hasLI))
				}
				seenOpsLevel = true
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
			End:   model.Position{Line: lineToLSP(line), Character: startChar + len(field)},
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
		line, col = li.LineCol(keyTok.Offset())
	}
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/duplicate-tag",
		Message:   fmt.Sprintf("use case %q: tag %q is already set; last value wins", ucName, key),
		Severity:  model.SeverityWarning,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + len(key)},
		},
	}
}

func duplicateTagBlockDiag(uri, ucName string, tb syntax.TagsBlock, li green.LineIndex, hasLI bool) model.Diagnostic {
	line, col := 0, 0
	if hasLI {
		if kw := tb.Keyword(); kw != nil {
			line, col = li.LineCol(kw.Offset())
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
			End:   model.Position{Line: lineToLSP(line), Character: startChar + len("tags")},
		},
	}
}
