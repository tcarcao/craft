// Package sema — lint rules for Craft DSL (v2 AST, B5).
// These rules mirror the heuristics in internal/linter/ but operate on
// the v2 AST (internal/ast) and the workspace symbol table rather than
// the ANTLR parser.DSLModel. The old linter package remains intact until
// ANTLR is deleted in S12.
package sema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/pkg/craft"
)

// LintWorkspace runs style and consistency checks across all workspace files.
// It accepts the per-file AST map and the merged workspace symbol table.
// Returns diagnostics with SourceURI populated so callers can route each
// finding to the correct file.
func LintWorkspace(perFileASTs map[string]*ast.File, ws WorkspaceSymbols) []craft.Diagnostic {
	var diags []craft.Diagnostic
	diags = append(diags, lintDeadEvents(perFileASTs)...)
	diags = append(diags, lintUnusedActors(perFileASTs, ws)...)
	diags = append(diags, lintEventPastTense(perFileASTs)...)
	return diags
}

// ── dead-event ────────────────────────────────────────────────────────────────
// Warning: an event published via `notifies` but never consumed by any
// `when … listens` or `when … "event"` trigger in any workspace file.

func lintDeadEvents(perFileASTs map[string]*ast.File) []craft.Diagnostic {
	type pubSite struct {
		uri  string
		line int
	}
	published := map[string]pubSite{}
	consumed := map[string]bool{}

	for uri, f := range perFileASTs {
		for _, uc := range f.UseCases {
			for _, sc := range uc.Scenarios {
				// Consume: event-trigger or domain-listen trigger
				if sc.Trigger.TriggerType == "event" || sc.Trigger.TriggerType == "domain_listen" {
					if sc.Trigger.Event != "" {
						consumed[sc.Trigger.Event] = true
					}
				}
				// Publish: async actions (notifies)
				for _, a := range sc.Actions {
					if a.ActionType == "async_action" && a.Event != "" {
						if _, seen := published[a.Event]; !seen {
							published[a.Event] = pubSite{uri: uri, line: a.Line}
						}
					}
				}
			}
		}
	}

	var diags []craft.Diagnostic
	for event, site := range published {
		if !consumed[event] {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/dead-event",
				Message:  fmt.Sprintf("event %q is published but never consumed", event),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(site.line)},
					End:   craft.Position{Line: ast.LineToLSP(site.line)},
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

func lintUnusedActors(perFileASTs map[string]*ast.File, ws WorkspaceSymbols) []craft.Diagnostic {
	if len(ws.Actors) == 0 {
		return nil
	}

	used := map[string]bool{}
	for _, f := range perFileASTs {
		for _, uc := range f.UseCases {
			for _, sc := range uc.Scenarios {
				if sc.Trigger.TriggerType == "external" && sc.Trigger.Actor != "" {
					used[sc.Trigger.Actor] = true
				}
			}
		}
	}

	var diags []craft.Diagnostic
	for name, sym := range ws.Actors {
		if !used[name] {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/unused-actor",
				Message:  fmt.Sprintf("actor %q is defined but never used as a trigger subject", name),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(sym.Line)},
					End:   craft.Position{Line: ast.LineToLSP(sym.Line), Character: len(name)},
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

func lintEventPastTense(perFileASTs map[string]*ast.File) []craft.Diagnostic {
	reported := map[string]bool{}
	var diags []craft.Diagnostic

	check := func(event, uri string, line int) {
		if event == "" || reported[event] {
			return
		}
		reported[event] = true
		if !pastTenseRe.MatchString(strings.ToLower(event)) {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/event-not-past-tense",
				Message:  fmt.Sprintf("event %q does not appear to use past tense", event),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(line)},
					End:   craft.Position{Line: ast.LineToLSP(line)},
				},
				SourceURI: uri,
			})
		}
	}

	for uri, f := range perFileASTs {
		for _, uc := range f.UseCases {
			for _, sc := range uc.Scenarios {
				if sc.Trigger.TriggerType == "event" || sc.Trigger.TriggerType == "domain_listen" {
					check(sc.Trigger.Event, uri, sc.Trigger.Line)
				}
				for _, a := range sc.Actions {
					if a.ActionType == "async_action" {
						check(a.Event, uri, a.Line)
					}
				}
			}
		}
	}
	return diags
}
