// Package sema — LintModel: lints that read the merged, resolved document.
// They run after analysis has resolved participants (internal/analysis),
// because the syntax trees LintWorkspace reads cannot see across files.
package sema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// LintModel runs the lints that need the resolved model. Every diagnostic
// carries the use case's SourceURI. The trees and line indexes are for the
// lints that point at a position the model does not keep, such as a tag
// value (Part B); the v2.24 lints read the model only.
func LintModel(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	if doc == nil {
		return nil
	}
	var diags []model.Diagnostic
	diags = append(diags, lintDuplicateScenarioBodies(doc)...)
	diags = append(diags, lintLinks(doc)...)
	diags = append(diags, lintJourneys(doc, perFileTrees, lis)...)
	return diags
}

// ── duplicate-scenario-body ───────────────────────────────────────────────────
// Warning: scenarios of one use case whose step trees are equal (spec §5.2).
// Notes (and, from v2.25, links) are ignored; the body must hold an action.
// The scenarios are grouped by body, and each group of two or more is
// reported once, at the group head: the first scenario with that body, whose
// trigger a fold rewrites. The message names the lines of the other members.

func lintDuplicateScenarioBodies(doc *model.CraftDoc) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range doc.UseCases {
		for _, group := range scenarioBodyGroups(uc.Scenarios) {
			if len(group) < 2 {
				continue
			}
			triggers := make([]model.Trigger, len(group))
			for k, i := range group {
				triggers[k] = uc.Scenarios[i].Trigger
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/lint/duplicate-scenario-body",
				Message:   duplicateBodyMessage(triggers),
				Severity:  model.SeverityWarning,
				SourceURI: uc.SourceURI,
				Range:     triggerRange(triggers[0]),
			})
		}
	}
	return diags
}

// scenarioBodyGroups returns the indexes of the scenarios that hold an
// action, grouped by equal body. Groups are in the order of their heads and
// members in source order.
func scenarioBodyGroups(scenarios []model.Scenario) [][]int {
	var groups [][]int
next:
	for j, sc := range scenarios {
		if len(sc.Actions()) == 0 {
			continue
		}
		for g, group := range groups {
			if stepsEqual(scenarios[group[0]].Steps, sc.Steps) {
				groups[g] = append(group, j)
				continue next
			}
		}
		groups = append(groups, []int{j})
	}
	return groups
}

// duplicateBodyMessage says how to fix a group of scenarios with one body;
// triggers[0] is the group head, where the warning sits. Triggers fold into
// one `listens` trigger when they have the same subject, no store ref (a
// store hand-off names one store, so it cannot join a list) and the same
// schedule. The message gives each such fold, with every item of the
// folded triggers once. When the triggers do not all fold into one, it names
// what tells the remaining triggers apart; with nothing to name, the
// scenarios may differ by site or environment.
func duplicateBodyMessage(triggers []model.Trigger) string {
	var lines []string
	for _, t := range triggers[1:] {
		lines = append(lines, strconv.Itoa(t.Line))
	}
	same := "this scenario and the scenario at line " + lines[0] + " run the same steps"
	if len(lines) > 1 {
		same = "this scenario and the scenarios at lines " + andList(lines) + " run the same steps"
	}

	classes := foldClasses(triggers)
	if len(classes) == 1 {
		return fmt.Sprintf("%s; fold them into one trigger: `%s`", same, foldedTrigger(classes[0]))
	}
	var folds []string
	for _, class := range classes {
		if len(class) < 2 {
			continue
		}
		var at []string
		for _, t := range class {
			at = append(at, strconv.Itoa(t.Line))
		}
		folds = append(folds, fmt.Sprintf("fold the scenarios at lines %s into one trigger: `%s`", andList(at), foldedTrigger(class)))
	}
	diff := ""
	for _, class := range classes[1:] {
		if diff = triggerDifference(classes[0][0], class[0]); diff != "" {
			break
		}
	}
	if len(folds) == 0 {
		if diff != "" {
			return fmt.Sprintf("%s but start %s, so one trigger cannot replace them; check that the steps really are the same, and if they differ, write the difference as an `alt`", same, diff)
		}
		return same + "; if they differ only by site, market or environment, write one scenario with a `sites`/`envs` tag; if the steps really differ, write the difference as an `alt`"
	}
	msg := same + "; " + strings.Join(folds, "; ")
	if diff != "" {
		return fmt.Sprintf("%s; the rest start %s, so one trigger cannot replace them all; check that the steps really are the same, and if they differ, write the difference as an `alt`", msg, diff)
	}
	return msg + "; if the rest differ only by site, market or environment, write one scenario with a `sites`/`envs` tag; if the steps really differ, write the difference as an `alt`"
}

