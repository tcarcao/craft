// Package sema implements the semantic analysis layer for Craft DSL.
// S3: actor namespace + duplicate-name rule.
// S4: domain namespace + duplicate-name for domains + cross-kind-name-reuse warning.
// S5: service namespace + ResolutionMap population + unresolved-reference +
//
//	invalid-reference-target rules + AnalyzeWorkspace for cross-file resolution.
//
// S6: use_case namespace + duplicate-use-case-name rule + extend unresolved-reference
//
//	to use-case trigger subjects and action parties.
//
// S8: exposure collection + invalid-exposure-target validation (to:→actor,
//
//	through:→service, contexts:→domain/service).
package sema

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// lineToLSP converts a 1-based source line to a 0-based LSP line number.
// Returns 0 for any non-positive input.
func lineToLSP(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
}

// colToLSP converts a 1-based source column to a 0-based LSP character offset.
// Returns 0 for any non-positive input.
//
// The input must already be a UTF-16 column — every Column field in this
// package is produced by LineIndex.LineCol16, never LineCol. LSP measures
// Position.Character in UTF-16 code units, so a byte column here would place
// diagnostics and definitions to the left of their real position on any line
// containing a multi-byte character.
func colToLSP(col int) int {
	if col <= 0 {
		return 0
	}
	return col - 1
}

// lspLen is the width of a name in LSP character units (UTF-16 code units),
// for computing a range End from its Start. len(s) would be a byte count.
func lspLen(s string) int { return int(green.UTF16Len(s)) }

// ActorSymbol holds collected information about an actor declaration.
type ActorSymbol struct {
	Name string
	Type string // "user", "system", "service", or custom type
	// Line is the 1-based source line of the actor name.
	Line int
	// Column is the 1-based column where the actor name starts.
	Column int
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
	// Column is the 1-based column where the domain name starts.
	Column int
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int
	// IsGrouped is true when the domain was declared inside a domains { } block.
	IsGrouped bool
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
	// Column is the 1-based column where the service name starts.
	Column int
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int
	// IsGrouped is true when the service was declared inside a services { } block.
	IsGrouped bool
	// URI is the file that contains this declaration.
	URI string
}

