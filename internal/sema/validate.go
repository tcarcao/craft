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

// edgeRealizationVerbs require a `bc:` left endpoint and a `service:` right endpoint.
var edgeRealizationVerbs = map[string]bool{"realized_by": true, "also_realizes": true}

// edgeTermVerbs require `term:` endpoints on both sides.
var edgeTermVerbs = map[string]bool{"same_as": true, "contrasts": true, "distinct_from": true}

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

// validateContextMapEdges checks each context_map edge's endpoint kinds
// against its verb (edge-endpoint-kind) and each endpoint's slug shape
// (malformed-slug). Also returns well-formed slug refs collected for
// cross-file resolution.
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

			leftKind, rightKind := refKindWord(left), refKindWord(right)
			switch {
			case edgeRealizationVerbs[verb]:
				if leftKind != "bc" || rightKind != "service" {
					diags = append(diags, edgeEndpointKindDiag(uri, verb, "a `bc:` left endpoint and a `service:` right endpoint", left, right, leftLine, leftCol))
				}
			case edgeTermVerbs[verb]:
				if leftKind != "term" || rightKind != "term" {
					diags = append(diags, edgeEndpointKindDiag(uri, verb, "`term:` endpoints on both sides", left, right, leftLine, leftCol))
				}
			}
		}
	}
	return diags, refs
}

func edgeEndpointKindDiag(uri, verb, requirement, left, right string, line, col int) model.Diagnostic {
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code:      "craft/sema/edge-endpoint-kind",
		Message:   fmt.Sprintf("edge %q %s %q requires %s", left, verb, right, requirement),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + len(left)},
		},
	}
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
