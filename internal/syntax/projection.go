package syntax

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/pkg/craft"
)

// ProjectFromTree projects a lossless red SyntaxNode tree into a *craft.CraftDoc
// using typed views directly — no lower.go dependency.
//
// li is reserved for future position-aware projection (Task 10); it is currently
// unused but kept on the public signature so callers wire LineIndex through.
func ProjectFromTree(root SyntaxNode, li green.LineIndex) *craft.CraftDoc {
	_ = li
	if root == (SyntaxNode{}) {
		return &craft.CraftDoc{UseCases: []craft.UseCase{}}
	}
	file := AsFile(root)
	doc := &craft.CraftDoc{UseCases: []craft.UseCase{}}

	// Services
	doc.Services = projectServicesFromViews(file)

	// Actors
	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil {
			continue
		}
		typeVal := a.ActorTypeValue()
		if typeVal == "" {
			continue // malformed — skip
		}
		doc.Actors = append(doc.Actors, craft.Actor{
			Name: nameTok.Text(),
			Type: craft.ActorType(typeVal),
			Line: 0,
		})
	}

	// Domains
	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		var bcNames []string
		for _, bc := range d.BoundedContexts() {
			bcTok := bc.Name()
			if bcTok != nil {
				bcNames = append(bcNames, bcTok.Text())
			}
		}
		doc.Domains = append(doc.Domains, craft.Domain{
			Name:            nameTok.Text(),
			BoundedContexts: bcNames,
		})
	}

	// Use cases — maintain global counter for action_N / scenario_N IDs
	counter := 0
	for _, uc := range file.UseCases() {
		titleTok := uc.Title()
		if titleTok == nil {
			continue
		}
		outUC := craft.UseCase{
			Name:      titleTok.Text(),
			Scenarios: []craft.Scenario{},
		}
		for _, sc := range uc.Scenarios() {
			counter++
			scenID := fmt.Sprintf("scenario_%d", counter)
			trigger := projectTriggerFromView(sc.Trigger())
			outSc := craft.Scenario{
				ID:      scenID,
				Trigger: trigger,
				Actions: []craft.Action{},
			}
			for _, action := range sc.Actions() {
				// Mirror lowerAction: two paths increment counter twice and skip the action.
				// Path 1: no subject — first token is SyntaxKindError.
				// Path 2: subject present but no verb — exactly one non-error token.
				tokens := action.Tokens()
				if len(tokens) > 0 && tokens[0].Kind() == SyntaxKindError {
					counter++
					counter++
					continue
				}
				if len(tokens) == 1 {
					counter++
					counter++
					continue
				}
				counter++
				actID := fmt.Sprintf("action_%d", counter)
				outSc.Actions = append(outSc.Actions, projectActionFromView(action, actID))
			}
			outUC.Scenarios = append(outUC.Scenarios, outSc)
		}
		doc.UseCases = append(doc.UseCases, outUC)
	}

	// Architectures
	for _, a := range file.Archs() {
		nameTok := a.Name()
		name := ""
		if nameTok != nil {
			name = nameTok.Text()
		}
		ab := craft.ArchBlock{
			Name:         name,
			Presentation: []craft.Component{},
			Gateway:      []craft.Component{},
		}
		for _, section := range a.Sections() {
			kwTok := section.Keyword()
			if kwTok == nil {
				continue
			}
			components := projectArchComponentsFromView(section.Components())
			switch kwTok.Kind() {
			case SyntaxKindKwPresentation:
				ab.Presentation = append(ab.Presentation, components...)
			case SyntaxKindKwGateway:
				ab.Gateway = append(ab.Gateway, components...)
			}
		}
		doc.Architectures = append(doc.Architectures, ab)
	}

	// Exposures
	for _, e := range file.Exposures() {
		nameTok := e.Name()
		if nameTok == nil {
			continue
		}
		doc.Exposures = append(doc.Exposures, craft.Exposure{
			Name:     nameTok.Text(),
			To:       e.To(),
			Contexts: e.Contexts(),
			Through:  e.Through(),
		})
	}

	return doc
}

func projectTriggerFromView(t TriggerDecl) craft.Trigger {
	kind := t.Kind()
	var actor, context, event, verb, phrase string
	switch kind {
	case "event":
		event = t.EventValue()
	case "domain_listen":
		context = t.ContextName()
		event = t.EventValue()
	default: // external
		// Match lower.go lowerTrigger: read by token position, not by kind.
		// Actor is tokens[0] (may be a keyword like KwUser, not SyntaxKindIdent).
		tokens := t.Tokens()
		if len(tokens) >= 1 {
			actor = tokens[0].Text()
		}
		if len(tokens) >= 2 {
			verb = tokens[1].Text()
		}
		phraseStart := 2
		if len(tokens) > 2 && isConnectorWord(tokens[2].Text()) {
			// connector is consumed (discarded) from phrase, matching lower.go `_ = connector`
			phraseStart = 3
		}
		if len(tokens) > phraseStart {
			var parts []string
			for _, tok := range tokens[phraseStart:] {
				parts = append(parts, tok.Text())
			}
			phrase = strings.Join(parts, " ")
		}
	}
	var description string
	switch kind {
	case "event":
		description = fmt.Sprintf("when %q", event)
	case "domain_listen":
		description = fmt.Sprintf("when %s listens %q", context, event)
	default:
		// Match lower.go: uses Sprintf without TrimSpace, producing trailing space when phrase="".
		description = fmt.Sprintf("when %s %s %s", actor, verb, phrase)
	}
	return craft.Trigger{
		Type:        craft.TriggerType(kind),
		Actor:       actor,
		Verb:        verb,
		Phrase:      phrase,
		Context:     context,
		Event:       event,
		Description: description,
	}
}