// UseCaseSymbol holds collected information about a use_case declaration.
type UseCaseSymbol struct {
	Name string
	// Line is the 1-based source line of the use_case keyword.
	Line int
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int
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

// BCPosition holds the source position of a bounded context declaration.
type BCPosition struct {
	// Line is the 1-based line of the bounded context name token.
	Line int
	// Column is the 1-based column of the bounded context name token.
	Column int
	// URI is the file that contains this bounded context.
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
	// Column is the 1-based column of the reference token.
	Column int
	// Length is the reference's width in UTF-16 units when it differs from
	// the name's: the `cron` / `every` keyword stands for the CRON actor. 0
	// means the width of Name.
	Length int
	// Keyword is true when the written token is not the name (the `cron` /
	// `every` keyword for CRON), so a rename must not rewrite it.
	Keyword bool
}

// Symbols is the output of the symbol-collection pass for a single file.
type Symbols struct {
	Actors    []ActorSymbol
	Domains   []DomainSymbol
	Services  []ServiceSymbol
	UseCases  []UseCaseSymbol
	Exposures []ExposureSymbol
	// Surfaces holds the file's surface declarations, checked against the
	// workspace in AnalyzeWorkspace.
	Surfaces []SurfaceSymbol
	// UseCaseRefs collects all name references from within use-case bodies
	// so AnalyzeWorkspace can resolve them cross-file.
	UseCaseRefs []UseCaseRef
	// BCPositions maps bounded-context name → its declaration position.
	// Populated by AnalyzeFile so MergeWorkspaceSymbols can propagate it.
	BCPositions map[string]BCPosition
	// SlugRefs collects well-formed kind-prefixed node-slug references
	// (domain:/bc:/service:) so AnalyzeWorkspace can attempt best-effort
	// cross-file resolution (craft/sema/unresolved-ref-local, Task 7).
	SlugRefs []SlugRefSite
	// ContextMapEdges collects context_map edges so AnalyzeWorkspace can
	// resolve each endpoint to a bounded context cross-file (Task 4).
	ContextMapEdges []ContextMapEdgeSite
	// GlossaryRelations collects glossary term relations so AnalyzeWorkspace
	// can resolve each term node's BC part to a bounded context cross-file
	// (Task A3).
	GlossaryRelations []GlossaryRelationSite
}

// WorkspaceSymbols merges per-file symbol tables for cross-file resolution.
type WorkspaceSymbols struct {
	// Actors maps name → symbol across all workspace files.
	Actors map[string]ActorSymbol
	// Domains maps name → symbol across all workspace files.
	Domains map[string]DomainSymbol
	// BoundedContexts maps bounded-context name → owning DomainSymbol.
	BoundedContexts map[string]DomainSymbol
	// BCPositions maps bounded-context name → its source position.
	BCPositions map[string]BCPosition
	// Services maps name → symbol across all workspace files.
	Services map[string]ServiceSymbol
	// UseCases maps name → symbol across all workspace files.
	UseCases map[string]UseCaseSymbol
	// DataStores maps data-store name → the service that owns it.
	DataStores map[string]string
	// ContextOwners maps a bounded context's canonical "<domain>/<bc>" →
	// the service that deploys it.
	ContextOwners map[string]string
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
	Kind lang.ParticipantKind // actor, domain, bounded context or service
	// One of the following is populated depending on Kind:
	Actor   *ActorSymbol
	Domain  *DomainSymbol
	Service *ServiceSymbol
	// BCLine, BCColumn, BCURI carry the position of the bounded context's own
	// declaration token (inside its parent domain body). Set when Kind is lang.ParticipantBoundedContext.
	BCLine   int
	BCColumn int
	BCURI    string
}

// AnalyzeFile collects symbols from a single file's syntax tree and runs
// validation rules for constructs present through S5 (actors + domains + services).
// It accepts the lossless syntax tree produced by syntax.Parse and reads it
// directly via typed view methods — no lower/AST conversion is needed.
// Returns the symbol table and any semantic diagnostics.
// li is the LineIndex for resolving byte offsets to line/col; pass it for accurate positions.
func AnalyzeFile(uri string, tree syntax.SyntaxNode, li ...green.LineIndex) (syms Symbols, diags []model.Diagnostic) {
	var lineIdx green.LineIndex
	hasLI := len(li) > 0
	if hasLI {
		lineIdx = li[0]
	}
	file := syntax.AsFile(tree)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("craft sema: panic recovered",
				"tier", "sema",
				"uri", uri,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
			)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/sema/sema-panic",
				Message:  fmt.Sprintf("internal sema error: %v", r),
				Severity: model.SeverityError,
			})
		}
	}()

	seenActors := make(map[string]ActorSymbol)
	seenDomains := make(map[string]DomainSymbol)
	seenServices := make(map[string]ServiceSymbol)

	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil {
			continue
		}
		actorType := a.ActorTypeValue()
		actorLine, actorCol := 0, 0
		if hasLI {
			actorLine, actorCol = lineIdx.LineCol16(nameTok.Offset())
		}
		sym := ActorSymbol{
			Name:   nameTok.Text(),
			Type:   actorType,
			Line:   actorLine,
			Column: actorCol,
			URI:    uri,
		}
		if prev, dup := seenActors[nameTok.Text()]; dup {
			startChar := colToLSP(sym.Column)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("actor %q already declared (first seen at line %d)", nameTok.Text(), prev.Line),
				Severity: model.SeverityError,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(sym.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(sym.Line), Character: startChar + lspLen(nameTok.Text())},
				},
			})
			continue
		}
		seenActors[nameTok.Text()] = sym
		syms.Actors = append(syms.Actors, sym)
	}

	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		// nameContent is the unquoted semantic value (identical to
		// nameTok.Text() for a bare ident; strips quotes for a quoted
		// name). Used everywhere the name is read as CONTENT — symbol
		// name, dup-key, message. The Range End below intentionally keeps
		// using nameTok.Text()'s raw length, since that must match the
		// on-disk token span (quotes included) for the diagnostic to
		// underline the right source range.
		nameContent := syntax.StringAwareText(*nameTok)
		var bcNames []string
		for _, bc := range d.BoundedContexts() {
			bcTok := bc.Name()
			if bcTok != nil {
				bcNames = append(bcNames, bcTok.Text())
			}
		}
		domLine, domCol := 0, 0
		if hasLI {
			domLine, domCol = lineIdx.LineCol16(nameTok.Offset())
		}
		domEndLine := 0
		if hasLI {
			domEndLine = d.EndLine(lineIdx)
		}
		sym := DomainSymbol{
			Name:            nameContent,
			BoundedContexts: bcNames,
			Line:            domLine,
			Column:          domCol,
			EndLine:         domEndLine,
			IsGrouped:       d.IsGrouped(),
			URI:             uri,
		}
		if prev, dup := seenDomains[nameContent]; dup {
			startChar := colToLSP(sym.Column)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("domain %q already declared (first seen at line %d)", nameContent, prev.Line),
				Severity: model.SeverityError,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(sym.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(sym.Line), Character: startChar + lspLen(nameTok.Text())},
				},
			})
			continue
		}
		seenDomains[nameContent] = sym
		syms.Domains = append(syms.Domains, sym)

		// Collect bounded context positions for each BC in this domain.
		for _, bc := range d.BoundedContexts() {
			bcTok := bc.Name()
			if bcTok == nil {
				continue
			}
			if syms.BCPositions == nil {
				syms.BCPositions = make(map[string]BCPosition)
			}
			bcLine, bcCol := 0, 0
			if hasLI {
				bcLine, bcCol = lineIdx.LineCol16(bcTok.Offset())
			}
			syms.BCPositions[bcTok.Text()] = BCPosition{
				Line:   bcLine,
				Column: bcCol,
				URI:    uri,
			}
		}
	}

	for _, s := range file.Services() {
		nameTok := s.Name()
		if nameTok == nil {
			continue
		}
		// nameContent is the unquoted semantic value — see the matching
		// comment in the domains loop above for why the Range End below
		// still uses nameTok.Text()'s raw (possibly-quoted) length.
		nameContent := syntax.StringAwareText(*nameTok)
		svcLine, svcCol := 0, 0
		if hasLI {
			svcLine, svcCol = lineIdx.LineCol16(nameTok.Offset())
		}
		svcEndLine := 0
		if hasLI {
			svcEndLine = s.EndLine(lineIdx)
		}
		sym := ServiceSymbol{
			Name:       nameContent,
			Contexts:   s.Contexts(),
			DataStores: s.DataStores(),
			Language:   s.Language(),
			Line:       svcLine,
			Column:     svcCol,
			EndLine:    svcEndLine,
			IsGrouped:  s.IsGrouped(),
			URI:        uri,
		}
		if prev, dup := seenServices[nameContent]; dup {
			startChar := colToLSP(sym.Column)
			diags = append(diags, model.Diagnostic{
				Code:     "craft/sema/duplicate-name",
				Message:  fmt.Sprintf("service %q already declared (first seen at line %d)", nameContent, prev.Line),
				Severity: model.SeverityError,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(sym.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(sym.Line), Character: startChar + lspLen(nameTok.Text())},
				},
			})
			continue
		}
		seenServices[nameContent] = sym
		syms.Services = append(syms.Services, sym)
	}

	// Cross-kind name reuse check (Q4b, Q23).
	for name, actorSym := range seenActors {
		if domSym, clash := seenDomains[name]; clash {
			startChar := colToLSP(domSym.Column)
			diags = append(diags, model.Diagnostic{
				Code: "craft/sema/cross-kind-name-reuse",
				Message: fmt.Sprintf(
					"%q is declared as both an actor (line %d) and a domain (line %d); "+
						"consider renaming to avoid confusion",
					name, actorSym.Line, domSym.Line,
				),
				Severity: model.SeverityWarning,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(domSym.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(domSym.Line), Character: startChar + lspLen(name)},
				},
			})
		}
	}

	// Use case collection + duplicate-use-case-name rule (Q23).
	seenUseCases := make(map[string]UseCaseSymbol)
	for _, uc := range file.UseCases() {
		titleTok := uc.Title()
		kwTok := uc.Keyword()
		if titleTok == nil {
			continue
		}
		name := uc.Name() // unquoted title text (Bug 8a fix — titleTok.Text() is now raw, quoted)
		ucLine := 0
		if hasLI && kwTok != nil {
			ucLine, _ = lineIdx.LineCol(kwTok.Offset())
		}
		ucEndLine := 0
		if hasLI {
			ucEndLine = uc.EndLine(lineIdx)
		}
		sym := UseCaseSymbol{Name: name, Line: ucLine, EndLine: ucEndLine, URI: uri}
		if prev, dup := seenUseCases[name]; dup {
			diags = append(diags, model.Diagnostic{
				Code: "craft/sema/duplicate-use-case-name",
				Message: fmt.Sprintf(
					"use_case %q already declared (first seen at line %d)", name, prev.Line,
				),
				Severity: model.SeverityError,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(ucLine)},
					End:   model.Position{Line: lineToLSP(ucLine), Character: lspLen(name)},
				},
			})
			continue
		}
		seenUseCases[name] = sym
		syms.UseCases = append(syms.UseCases, sym)

		// Collect reference sites from all scenarios for cross-resolution.
		for _, sc := range uc.Scenarios() {
			trigger := sc.Trigger()
			whenTok := sc.When()
			triggerLine := 0
			if hasLI && whenTok != nil {
				triggerLine, _ = lineIdx.LineCol(whenTok.Offset())
			}
			// Trigger subject (actor or domain name).
			if actor := trigger.ActorName(); actor != "" {
				col := 0
				if hasLI {
					col = trigger.ActorCol(lineIdx)
				}
				syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
					Name:   actor,
					Line:   triggerLine,
					Column: col,
				})
			}
			if kw := trigger.ScheduleKeyword(); kw != nil {
				// A scheduled trigger runs as CRON, written as its keyword.
				col := 0
				if hasLI {
					col = trigger.ActorCol(lineIdx)
				}
				syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
					Name:    lang.SchedulerActor,
					Line:    triggerLine,
					Column:  col,
					Length:  lspLen(kw.Text()),
					Keyword: true,
				})
			}
			if ctx := trigger.ContextName(); ctx != "" {
				col := 0
				if hasLI {
					col = trigger.ActorCol(lineIdx) // ActorCol works for both trigger kinds
				}
				syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
					Name:   ctx,
					Line:   triggerLine,
					Column: col,
				})
			}
			// Action parties.
			for _, action := range sc.Actions() {
				actionLine := 0
				if hasLI {
					actionLine = action.Line(lineIdx)
				}
				if subj := action.SubjectName(); subj != "" {
					col := 0
					if hasLI {
						col = action.SubjectCol(lineIdx)
					}
					syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
						Name:   subj,
						Line:   actionLine,
						Column: col,
					})
				}
				if target := action.TargetName(); target != "" {
					col := 0
					if hasLI {
						col = action.TargetCol(lineIdx)
					}
					syms.UseCaseRefs = append(syms.UseCaseRefs, UseCaseRef{
						Name:   target,
						Line:   actionLine,
						Column: col,
					})
				}
			}
		}
	}

	// Exposure collection (S8). Exposures have no duplicate-name rule (two
	// exposures named "default" in separate files is valid; the design
	// intentionally allows one exposure per API gateway).
	for _, e := range file.Exposures() {
		nameTok := e.Name()
		if nameTok == nil {
			continue
		}
		expLineNum := 0
		if hasLI {
			expLineNum = e.Line(lineIdx)
		}
		syms.Exposures = append(syms.Exposures, ExposureSymbol{
			Name:     nameTok.Text(),
			To:       e.To(),
			Contexts: e.Contexts(),
			Through:  e.Through(),
			Line:     expLineNum,
			URI:      uri,
		})
	}

	// Task 7 (Slice E): local well-formedness validation — slug shape,
	// context_map edge endpoint kinds, deprecated quoted notifies/listens
	// forms, and duplicate service catalog_ref:/repo: anchors. Well-formed
	// kind-prefixed slug refs are also collected into syms.SlugRefs for
	// AnalyzeWorkspace's cross-file unresolved-ref-local check.
	ucDiags, ucRefs := validateUseCaseRefs(uri, file, lineIdx, hasLI)
	diags = append(diags, ucDiags...)
	syms.SlugRefs = append(syms.SlugRefs, ucRefs...)

	cmDiags, cmRefs := validateContextMapEdges(uri, file, lineIdx, hasLI)
	diags = append(diags, cmDiags...)
	syms.SlugRefs = append(syms.SlugRefs, cmRefs...)

	// Task 4: collect context_map edges for cross-file endpoint resolution in
	// AnalyzeWorkspace (per-file has no WorkspaceSymbols; see SlugRefs pattern).
	syms.ContextMapEdges = append(syms.ContextMapEdges, collectContextMapEdgeSites(uri, file, lineIdx, hasLI)...)

	// Task A3: collect glossary term relations for cross-file BC resolution
	// in AnalyzeWorkspace, mirroring ContextMapEdges above.
	syms.GlossaryRelations = append(syms.GlossaryRelations, collectGlossaryRelationSites(uri, file, lineIdx, hasLI)...)

	diags = append(diags, validateServiceAnchors(uri, file, lineIdx, hasLI)...)
	diags = append(diags, validateUseCaseTags(uri, file, lineIdx, hasLI)...)
	diags = append(diags, validateSchedules(uri, file, lineIdx, hasLI)...)
	diags = append(diags, validateEventLists(uri, file, lineIdx, hasLI)...)

	syms.Surfaces = collectSurfaces(uri, file, lineIdx, hasLI)
	diags = append(diags, validateSurfaces(uri, file, lineIdx, hasLI)...)

	return syms, diags
}

