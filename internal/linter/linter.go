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

// Lint runs all lint rules against a merged model and returns findings.
// The files slice is used to attribute cross-file results; pass the ordered list of parsed files.
func Lint(files []string, model *parser.DSLModel) []LintResult {
	var results []LintResult
	results = append(results, checkDeadEvents(model)...)
	results = append(results, checkUnusedActors(model)...)
	results = append(results, checkUndefinedContexts(model)...)
	results = append(results, checkUndefinedServiceContexts(model)...)
	results = append(results, checkEventPastTense(model)...)
	return results
}

// checkDeadEvents warns when an event is published (notifies) but never consumed (listens).
func checkDeadEvents(model *parser.DSLModel) []LintResult {
	published := map[string]bool{}
	consumed := map[string]bool{}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeEvent || sc.Trigger.Type == parser.TriggerTypeDomainListen {
				consumed[sc.Trigger.Event] = true
			}
			for _, a := range sc.Actions {
				if a.Type == parser.ActionTypeAsync {
					published[a.Event] = true
				}
			}
		}
	}

	var results []LintResult
	for event := range published {
		if !consumed[event] {
			results = append(results, LintResult{
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("event %q is published but never consumed", event),
			})
		}
	}
	return results
}

// checkUnusedActors warns when an actor is defined but never appears as a trigger.
func checkUnusedActors(model *parser.DSLModel) []LintResult {
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
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("actor %q is defined but never used as a trigger", actor.Name),
			})
		}
	}
	return results
}

// definedContextSet builds the set of all bounded contexts declared in domain blocks.
func definedContextSet(model *parser.DSLModel) map[string]bool {
	set := map[string]bool{}
	for _, d := range model.Domains {
		for _, bc := range d.BoundedContexts {
			set[bc] = true
		}
	}
	return set
}

// checkUndefinedContexts errors when a bounded context referenced in a use case is not defined.
func checkUndefinedContexts(model *parser.DSLModel) []LintResult {
	if len(model.Domains) == 0 {
		return nil
	}
	defined := definedContextSet(model)

	reported := map[string]bool{}
	var results []LintResult

	report := func(name string) {
		if name == "" || reported[name] {
			return
		}
		if !defined[name] {
			reported[name] = true
			results = append(results, LintResult{
				Severity: SeverityError,
				Message:  fmt.Sprintf("bounded context %q is referenced in a use case but not defined in any domain block", name),
			})
		}
	}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeDomainListen {
				report(sc.Trigger.Domain)
			}
			for _, a := range sc.Actions {
				report(a.Domain)
				report(a.TargetDomain)
			}
		}
	}
	return results
}

// checkUndefinedServiceContexts errors when a service lists a context not defined in any domain.
func checkUndefinedServiceContexts(model *parser.DSLModel) []LintResult {
	if len(model.Domains) == 0 {
		return nil
	}
	defined := definedContextSet(model)

	var results []LintResult
	for _, svc := range model.Services {
		for _, ctx := range svc.Contexts {
			if !defined[ctx] {
				results = append(results, LintResult{
					Severity: SeverityError,
					Message:  fmt.Sprintf("service %q references context %q which is not defined in any domain block", svc.Name, ctx),
				})
			}
		}
	}
	return results
}

// pastTenseRe matches a word ending in common past-tense suffixes.
var pastTenseRe = regexp.MustCompile(`(?i)\b\w+(ed|en)\b`)

// checkEventPastTense warns when an event name doesn't appear to use past tense.
func checkEventPastTense(model *parser.DSLModel) []LintResult {
	reported := map[string]bool{}
	var results []LintResult

	check := func(event string) {
		if event == "" || reported[event] {
			return
		}
		reported[event] = true
		if !pastTenseRe.MatchString(strings.ToLower(event)) {
			results = append(results, LintResult{
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("event %q does not appear to use past tense", event),
			})
		}
	}

	for _, uc := range model.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == parser.TriggerTypeEvent || sc.Trigger.Type == parser.TriggerTypeDomainListen {
				check(sc.Trigger.Event)
			}
			for _, a := range sc.Actions {
				if a.Type == parser.ActionTypeAsync {
					check(a.Event)
				}
			}
		}
	}
	return results
}
