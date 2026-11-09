package lsp

import (
	"regexp"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// Actor bodies in the editor (v2.37): hover on an actor shows its AI kind
// and owner, hover on `ai` and `owner` explains the key, completion offers
// the body keys and the AI kinds after `ai:`, and an actor with a body
// folds and spans its body in the outline.

// withActorBody adds an actor's AI kind and owner to a hover, one line each.
func withActorBody(hover, ai, owner string) string {
	if ai != "" {
		hover += "\nai: " + ai + " (" + aiKindMeaning(ai) + ")"
	}
	if owner != "" {
		hover += "\nowner: " + owner
	}
	return hover
}

// aiKindMeaning explains one AI kind in a few words.
func aiKindMeaning(kind string) string {
	switch kind {
	case "ml":
		return "a trained model"
	case "genai":
		return "a generative model"
	case "agentic":
		return "an agent that acts"
	}
	return "not a known AI kind"
}

// actorBodyHover is the hover of a declaration's own body: the AI kind as
// written when it is valid, and the owner.
func actorBodyHover(hover string, a syntax.ActorDecl) string {
	ai, _ := a.Property(lang.ActorAI)
	owner, _ := a.Property(lang.ActorOwner)
	return withActorBody(hover, ai, owner)
}

// modelActorHover adds the AI kind and owner of the actor the model keeps
// for name (the first declaration).
func modelActorHover(hover string, doc *model.CraftDoc, name string) string {
	if a := doc.ActorByName(name); a != nil {
		return withActorBody(hover, a.AI, a.Owner)
	}
	return hover
}

// actorKeyHover explains the keys of an actor body.
func actorKeyHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, a := range file.Actors() {
		for _, p := range a.Properties() {
			if !within(p.KeyToken(), offset) {
				continue
			}
			switch p.Key() {
			case lang.ActorAI:
				return "ai: the kind of AI system this actor is: ml (a trained model), genai (a generative model) or agentic (an agent that acts). The use cases that call it count towards AI coverage. Optional", true
			case lang.ActorOwner:
				return "owner: who runs the system, such as a team or a vendor. Optional", true
			}
			return "", false
		}
	}
	return "", false
}

// actorFoldingRanges folds each actor body that spans lines.
func actorFoldingRanges(file syntax.File, li green.LineIndex) []protocol.FoldingRange {
	var out []protocol.FoldingRange
	for _, a := range file.Actors() {
		if !a.HasBody() {
			continue
		}
		if start, end := a.Line(li), a.EndLine(li); start > 0 && end > start {
			out = append(out, protocol.FoldingRange{
				StartLine: lspLine(start),
				EndLine:   lspLine(end),
				Kind:      protocol.RegionFoldingRange,
			})
		}
	}
	return out
}

// actorAIRe matches an actor's `ai:` typed up to its value, on its own line
// or right after a one-line body's `{`.
var actorAIRe = regexp.MustCompile(`(^|\{)\s*ai:\s*\w*$`)

// actorBodyCompletions are the lines of an actor body.
func actorBodyCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	return &protocol.CompletionList{Items: []protocol.CompletionItem{
		{Label: "ai:", Kind: protocol.CompletionItemKindField,
			Detail:     "optional: the kind of AI system, " + strings.Join(lang.AIKinds(), ", "),
			InsertText: "ai: ${1|" + strings.Join(lang.AIKinds(), ",") + "|}", InsertTextFormat: snip},
		{Label: "owner:", Kind: protocol.CompletionItemKindField, Detail: "optional: who runs the system",
			InsertText: `owner: "${1:team}"`, InsertTextFormat: snip},
		{Label: "note", Kind: protocol.CompletionItemKindKeyword, Detail: "a remark about the actor",
			InsertText: `note "${1:text}"`, InsertTextFormat: snip},
	}}
}

// aiKindCompletions are ml, genai and agentic.
func aiKindCompletions() *protocol.CompletionList {
	var items []protocol.CompletionItem
	for _, k := range lang.AIKinds() {
		items = append(items, protocol.CompletionItem{Label: k, Kind: protocol.CompletionItemKindEnumMember, Detail: aiKindMeaning(k)})
	}
	return &protocol.CompletionList{Items: items}
}