// MergeWorkspaceSymbols merges per-file symbol tables into a workspace-level
// index. Cross-file duplicate service declarations emit a warning diagnostic
// with SourceURI set to the second file that declares the same name.
//
// Files are merged in ascending URI order and the FIRST declaration of a name
// wins, for every symbol kind. Both halves matter, and for the same reason the
// lint rules walk sorted (see sortedTreeURIs): these maps are keyed by bare
// name, so a name declared in two files has exactly one winner, and the winner
// is what every downstream consumer reports as the declaration site — the URI
// and line of an unused-actor finding, the owning domain a bare bounded-context
// reference resolves to, the position go-to-definition jumps to. Ranging over
// perFile directly let Go's map seed pick that winner, so those answers changed
// between two runs over identical input. Sorting alone is not enough: with
// last-write-wins the winner would be the alphabetically LAST file, which
// contradicts the rule the service branch below already documents.
func MergeWorkspaceSymbols(perFile map[string]Symbols) (WorkspaceSymbols, []model.Diagnostic) {
	ws := WorkspaceSymbols{
		Actors:          make(map[string]ActorSymbol),
		Domains:         make(map[string]DomainSymbol),
		BoundedContexts: make(map[string]DomainSymbol),
		BCPositions:     make(map[string]BCPosition),
		Services:        make(map[string]ServiceSymbol),
		UseCases:        make(map[string]UseCaseSymbol),
		DataStores:      make(map[string]string),
		ContextOwners:   make(map[string]string),
	}
	uris := make([]string, 0, len(perFile))
	for uri := range perFile {
		uris = append(uris, uri)
	}
	sort.Strings(uris)

	var diags []model.Diagnostic
	for _, uri := range uris {
		syms := perFile[uri]
		for _, a := range syms.Actors {
			if _, dup := ws.Actors[a.Name]; !dup {
				ws.Actors[a.Name] = a
			}
		}
		for _, d := range syms.Domains {
			if prev, dup := ws.Domains[d.Name]; !dup {
				ws.Domains[d.Name] = d
			} else if prev.URI != d.URI {
				// Same rule as a service declared in two files: the domain
				// symbol keeps the first declaration, so report the later one
				// instead of silently dropping it. (A repeat within one file is
				// already a duplicate-name error.)
				startChar := colToLSP(d.Column)
				diags = append(diags, model.Diagnostic{
					Code: "craft/sema/duplicate-name",
					Message: fmt.Sprintf(
						"domain %q already declared in %s (line %d)",
						d.Name, prev.URI, prev.Line,
					),
					Severity:  model.SeverityWarning,
					SourceURI: d.URI,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(d.Line), Character: startChar},
						End:   model.Position{Line: lineToLSP(d.Line), Character: startChar + lspLen(d.Name)},
					},
				})
			}
			for _, bc := range d.BoundedContexts {
				if _, dup := ws.BoundedContexts[bc]; !dup {
					ws.BoundedContexts[bc] = d
				}
			}
		}
		for _, s := range syms.Services {
			if prev, dup := ws.Services[s.Name]; dup && prev.URI != s.URI {
				startChar := colToLSP(s.Column)
				diags = append(diags, model.Diagnostic{
					Code: "craft/sema/duplicate-name",
					Message: fmt.Sprintf(
						"service %q already declared in %s (line %d)",
						s.Name, prev.URI, prev.Line,
					),
					Severity:  model.SeverityWarning,
					SourceURI: s.URI,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(s.Line), Character: startChar},
						End:   model.Position{Line: lineToLSP(s.Line), Character: startChar + lspLen(s.Name)},
					},
				})
				// Keep the first declaration in the map so resolution is stable.
				continue
			}
			ws.Services[s.Name] = s
		}
		for _, uc := range syms.UseCases {
			if _, dup := ws.UseCases[uc.Name]; !dup {
				ws.UseCases[uc.Name] = uc
			}
		}
		for name, pos := range syms.BCPositions {
			if _, dup := ws.BCPositions[name]; !dup {
				ws.BCPositions[name] = pos
			}
		}
	}
	// Service membership needs every domain merged first, to resolve each
	// contexts entry to its canonical bounded context.
	diags = append(diags, indexServiceMembers(&ws, perFile, uris)...)
	return ws, diags
}

