package linter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tcarcao/craft/internal/parser"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// LintResult is a single lint finding.
type LintResult struct {
	File     string
	Line     int
	Severity Severity
	Message  string
}

// Rule is the interface all lint rules implement.
type Rule interface {
	// Name returns the rule identifier (e.g. "dead-event").
	Name() string
	// Check runs the rule against a merged model and returns findings.
	Check(model *parser.DSLModel) []LintResult
}

var rules []Rule

func init() {
	rules = []Rule{
		deadEventRule{},
		unusedActorRule{},
		undefinedContextRule{},
		undefinedServiceContextRule{},
		orphanedActionRule{},
		eventPastTenseRule{},
	}
}

// Lint runs all registered rules against the merged model.
func Lint(files []string, model *parser.DSLModel) []LintResult {
	var results []LintResult
	for _, r := range rules {
		results = append(results, r.Check(model)...)
	}
	return results
}

// ── dead-event ────────────────────────────────────────────────────────────────

type deadEventRule struct{}

func (deadEventRule) Name() string { return "dead-event" }

func (deadEventRule) Check(model *parser.DSLModel) []LintResult {
	type pub struct{ line int }
	published := map[string]pub{}
	consumed := map[string]bool{}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeEvent || sc.Trigger.Type == parser.TriggerTypeDomainListen {
				consumed[sc.Trigger.Event] = true
			}
			for _, a := range sc.Actions {
				if a.Type == parser.ActionTypeAsync {
					if _, seen := published[a.Event]; !seen {
						published[a.Event] = pub{line: a.Line}
					}
				}
			}
		}
	}

	var results []LintResult
	for event, p := range published {
		if !consumed[event] {
			results = append(results, LintResult{
				Line:     p.line,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("event %q is published but never consumed", event),
			})
		}
	}
	return results
}

// ── unused-actor ──────────────────────────────────────────────────────────────

type unusedActorRule struct{}

func (unusedActorRule) Name() string { return "unused-actor" }

func (unusedActorRule) Check(model *parser.DSLModel) []LintResult {
	used := map[string]bool{}
	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeExternal {
				used[sc.Trigger.Actor] = true
			}
		}
	}

	var results []LintResult
	for _, actor := range model.Actors {
		if !used[actor.Name] {
			results = append(results, LintResult{
				Line:     actor.Line,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("actor %q is defined but never used as a trigger", actor.Name),
			})
		}
	}
	return results
}

// ── undefined-context ─────────────────────────────────────────────────────────

type undefinedContextRule struct{}

func (undefinedContextRule) Name() string { return "undefined-context" }

func (undefinedContextRule) Check(model *parser.DSLModel) []LintResult {
	if len(model.Domains) == 0 {
		return nil
	}
	defined := definedContextSet(model)
	reported := map[string]bool{}
	var results []LintResult

	report := func(name string, line int) {
		if name == "" || reported[name] || defined[name] {
			return
		}
		reported[name] = true
		results = append(results, LintResult{
			Line:     line,
			Severity: SeverityError,
			Message:  fmt.Sprintf("bounded context %q is referenced in a use case but not defined in any domain block", name),
		})
	}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeDomainListen {
				report(sc.Trigger.Domain, 0)
			}
			for _, a := range sc.Actions {
				report(a.Domain, a.Line)
				report(a.TargetDomain, a.Line)
			}
		}
	}
	return results
}

// ── undefined-service-context ─────────────────────────────────────────────────

type undefinedServiceContextRule struct{}

func (undefinedServiceContextRule) Name() string { return "undefined-service-context" }

func (undefinedServiceContextRule) Check(model *parser.DSLModel) []LintResult {
	if len(model.Domains) == 0 {
		return nil
	}
	defined := definedContextSet(model)

	var results []LintResult
	for _, svc := range model.Services {
		for _, ctx := range svc.Contexts {
			if !defined[ctx] {
				results = append(results, LintResult{
					Line:     svc.Line,
					Severity: SeverityError,
					Message:  fmt.Sprintf("service %q references context %q which is not defined in any domain block", svc.Name, ctx),
				})
			}
		}
	}
	return results
}

// ── orphaned-action ───────────────────────────────────────────────────────────

type orphanedActionRule struct{}

func (orphanedActionRule) Name() string { return "orphaned-action" }

func (orphanedActionRule) Check(model *parser.DSLModel) []LintResult {
	var results []LintResult
	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			for _, a := range sc.Actions {
				if a.Type == "" {
					results = append(results, LintResult{
						Line:     a.Line,
						Severity: SeverityError,
						Message:  fmt.Sprintf("action on line %d has no type — it may be outside a 'when' block", a.Line),
					})
				}
			}
		}
	}
	return results
}

// ── event-past-tense ──────────────────────────────────────────────────────────

type eventPastTenseRule struct{}

func (eventPastTenseRule) Name() string { return "event-past-tense" }

var pastTenseRe = regexp.MustCompile(`(?i)\b\w+(ed|en)\b`)

func (eventPastTenseRule) Check(model *parser.DSLModel) []LintResult {
	reported := map[string]bool{}
	var results []LintResult

	check := func(event string, line int) {
		if event == "" || reported[event] {
			return
		}
		reported[event] = true
		if !pastTenseRe.MatchString(strings.ToLower(event)) {
			results = append(results, LintResult{
				Line:     line,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("event %q does not appear to use past tense", event),
			})
		}
	}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeEvent || sc.Trigger.Type == parser.TriggerTypeDomainListen {
				check(sc.Trigger.Event, 0)
			}
			for _, a := range sc.Actions {
				if a.Type == parser.ActionTypeAsync {
					check(a.Event, a.Line)
				}
			}
		}
	}
	return results
}

// ── helpers ───────────────────────────────────────────────────────────────────

func definedContextSet(model *parser.DSLModel) map[string]bool {
	set := map[string]bool{}
	for _, d := range model.Domains {
		for _, bc := range d.BoundedContexts {
			set[bc] = true
		}
	}
	return set
}