// foldClasses splits a group's triggers into the sets that one trigger can
// replace: `listens` triggers with the same subject, no store ref and the
// same schedule share a set; every other trigger is a set of its own. Sets
// are in the order of their first member.
func foldClasses(triggers []model.Trigger) [][]model.Trigger {
	var classes [][]model.Trigger
	foldable := func(t model.Trigger) bool { return t.Type == lang.TriggerDomainListen && t.Store == nil }
next:
	for _, t := range triggers {
		if foldable(t) {
			for c, class := range classes {
				h := class[0]
				if foldable(h) && h.Context == t.Context && h.ScheduleKind == t.ScheduleKind && h.Schedule == t.Schedule {
					classes[c] = append(class, t)
					continue next
				}
			}
		}
		classes = append(classes, []model.Trigger{t})
	}
	return classes
}

// foldedTrigger writes one `listens` trigger for a fold class: every item
// of its triggers once, in source order, and the shared schedule.
func foldedTrigger(class []model.Trigger) string {
	var items []string
	for _, t := range class {
		for _, ev := range refOrEvents(t) {
			if !slices.Contains(items, ev) {
				items = append(items, ev)
			}
		}
	}
	h := class[0]
	fold := fmt.Sprintf("when %s listens %s", h.Context, strings.Join(items, ", "))
	if h.ScheduleKind != "" {
		fold += fmt.Sprintf(" %s %q", h.ScheduleKind, h.Schedule)
	}
	return fold
}

// andList joins "a", "b", "c" as "a, b and c".
func andList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// triggerDifference names what tells two triggers apart, for
// duplicateBodyMessage: the initiator, the schedule, a `receives` operation,
// or a store hand-off. "" when it finds none of these.
func triggerDifference(a, b model.Trigger) string {
	initiator := func(t model.Trigger) string {
		if t.Type == lang.TriggerExternal {
			return t.Actor
		}
		return t.Context
	}
	schedule := func(t model.Trigger) string {
		if t.ScheduleKind == "" {
			return "no schedule"
		}
		return fmt.Sprintf("%s %q", t.ScheduleKind, t.Schedule)
	}
	opText := func(t model.Trigger) string {
		if t.Operation == nil {
			return ""
		}
		return t.Operation.Text
	}
	switch {
	case initiator(a) != initiator(b):
		return fmt.Sprintf("from different initiators (`%s` and `%s`)", initiator(a), initiator(b))
	case a.ScheduleKind != b.ScheduleKind || a.Schedule != b.Schedule:
		return fmt.Sprintf("on different schedules (`%s` and `%s`)", schedule(a), schedule(b))
	case opText(a) != opText(b):
		return fmt.Sprintf("through different operations (`[%s]` and `[%s]`)", opText(a), opText(b))
	case a.Store != nil || b.Store != nil:
		return "from a store hand-off, which names one store and cannot share a `listens` list"
	}
	return ""
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

// ── links ─────────────────────────────────────────────────────────────────────
// unresolved-use-case (warning: the target may live in a file that was not
// loaded), ambiguous-use-case (error), link-to-former-title, continues-not-last
// and self-link (warnings), spec §15.3. The range is the title string.

func lintLinks(doc *model.CraftDoc) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(uc model.UseCase, l *model.Link, code string, sev model.Severity, msg string) {
		var r model.Range
		if l.Line > 0 {
			start := colToLSP(l.Column)
			r = model.Range{
				Start: model.Position{Line: lineToLSP(l.Line), Character: start},
				End:   model.Position{Line: lineToLSP(l.Line), Character: start + lspLen(strconv.Quote(l.Title))},
			}
		}
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uc.SourceURI, Range: r})
	}
	var walk func(uc model.UseCase, steps []model.Step)
	walk = func(uc model.UseCase, steps []model.Step) {
		for i, st := range steps {
			if st.Block != nil {
				for _, br := range st.Block.Branches {
					walk(uc, br.Steps)
				}
			}
			l := st.Link
			if l == nil || l.Title == "" {
				continue // a missing title is the parser's missing-label
			}
			matches, former := model.ResolveUseCaseTitle(doc, l.Title)
			switch {
			case len(matches) == 0:
				add(uc, l, "craft/sema/unresolved-use-case", model.SeverityWarning,
					fmt.Sprintf("no use case is titled %q in the loaded files; check the title, or load the file that defines it", l.Title))
			case len(matches) > 1:
				add(uc, l, "craft/sema/ambiguous-use-case", model.SeverityError,
					fmt.Sprintf("%q names use cases in %d files; rename one of them: a link names one use case", l.Title, len(matches)))
			case former:
				add(uc, l, "craft/lint/link-to-former-title", model.SeverityWarning,
					fmt.Sprintf("%q is a former title of %q; link to its current title", l.Title, matches[0].Name))
			}
			if len(matches) == 1 && matches[0].Name == uc.Name && matches[0].SourceURI == uc.SourceURI {
				add(uc, l, "craft/lint/self-link", model.SeverityWarning, "this use case links to itself; link to another use case, or remove the link")
			}
			if l.Kind == model.LinkContinues {
				for _, after := range steps[i+1:] {
					if after.Note == nil {
						add(uc, l, "craft/lint/continues-not-last", model.SeverityWarning,
							"`continues` ends this body: the steps after it never run here; move them to the target or use `includes`")
						break
					}
				}
			}
		}
	}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			walk(uc, sc.Steps)
		}
	}
	return diags
}

