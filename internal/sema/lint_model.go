// Package sema — LintModel: lints that read the merged, resolved document.
// They run after analysis has resolved participants (internal/analysis),
// because the syntax trees LintWorkspace reads cannot see across files.
package sema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// LintModel runs the lints that need the resolved model. Every diagnostic
// carries the use case's SourceURI. The trees and line indexes are for the
// lints that point at a position the model does not keep, such as a tag
// value (Part B); the v2.24 lints read the model only.
func LintModel(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	_, _ = perFileTrees, lis
	if doc == nil {
		return nil
	}
	var diags []model.Diagnostic
	diags = append(diags, lintDuplicateScenarioBodies(doc)...)
	return diags
}

// ── duplicate-scenario-body ───────────────────────────────────────────────────
// Warning: two scenarios of one use case whose step trees are equal (spec
// §5.2). Notes (and, from v2.25, links) are ignored; the body must hold an
// action. The range is the second scenario's trigger subject. A scenario is
// reported once, against the first earlier scenario it repeats.

func lintDuplicateScenarioBodies(doc *model.CraftDoc) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range doc.UseCases {
		for j := 1; j < len(uc.Scenarios); j++ {
			b := uc.Scenarios[j]
			if len(b.Actions()) == 0 {
				continue
			}
			for i := 0; i < j; i++ {
				a := uc.Scenarios[i]
				if !stepsEqual(a.Steps, b.Steps) {
					continue
				}
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/duplicate-scenario-body",
					Message:   duplicateBodyMessage(a.Trigger, b.Trigger),
					Severity:  model.SeverityWarning,
					SourceURI: uc.SourceURI,
					Range:     triggerRange(b.Trigger),
				})
				break
			}
		}
	}
	return diags
}

func duplicateBodyMessage(a, b model.Trigger) string {
	if a.Type == lang.TriggerDomainListen && b.Type == lang.TriggerDomainListen && a.Context == b.Context {
		items := append(refOrEvents(a), refOrEvents(b)...)
		return fmt.Sprintf("the two scenarios run the same steps; fold them into one trigger: `when %s listens %s`", a.Context, strings.Join(items, ", "))
	}
	return "the two scenarios run the same steps; if they differ only by site, market or environment, write one scenario with a `sites`/`envs` tag; if the steps really differ, write the difference as an `alt`"
}

// refOrEvents is a listens trigger's items as an author writes them: a ref
// bare, a free-text event (one with a space) quoted.
func refOrEvents(t model.Trigger) []string {
	var out []string
	for _, ev := range t.EventList() {
		if strings.ContainsAny(ev, " \t") {
			ev = fmt.Sprintf("%q", ev)
		}
		out = append(out, ev)
	}
	return out
}

// triggerRange spans the trigger's subject, or the `when` keyword when the
// trigger has no subject column (an event trigger).
func triggerRange(t model.Trigger) model.Range {
	name := t.Context
	if t.Type == lang.TriggerExternal {
		name = t.Actor
	}
	start := colToLSP(t.Column)
	length := lspLen(name)
	if t.Column == 0 || name == "" {
		length = 4
	}
	return model.Range{
		Start: model.Position{Line: lineToLSP(t.Line), Character: start},
		End:   model.Position{Line: lineToLSP(t.Line), Character: start + length},
	}
}

// stepsEqual compares two step lists for duplicate-scenario-body: same
// shape; actions equal on Type, Context, Verb, TargetContext, Event,
// (every item of a list), Connector, Phrase and Operation.Text; blocks equal on kind and labels.
// Notes are skipped on both sides.
func stepsEqual(a, b []model.Step) bool {
	a, b = withoutNotes(a), withoutNotes(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		switch {
		case x.Action != nil && y.Action != nil:
			if !actionsEqual(*x.Action, *y.Action) {
				return false
			}
		case x.Block != nil && y.Block != nil:
			if x.Block.Kind != y.Block.Kind || len(x.Block.Branches) != len(y.Block.Branches) {
				return false
			}
			for k := range x.Block.Branches {
				bx, by := x.Block.Branches[k], y.Block.Branches[k]
				if bx.Label != by.Label || !stepsEqual(bx.Steps, by.Steps) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func withoutNotes(steps []model.Step) []model.Step {
	var out []model.Step
	for _, st := range steps {
		if st.Action != nil || st.Block != nil {
			out = append(out, st)
		}
	}
	return out
}

func actionsEqual(x, y model.Action) bool {
	opText := func(o *model.Operation) string {
		if o == nil {
			return ""
		}
		return o.Text
	}
	// EventList, not Event: two `notifies one of` lists that share only
	// their first item are different steps.
	return x.Type == y.Type && x.Context == y.Context && x.Verb == y.Verb && x.TargetContext == y.TargetContext &&
		slices.Equal(x.EventList(), y.EventList()) && x.Connector == y.Connector && x.Phrase == y.Phrase && opText(x.Operation) == opText(y.Operation)
}