func projectActionFromView(a ActionDecl, id string) craft.Action {
	kind := a.Kind()
	subject := a.SubjectName()
	target := a.TargetName()
	event := a.EventValue()
	connector := a.ConnectorValue()
	phrase := a.PhraseText()

	// Build description to match lower.go exactly.
	var verb, description string
	switch kind {
	case "sync_action":
		// Match lower.go lowerAsksAction: connector is at tokens[3] (KwTo or "for" ident).
		// a.ConnectorValue() only returns KwTo; must check tokens for "for" too.
		{
			tokens := a.Tokens()
			i := 3 // after subject, asks, target
			if i < len(tokens) {
				tok := tokens[i]
				if tok.Kind() == SyntaxKindKwTo || (tok.Kind() == SyntaxKindIdent && tok.Text() == "for") {
					connector = tok.Text()
					i++
				}
			}
			// phrase from remaining tokens
			var phraseParts []string
			for _, tok := range tokens[i:] {
				phraseParts = append(phraseParts, tok.Text())
			}
			phrase = strings.Join(phraseParts, " ")
			// description matches lower.go: always include connector field (may be empty, adding trailing space)
			description = subject + " asks " + target + " " + connector
			if phrase != "" {
				description += " " + phrase
			}
		}
	case "async_action":
		// lowerNotifiesAction: fmt.Sprintf("%s notifies %q", subject, event)
		description = fmt.Sprintf("%s notifies %q", subject, event)
	case "return_action":
		// Match lower.go lowerReturnsAction: extract optional connector after `to target`.
		// tokens[0]=subject, tokens[1]=returns, then optional `to <target>`, then optional connector, phrase.
		// Reset connector (a.ConnectorValue() returns `to` from KwTo which is part of target syntax, not a connector).
		connector = ""
		{
			tokens := a.Tokens()
			i := 2
			// skip optional `to <target>`
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindKwTo {
				i += 2 // skip `to` and target
			}
			// optional connector_word after target
			if i < len(tokens) && isConnectorWord(tokens[i].Text()) {
				connector = tokens[i].Text()
				i++
			}
			// phrase from remaining tokens
			var phraseParts []string
			for _, tok := range tokens[i:] {
				phraseParts = append(phraseParts, tok.Text())
			}
			phrase = strings.Join(phraseParts, " ")
			if target != "" {
				description = fmt.Sprintf("%s returns %s to %s", subject, phrase, target)
			} else {
				description = fmt.Sprintf("%s returns %s", subject, phrase)
			}
		}
	default: // internal_action
		verb = a.VerbValue()
		// Match lower.go: isConnectorWord applies to any connector ident (a/an/the/as/to/etc.),
		// not just SyntaxKindKwTo. Extract connector directly from tokens.
		tokens := a.Tokens()
		phraseStart := 2
		if len(tokens) > 2 && isConnectorWord(tokens[2].Text()) {
			connector = tokens[2].Text()
			phraseStart = 3
		}
		var phraseParts []string
		for _, tok := range tokens[phraseStart:] {
			phraseParts = append(phraseParts, tok.Text())
		}
		phrase = strings.Join(phraseParts, " ")
		description = subject + " " + verb
		if connector != "" {
			description += " " + connector
		}
		if phrase != "" {
			description += " " + phrase
		}
	}

	return craft.Action{
		ID:            id,
		Type:          craft.ActionType(kind),
		Context:       subject,
		Verb:          verb,
		TargetContext: target,
		Event:         event,
		Connector:     connector,
		Phrase:        phrase,
		Description:   description,
		Line:          0,
	}
}

// serviceNameTok returns the name token for a service decl — first Ident or String child
// (skipping the `service` keyword itself). Matches lower.go lowerServiceDecl name extraction.
func serviceNameTok(svc ServiceDecl) *SyntaxToken {
	if svc.node == (SyntaxNode{}) {
		return nil
	}
	for _, child := range svc.node.Children() {
		tok, ok := child.(SyntaxToken)
		if !ok {
			continue
		}
		if tok.Kind() == SyntaxKindKwService {
			continue // skip the `service` keyword
		}
		if tok.Kind() == SyntaxKindIdent || tok.Kind() == SyntaxKindString {
			t := tok
			return &t
		}
	}
	return nil
}