// ── journeys ──────────────────────────────────────────────────────────────────
// journey-list (warning): a use case whose journey tag lists two or more
// values (spec §16). journey-unconnected (hint): a member of a journey with
// two or more members that is the end of no spine edge (spec §17.3). Both
// point at the journey tag's first value.

func lintJourneys(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(uc model.UseCase, code string, sev model.Severity, msg string) {
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uc.SourceURI,
			Range: journeyValueRange(uc, perFileTrees, lis)})
	}
	for _, uc := range doc.UseCases {
		if len(uc.TagValues[model.TagJourney]) > 1 {
			add(uc, "craft/lint/journey-list", model.SeverityWarning,
				"a use case belongs to one journey; BuildJourney uses the first value")
		}
	}
	for _, slug := range journey.Slugs(doc) {
		j := journey.Build(doc, slug)
		if len(j.Members) < 2 {
			continue
		}
		connected := make([]bool, len(j.Members))
		for _, e := range j.Edges {
			connected[e.From], connected[e.To] = true, true
		}
		for k, m := range j.Members {
			if !connected[k] {
				add(doc.UseCases[m.UseCase], "craft/lint/journey-unconnected", model.SeverityHint,
					fmt.Sprintf("nothing in the model connects this use case to the rest of `%s`: add the notifies/listens pair, a continues link, or move it to its own journey", slug))
			}
		}
	}
	return diags
}

// journeyValueRange is the range of the first value of uc's journey tag: the
// use case is found in its file's tree by its title line. The zero range when
// the tree or the tag is not there.
func journeyValueRange(uc model.UseCase, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) model.Range {
	root, ok := perFileTrees[uc.SourceURI]
	li, hasLI := lis[uc.SourceURI]
	if !ok || !hasLI {
		return model.Range{}
	}
	for _, decl := range syntax.AsFile(root).UseCases() {
		if decl.Line(li) != uc.Line {
			continue
		}
		for _, tb := range decl.TagsBlocks() {
			for _, ts := range tb.Tags() {
				if k := ts.Key(); k == nil || k.Text() != model.TagJourney {
					continue
				}
				rs := ts.ValueRanges()
				if len(rs) == 0 {
					return model.Range{}
				}
				sl, sc := li.LineCol16(rs[0].Start)
				el, ec := li.LineCol16(rs[0].End)
				return model.Range{
					Start: model.Position{Line: lineToLSP(sl), Character: colToLSP(sc)},
					End:   model.Position{Line: lineToLSP(el), Character: colToLSP(ec)},
				}
			}
		}
	}
	return model.Range{}
}
