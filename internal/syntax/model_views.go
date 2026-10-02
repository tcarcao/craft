package syntax

import "github.com/tcarcao/craft/v2/internal/lang"

// View accessors used to build the document model (internal/analysis). They
// answer positional, token-level questions so that model building never has to
// index into a node's flat token slice itself.

// ExternalSubjectText returns the subject of an external trigger, read by
// position: significant element 0, which may be a keyword such as `user` or
// `cron` rather than an identifier, or a multi-token qualified ref.
func (t TriggerDecl) ExternalSubjectText() string {
	elems := significantElements(t.node)
	if len(elems) == 0 {
		return ""
	}
	return refAwareText(elems[0])
}

// ExternalVerbText returns the verb of an external trigger: the token right
// after the subject, whose index depends on the subject's token span.
func (t TriggerDecl) ExternalVerbText() string {
	tokens := t.Tokens()
	if i := tokenIndexAt(significantElements(t.node), 1); i < len(tokens) {
		return tokens[i].Text()
	}
	return ""
}

// EventRef returns the event slot's ref text when a domain-listen trigger's
// event is written as a typed ref, and "" for the quoted `listens "X"` form.
func (t TriggerDecl) EventRef() string {
	if elems := significantElements(t.node); len(elems) >= 3 {
		return refIfWrapped(elems[2])
	}
	return ""
}

// ObjectRef returns the object slot's ref text (the target of a sync action,
// the event of an async action) when it is written as a typed ref, and ""
// for a quoted string or a plain name. Other action kinds return "".
func (a ActionDecl) ObjectRef() string {
	switch a.Kind() {
	case lang.ActionSync, lang.ActionAsync:
		if elems := significantElements(a.node); len(elems) >= 3 {
			return refIfWrapped(elems[2])
		}
	}
	return ""
}

// DescriptionConnector returns the action's connector word, whose rule
// differs by kind: a sync action takes `to` or `for` right after its target;
// return and internal actions take any connector word right after their
// structural slots (the `to` of `returns to X` is part of the target, never a
// connector). Otherwise it falls back to the node's `to` keyword child, except
// for a return action, which then has none.
func (a ActionDecl) DescriptionConnector() string {
	tokens := a.Tokens()
	i := slotEndIndex(a)
	switch a.Kind() {
	case lang.ActionSync:
		if i < len(tokens) {
			tok := tokens[i]
			if tok.Kind() == SyntaxKindKwTo || (tok.Kind() == SyntaxKindIdent && tok.Text() == "for") {
				return tok.Text()
			}
		}
	case lang.ActionReturn, lang.ActionInternal:
		if i < len(tokens) && isConnectorWord(tokens[i].Text()) {
			return tokens[i].Text()
		}
		if a.Kind() == lang.ActionReturn {
			return ""
		}
	}
	return a.ConnectorValue()
}

// IsIncomplete reports whether the action line is too broken to model: it has
// no subject (the first token is an error token) or a subject but no verb.
func (a ActionDecl) IsIncomplete() bool {
	tokens := a.Tokens()
	if len(tokens) > 0 && tokens[0].Kind() == SyntaxKindError {
		return true
	}
	return len(tokens) == 1
}

// IsFlow reports whether the component is a flow chain (`A > B > C`) rather
// than a single component.
func (c ArchComponent) IsFlow() bool {
	if c.isZero() {
		return false
	}
	for _, tok := range c.node.Tokens() {
		if tok.Kind() == SyntaxKindGT {
			return true
		}
	}
	return false
}

// ArchFlowStep is one link of a flow component chain.
type ArchFlowStep struct {
	Name      string
	Modifiers []ArchModifierValue
}

// ArchModifierValue is a modifier's key and its unquoted value.
type ArchModifierValue struct {
	Key, Value string
}

// FlowSteps returns the links of a flow component chain in source order. A
// link's name is its first identifier; modifiers attach to the current link.
func (c ArchComponent) FlowSteps() []ArchFlowStep {
	if c.isZero() {
		return nil
	}
	var steps []ArchFlowStep
	var cur ArchFlowStep
	flush := func() {
		if cur.Name != "" {
			steps = append(steps, cur)
		}
		cur = ArchFlowStep{}
	}
	for _, child := range c.node.Children() {
		switch ch := child.(type) {
		case SyntaxToken:
			switch ch.Kind() {
			case SyntaxKindGT:
				flush()
			case SyntaxKindIdent:
				if cur.Name == "" {
					cur.Name = ch.Text()
				}
			}
		case SyntaxNode:
			if ch.Kind() != SyntaxKindArchModifier {
				continue
			}
			toks := ch.Tokens()
			if len(toks) == 0 {
				continue
			}
			mod := ArchModifierValue{Key: toks[0].Text()}
			for j, tok := range toks {
				if tok.Kind() == SyntaxKindColon && j+1 < len(toks) {
					mod.Value = stringAwareText(toks[j+1])
					break
				}
			}
			cur.Modifiers = append(cur.Modifiers, mod)
		}
	}
	flush()
	return steps
}
