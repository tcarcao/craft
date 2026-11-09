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

// validateActorBodies checks the body of each actor of one file (v2.37):
// `ai:` is one of ml, genai or agentic (craft/sema/invalid-ai-kind), and a
// property is written once (craft/sema/duplicate-actor-property; the last
// value wins). Unknown keys are the parser's warning.
func validateActorBodies(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(code string, sev model.Severity, r model.Range, msg string) {
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uri, Range: r})
	}
	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil || !a.HasBody() {
			continue
		}
		seen := map[string]bool{}
		for _, p := range a.Properties() {
			if !p.Known() {
				continue // craft/syntax/unknown-actor-property
			}
			key := p.Key()
			if seen[key] {
				add("craft/sema/duplicate-actor-property", model.SeverityWarning, tokenDiagRange(p.KeyToken(), li, hasLI),
					fmt.Sprintf("actor %s: `%s:` is already set; the last value wins", nameTok.Text(), key))
			}
			seen[key] = true
			if key == lang.ActorAI && p.ValueToken() != nil && !slices.Contains(lang.AIKinds(), p.Value()) {
				add("craft/sema/invalid-ai-kind", model.SeverityError, tokenDiagRange(p.ValueToken(), li, hasLI),
					fmt.Sprintf("actor %s: AI kind %q is not one of %s (ml: a trained model; genai: a generative model; agentic: an agent that acts)",
						nameTok.Text(), p.Value(), strings.Join(lang.AIKinds(), ", ")))
			}
		}
	}
	return diags
}

// ── conflicting-ai-kind ───────────────────────────────────────────────────────
// Warning: one actor is declared again with another `ai:` (or with one,
// after a declaration without). The first declaration in sorted file order
// is the actor's, as for display names, so the later value is reported.

func lintAIKinds(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	type decl struct{ ai, uri string }
	var diags []model.Diagnostic
	declared := map[string]decl{} // actor name → its first declaration
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, a := range syntax.AsFile(perFileTrees[uri]).Actors() {
			nameTok := a.Name()
			if nameTok == nil {
				continue
			}
			name := nameTok.Text()
			ai, _ := a.Property(lang.ActorAI)
			prev, again := declared[name]
			if !again {
				declared[name] = decl{ai, uri}
				continue
			}
			p, ok := a.PropertyDecl(lang.ActorAI)
			if !ok || prev.ai == ai {
				continue
			}
			msg := fmt.Sprintf("actor %s is first declared with `ai: %s` (%s), which the model keeps; use the same kind here or remove this one",
				name, prev.ai, prev.uri)
			if prev.ai == "" {
				msg = fmt.Sprintf("actor %s is first declared without `ai:` (%s), so the model keeps it unclassified; move `ai: %s` there or remove this one",
					name, prev.uri, ai)
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/lint/conflicting-ai-kind",
				Message:   msg,
				Severity:  model.SeverityWarning,
				SourceURI: uri,
				Range:     tokenDiagRange(p.ValueToken(), li, true),
			})
		}
	}
	return diags
}