func projectServicesFromViews(file File) []craft.Service {
	type entry struct {
		svc    craft.Service
		ctxSet map[string]bool
		dsSet  map[string]bool
	}
	var order []string
	byName := map[string]*entry{}

	for _, svc := range file.Services() {
		nameTok := serviceNameTok(svc)
		if nameTok == nil {
			continue
		}
		name := nameTok.Text()
		e, exists := byName[name]
		if !exists {
			e = &entry{
				svc: craft.Service{
					Name:       name,
					Deployment: craft.DeploymentStrategy{Type: svc.DeploymentType()},
					Line:       0,
				},
				ctxSet: map[string]bool{},
				dsSet:  map[string]bool{},
			}
			byName[name] = e
			order = append(order, name)
		}
		for _, c := range svc.Contexts() {
			if !e.ctxSet[c] {
				e.ctxSet[c] = true
				e.svc.Contexts = append(e.svc.Contexts, c)
			}
		}
		for _, d := range svc.DataStores() {
			if !e.dsSet[d] {
				e.dsSet[d] = true
				e.svc.DataStores = append(e.svc.DataStores, d)
			}
		}
		if e.svc.Language == "" {
			e.svc.Language = svc.Language()
		}
		if e.svc.Deployment.Type == "" {
			e.svc.Deployment.Type = svc.DeploymentType()
		}
		if len(e.svc.Deployment.Rules) == 0 {
			for _, r := range svc.DeploymentRules() {
				e.svc.Deployment.Rules = append(e.svc.Deployment.Rules, craft.DeploymentRule{
					Percentage: r.Percentage,
					Target:     r.Target,
				})
			}
		}
	}

	out := make([]craft.Service, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name].svc)
	}
	return out
}

func projectArchComponentsFromView(comps []ArchComponent) []craft.Component {
	var out []craft.Component
	for _, c := range comps {
		out = append(out, projectArchComponentFromView(c))
	}
	return out
}

func projectArchComponentFromView(c ArchComponent) craft.Component {
	if c.node == (SyntaxNode{}) {
		return craft.Component{Type: craft.ComponentType("simple")}
	}

	// Detect flow vs simple by looking for GT token in the node's tokens.
	// Mirrors lowerArchComponent in lower.go.
	tokens := c.node.Tokens()
	hasGT := false
	for _, tok := range tokens {
		if tok.Kind() == SyntaxKindGT {
			hasGT = true
			break
		}
	}

	if !hasGT {
		return projectSimpleArchComponentFromView(c)
	}
	return projectFlowArchComponentFromView(c)
}

// projectSimpleArchComponentFromView builds a simple Component from an ArchComponent view.
func projectSimpleArchComponentFromView(c ArchComponent) craft.Component {
	nameTok := c.Name()
	name := ""
	if nameTok != nil {
		name = nameTok.Text()
	}
	comp := craft.Component{
		Name: name,
		Type: craft.ComponentType("simple"),
	}
	for _, m := range c.Modifiers() {
		keyTok := m.Key()
		valTok := m.Value()
		key, val := "", ""
		if keyTok != nil {
			key = keyTok.Text()
		}
		if valTok != nil {
			val = valTok.Text()
		}
		comp.Modifiers = append(comp.Modifiers, craft.ComponentModifier{Key: key, Value: val})
	}
	return comp
}

// projectFlowArchComponentFromView builds a flow Component (chain) from an ArchComponent view.
// Mirrors lowerFlowComponent in lower.go.
func projectFlowArchComponentFromView(c ArchComponent) craft.Component {
	var chain []craft.Component
	var currentName string
	var currentMods []craft.ComponentModifier

	flush := func() {
		if currentName == "" {
			return
		}
		chain = append(chain, craft.Component{
			Name:      currentName,
			Type:      craft.ComponentType("simple"),
			Modifiers: currentMods,
		})
		currentName = ""
		currentMods = nil
	}

	for _, child := range c.node.Children() {
		switch ch := child.(type) {
		case SyntaxToken:
			switch ch.Kind() {
			case SyntaxKindGT:
				flush()
			case SyntaxKindIdent:
				if currentName == "" {
					currentName = ch.Text()
				}
			}
		case SyntaxNode:
			if ch.Kind() == SyntaxKindArchModifier {
				toks := ch.Tokens()
				if len(toks) == 0 {
					continue
				}
				key := toks[0].Text()
				var val string
				for j, tok := range toks {
					if tok.Kind() == SyntaxKindColon && j+1 < len(toks) {
						val = toks[j+1].Text()
						break
					}
				}
				currentMods = append(currentMods, craft.ComponentModifier{Key: key, Value: val})
			}
		}
	}
	flush()

	return craft.Component{
		Type:  craft.ComponentType("flow"),
		Chain: chain,
	}
}
