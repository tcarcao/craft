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

	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

// LintWorkspace runs style and consistency checks across all workspace files.
// It accepts the per-file syntax tree map and the merged workspace symbol table.
// Returns diagnostics with SourceURI populated so callers can route each
// finding to the correct file.
func LintWorkspace(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols) []craft.Diagnostic {
	var diags []craft.Diagnostic
	diags = append(diags, lintDeadEvents(perFileTrees)...)
	diags = append(diags, lintUnusedActors(perFileTrees, ws)...)
	diags = append(diags, lintEventPastTense(perFileTrees)...)
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

func lintDeadEvents(perFileTrees map[string]syntax.SyntaxNode) []craft.Diagnostic {
	type pubSite struct {
		uri, event string
		line, col  int
		tokenLen   int
	}
	published := map[string]pubSite{}
	consumed := map[string]bool{}

	for uri, tree := range perFileTrees {
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
									line:     action.Line(),
									col:      action.EventCol(),
									tokenLen: eventTokenLen(ev, action.EventIsString()),
								}
							}
						}
					}
				}
			}
		}
	}

	var diags []craft.Diagnostic
	for event, site := range published {
		if !consumed[event] {
			startChar := colToLSP(site.col)
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/dead-event",
				Message:  fmt.Sprintf("event %q is published but never consumed", event),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: lineToLSP(site.line), Character: startChar},
					End:   craft.Position{Line: lineToLSP(site.line), Character: startChar + site.tokenLen},
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

func lintUnusedActors(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols) []craft.Diagnostic {
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

	var diags []craft.Diagnostic
	for name, sym := range ws.Actors {
		if !used[name] {
			startChar := colToLSP(sym.Column)
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/unused-actor",
				Message:  fmt.Sprintf("actor %q is defined but never used as a trigger subject", name),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: lineToLSP(sym.Line), Character: startChar},
					End:   craft.Position{Line: lineToLSP(sym.Line), Character: startChar + len(name)},
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

func lintEventPastTense(perFileTrees map[string]syntax.SyntaxNode) []craft.Diagnostic {
	reported := map[string]bool{}
	var diags []craft.Diagnostic

	check := func(event, uri string, line, col int, isString bool) {
		if event == "" || reported[event] {
			return
		}
		reported[event] = true
		if !pastTenseRe.MatchString(strings.ToLower(event)) {
			startChar := colToLSP(col)
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/lint/event-not-past-tense",
				Message:  fmt.Sprintf("event %q does not appear to use past tense", event),
				Severity: craft.SeverityWarning,
				Range: craft.Range{
					Start: craft.Position{Line: lineToLSP(line), Character: startChar},
					End:   craft.Position{Line: lineToLSP(line), Character: startChar + eventTokenLen(event, isString)},
				},
				SourceURI: uri,
			})
		}
	}

	for uri, tree := range perFileTrees {
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				trigger := sc.Trigger()
				whenTok := sc.When()
				triggerLine := 0
				if whenTok != nil {
					_ = whenTok // line not available without LineIndex; triggerLine stays 0
				}
				if k := trigger.Kind(); k == "event" || k == "domain_listen" {
					check(trigger.EventValue(), uri, triggerLine, trigger.EventCol(), trigger.EventIsString())
				}
				for _, action := range sc.Actions() {
					if action.Kind() == "async_action" {
						check(action.EventValue(), uri, action.Line(), action.EventCol(), action.EventIsString())
					}
				}
			}
		}
	}
	return diags
}