// AnalyzeWorkspace runs cross-file resolution for all files. It populates a
// ResolutionMap and returns workspace-level diagnostics (unresolved references,
// etc.). The caller should first call MergeWorkspaceSymbols and pass its
// outputs; merge-phase diagnostics (cross-file duplicates) are returned there.
func AnalyzeWorkspace(perFile map[string]Symbols, ws WorkspaceSymbols) (ResolutionMap, []model.Diagnostic) {
	rm := ResolutionMap{
		ServiceContexts: make(map[serviceContextKey]DomainSymbol),
		UseCaseRefs:     make(map[useCaseRefKey]UseCaseRefTarget),
	}
	var diags []model.Diagnostic

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
					startChar := colToLSP(svc.Column)
					diags = append(diags, model.Diagnostic{
						Code: "craft/sema/unresolved-reference",
						Message: fmt.Sprintf(
							"service %q references context %q which is not declared in any domain",
							svc.Name, ctxName,
						),
						Severity:  model.SeverityError,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(svc.Line), Character: startChar},
							End:   model.Position{Line: lineToLSP(svc.Line), Character: startChar + lspLen(svc.Name)},
						},
					})
					continue
				}
				rm.ServiceContexts[key] = domSym
			}
		}
	}

	// Resolve use-case reference sites (S6).
	// Emit craft/sema/unresolved-reference only when the workspace has at least one
	// declared symbol: if no declarations exist, every name is unknown by definition
	// and flooding diagnostics would be unhelpful on a brand-new file.
	wsHasSymbols := len(ws.Actors) > 0 || len(ws.Domains) > 0 || len(ws.Services) > 0
	for uri, syms := range perFile {
		for _, ref := range syms.UseCaseRefs {
			key := useCaseRefKey{URI: uri, Line: ref.Line, Name: ref.Name}
			p, ambiguous := ResolveParticipant(ws, "", ref.Name)
			if ambiguous {
				// LintWorkspace reports it (ambiguous-bc). Navigation still
				// jumps to one deterministic declaration.
				if d, ok := ws.BoundedContexts[ref.Name]; ok {
					rm.UseCaseRefs[key] = ws.bcTarget(d, ref.Name)
				}
				continue
			}
			if p.Kind != "" {
				if target, ok := ws.useCaseRefTarget(p); ok {
					rm.UseCaseRefs[key] = target
				}
				continue
			}
			if !wsHasSymbols {
				continue
			}
			startChar := colToLSP(ref.Column)
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/unresolved-reference",
				Message:   fmt.Sprintf("use-case participant %q is not declared as a bounded context, domain, service, actor or data store", ref.Name),
				Severity:  model.SeverityWarning,
				SourceURI: uri,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(ref.Line), Character: startChar},
					End:   model.Position{Line: lineToLSP(ref.Line), Character: startChar + refWidth(ref)},
				},
			})
		}
	}

	// Validate exposure targets (S8, Q23: craft/sema/invalid-exposure-target).
	// Only fire when the referenced name is known but is of the wrong kind.
	// Unknown names are silently ignored (many exposure targets may be
	// architecture components that aren't modelled as DSL declarations).
	for uri, syms := range perFile {
		for _, exp := range syms.Exposures {
			expRange := model.Range{
				Start: model.Position{Line: lineToLSP(exp.Line)},
				End:   model.Position{Line: lineToLSP(exp.Line), Character: lspLen(exp.Name)},
			}
			// `to:` must name actors.
			for _, name := range exp.To {
				if _, isActor := ws.Actors[name]; isActor {
					continue // correct — actor reference
				}
				if _, isDomain := ws.Domains[name]; isDomain {
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `to:` target %q is a domain, not an actor — use actor declarations for `to:` targets", exp.Name, name),
						Severity:  model.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
					continue
				}
				if _, isService := ws.Services[name]; isService {
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `to:` target %q is a service, not an actor — use actor declarations for `to:` targets", exp.Name, name),
						Severity:  model.SeverityError,
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
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `through:` target %q is an actor, not a service — use service declarations for `through:` targets", exp.Name, name),
						Severity:  model.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
					continue
				}
				if _, isDomain := ws.Domains[name]; isDomain {
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `through:` target %q is a domain, not a service — use service declarations for `through:` targets", exp.Name, name),
						Severity:  model.SeverityError,
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
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/invalid-exposure-target",
						Message:   fmt.Sprintf("exposure %q: `contexts:` target %q is an actor — use domain or service declarations for `contexts:` targets", exp.Name, name),
						Severity:  model.SeverityError,
						SourceURI: uri,
						Range:     expRange,
					})
				}
				// Unknown name: skip.
			}
		}
	}

	// Task 7: best-effort local resolution for kind-prefixed node-slug refs
	// (domain:/bc:/service: — term: is skipped; craft tracks no term
	// declarations, so that resolution is hub-only). Unresolved ⇒ warning
	// only, never an error — the hub is authoritative for cross-context
	// resolution. Guarded by wsHasSymbols for the same reason as the
	// use-case ref check above: a brand-new file-set with no declarations
	// at all shouldn't flood every ref as "unresolved".
	if wsHasSymbols {
		for uri, syms := range perFile {
			for _, sr := range syms.SlugRefs {
				var resolved bool
				switch sr.Kind {
				case "domain":
					_, resolved = ws.Domains[sr.Name]
				case "bc":
					_, resolved = ws.BoundedContexts[sr.Name]
				case "service":
					_, resolved = ws.Services[sr.Name]
				}
				if resolved {
					continue
				}
				startChar := colToLSP(sr.Col)
				diags = append(diags, model.Diagnostic{
					Code: "craft/sema/unresolved-ref-local",
					Message: fmt.Sprintf(
						"%s slug %q names no %s declared in this file-set; the hub is authoritative for cross-context resolution",
						sr.Kind, sr.Text, sr.Kind,
					),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(sr.Line), Character: startChar},
						End:   model.Position{Line: lineToLSP(sr.Line), Character: startChar + lspLen(sr.Text)},
					},
				})
			}
		}
	}

	// Task 4: resolve context_map edge endpoints to bounded contexts. Each site
	// carries its own SourceURI; the broken-fixture harness filters by URI.
	// Task 5: seen spans the whole workspace (all files), so a symmetric pair
	// declared once per file, or split across files, still dedupes correctly.
	seen := map[string]bool{}
	for _, syms := range perFile {
		for _, site := range syms.ContextMapEdges {
			diags = append(diags, relationshipEdgeDiags(ws, site)...)
			diags = append(diags, redundantRelationshipDiag(ws, site, seen)...)
		}
	}

	// Task A3: resolve glossary term-node endpoints to bounded contexts,
	// mirroring the context_map edge loop above.
	// Task A4: glossary-redundant and glossary-conflicting-relation lints,
	// keyed on the sorted canonical term-node pair; maps span the whole
	// workspace (mirroring redundantRelationshipDiag's seen placement above)
	// so duplicates/conflicts split across files still dedupe/fire correctly.
	glossarySeenVerb := map[string]bool{}
	glossaryVerbsForPair := map[string]map[string]bool{}
	glossaryConflictReported := map[string]bool{}
	for _, syms := range perFile {
		for _, site := range syms.GlossaryRelations {
			diags = append(diags, glossaryRelationDiags(ws, site)...)
			diags = append(diags, glossaryLintDiags(ws, site, glossarySeenVerb, glossaryVerbsForPair, glossaryConflictReported)...)
		}
	}

	diags = append(diags, resolveSurfaces(perFile, ws)...)

	return rm, diags
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

// refWidth is the UTF-16 width of the token a reference was written as.
func refWidth(ref UseCaseRef) int {
	if ref.Length > 0 {
		return ref.Length
	}
	return lspLen(ref.Name)
}
