package syntax

import (
	"fmt"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
)

// ProjectFromTree projects a lossless red SyntaxNode tree into a *model.CraftDoc
// using typed views directly — no lower.go dependency.
func ProjectFromTree(root SyntaxNode, li green.LineIndex, sourceURI ...string) *model.CraftDoc {
	src := ""
	if len(sourceURI) > 0 {
		src = sourceURI[0]
	}
	if root == (SyntaxNode{}) {
		return &model.CraftDoc{UseCases: []model.UseCase{}}
	}
	file := AsFile(root)
	doc := &model.CraftDoc{UseCases: []model.UseCase{}}

	// Services
	doc.Services = projectServicesFromViews(file, li)

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
		actorLine, _ := li.LineCol(nameTok.Offset())
		doc.Actors = append(doc.Actors, model.Actor{
			Name: nameTok.Text(),
			Type: model.ActorType(typeVal),
			Line: actorLine,
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
		doc.Domains = append(doc.Domains, model.Domain{
			Name:            stringAwareText(*nameTok),
			BoundedContexts: bcNames,
		})
	}

	// Use cases — maintain global counter for action_N / scenario_N IDs
	counter := 0
	for _, uc := range file.UseCases() {
		if uc.Title() == nil {
			continue
		}
		ucLine, _ := li.LineCol(uc.Title().Offset())
		outUC := model.UseCase{
			Name:      uc.Name(), // unquoted title text (Bug 8a fix)
			Scenarios: []model.Scenario{},
			Line:      ucLine,
			SourceURI: src,
		}
		for _, sc := range uc.Scenarios() {
			counter++
			scenID := fmt.Sprintf("scenario_%d", counter)
			trigger := projectTriggerFromView(sc.Trigger())
			outSc := model.Scenario{
				ID:      scenID,
				Trigger: trigger,
				Actions: []model.Action{},
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
				outSc.Actions = append(outSc.Actions, projectActionFromView(action, actID, li))
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
		ab := model.ArchBlock{
			Name:         name,
			Presentation: []model.Component{},
			Gateway:      []model.Component{},
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
		doc.Exposures = append(doc.Exposures, model.Exposure{
			Name:     nameTok.Text(),
			To:       e.To(),
			Contexts: e.Contexts(),
			Through:  e.Through(),
		})
	}

	// ContextMap edges
	for _, cm := range file.ContextMaps() {
		for _, e := range cm.Edges() {
			left, verb, right := e.Left(), e.Verb(), e.Right()
			if left == "" || verb == "" || right == "" {
				continue // malformed — skip
			}
			doc.ContextMap = append(doc.ContextMap, model.Edge{
				Left:  left,
				Verb:  verb,
				Right: right,
			})
		}
	}

	return doc
}

func projectTriggerFromView(t TriggerDecl) model.Trigger {
	kind := t.Kind()
	var actor, context, event, verb, phrase, ref string
	switch kind {
	case "event":
		event = t.EventValue()
	case "domain_listen":
		context = t.ContextName()
		event = t.EventValue()
		// ref is populated only when the event was written as a typed ref
		// (Task 4), not the legacy quoted `listens "X"` form.
		if elems := significantElements(t.node); len(elems) >= 3 {
			ref = refIfWrapped(elems[2])
		}
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
		// phrase is the exact raw source substring (Bug 8a fix), not a
		// space-joined reconstruction that would insert spaces into tight
		// punctuation — mirrors ActionDecl.PhraseText().
		phrase = t.PhraseText()
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
	return model.Trigger{
		Type:        model.TriggerType(kind),
		Actor:       actor,
		Verb:        verb,
		Phrase:      phrase,
		Context:     context,
		Event:       event,
		Ref:         ref,
		Description: description,
	}
}

func projectActionFromView(a ActionDecl, id string, li green.LineIndex) model.Action {
	kind := a.Kind()
	subject := a.SubjectName()
	target := a.TargetName()
	event := a.EventValue()
	connector := a.ConnectorValue()
	phrase := a.PhraseText()

	// ref is populated only when the sync_action target or async_action
	// event was written as a typed ref (Task 4), not the legacy quoted
	// `notifies "X"` form or a plain unwrapped name.
	var ref string
	if kind == "sync_action" || kind == "async_action" {
		if elems := significantElements(a.node); len(elems) >= 3 {
			ref = refIfWrapped(elems[2])
		}
	}

	// Build description to match lower.go exactly.
	var verb, description string
	switch kind {
	case "sync_action":
		// Match lower.go lowerAsksAction: connector is at tokens[3] (KwTo or "for" ident).
		// a.ConnectorValue() only returns KwTo; must check tokens for "for" too.
		{
			tokens := a.Tokens()
			// target (tokens index 2) may be a multi-token Ref (Task 4, e.g.
			// bc:re/billing spans 5 flat tokens); skip its actual span rather
			// than assuming exactly one flat token, or the connector/phrase
			// split below would start mid-ref.
			i := 2 // after subject, asks
			if elems := significantElements(a.node); len(elems) > 2 {
				i += elementSpan(elems[2])
			} else {
				i++
			}
			if i < len(tokens) {
				tok := tokens[i]
				if tok.Kind() == SyntaxKindKwTo || (tok.Kind() == SyntaxKindIdent && tok.Text() == "for") {
					connector = tok.Text()
				}
			}
			// phrase is already set from a.PhraseText() above (Bug 8a fix) — the
			// exact raw source substring, not a space-joined reconstruction that
			// would insert spaces into tight punctuation like `(1! & 2!)`.
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
			}
			// phrase is already set from a.PhraseText() above (Bug 8a fix) — see
			// the sync_action case comment.
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
		if len(tokens) > 2 && isConnectorWord(tokens[2].Text()) {
			connector = tokens[2].Text()
		}
		// phrase is already set from a.PhraseText() above (Bug 8a fix) — see
		// the sync_action case comment.
		description = subject + " " + verb
		if connector != "" {
			description += " " + connector
		}
		if phrase != "" {
			description += " " + phrase
		}
	}

	actionLine := 0
	if toks := a.Tokens(); len(toks) > 0 {
		actionLine, _ = li.LineCol(toks[0].Offset())
	}
	return model.Action{
		ID:            id,
		Type:          model.ActionType(kind),
		Context:       subject,
		Verb:          verb,
		TargetContext: target,
		Event:         event,
		Ref:           ref,
		Connector:     connector,
		Phrase:        phrase,
		Description:   description,
		Line:          actionLine,
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

func projectServicesFromViews(file File, li green.LineIndex) []model.Service {
	type entry struct {
		svc    model.Service
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
		// name may be quoted (`service "Order Service" { ... }`); stringAwareText
		// unquotes it (Bug 8a fix) — nameTok.Text() is now the raw source text.
		name := stringAwareText(*nameTok)
		e, exists := byName[name]
		if !exists {
			svcLine, _ := li.LineCol(nameTok.Offset())
			e = &entry{
				svc: model.Service{
					Name:       name,
					Deployment: model.DeploymentStrategy{Type: svc.DeploymentType()},
					Line:       svcLine,
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
		if e.svc.OpsLevel == "" {
			e.svc.OpsLevel = svc.OpsLevel()
		}
		if e.svc.Repo == "" {
			e.svc.Repo = svc.Repo()
		}
		if e.svc.Deployment.Type == "" {
			e.svc.Deployment.Type = svc.DeploymentType()
		}
		if len(e.svc.Deployment.Rules) == 0 {
			for _, r := range svc.DeploymentRules() {
				e.svc.Deployment.Rules = append(e.svc.Deployment.Rules, model.DeploymentRule{
					Percentage: r.Percentage,
					Target:     r.Target,
				})
			}
		}
	}

	out := make([]model.Service, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name].svc)
	}
	return out
}

func projectArchComponentsFromView(comps []ArchComponent) []model.Component {
	var out []model.Component
	for _, c := range comps {
		out = append(out, projectArchComponentFromView(c))
	}
	return out
}

func projectArchComponentFromView(c ArchComponent) model.Component {
	if c.node == (SyntaxNode{}) {
		return model.Component{Type: model.ComponentType("simple")}
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
func projectSimpleArchComponentFromView(c ArchComponent) model.Component {
	nameTok := c.Name()
	name := ""
	if nameTok != nil {
		name = nameTok.Text()
	}
	comp := model.Component{
		Name: name,
		Type: model.ComponentType("simple"),
	}
	for _, m := range c.Modifiers() {
		keyTok := m.Key()
		valTok := m.Value()
		key, val := "", ""
		if keyTok != nil {
			key = keyTok.Text()
		}
		if valTok != nil {
			// Value() may be an Ident, String, or Number token (see ArchModifier.Value
			// doc comment); stringAwareText unquotes String tokens so a quoted
			// modifier value (e.g. Comp[label:"some value"]) projects unquoted.
			val = stringAwareText(*valTok)
		}
		comp.Modifiers = append(comp.Modifiers, model.ComponentModifier{Key: key, Value: val})
	}
	return comp
}

// projectFlowArchComponentFromView builds a flow Component (chain) from an ArchComponent view.
// Mirrors lowerFlowComponent in lower.go.
func projectFlowArchComponentFromView(c ArchComponent) model.Component {
	var chain []model.Component
	var currentName string
	var currentMods []model.ComponentModifier

	flush := func() {
		if currentName == "" {
			return
		}
		chain = append(chain, model.Component{
			Name:      currentName,
			Type:      model.ComponentType("simple"),
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
						// The modifier value token may be Ident, String, or
						// Number; stringAwareText unquotes a String token so a
						// quoted flow-component modifier value projects unquoted.
						val = stringAwareText(toks[j+1])
						break
					}
				}
				currentMods = append(currentMods, model.ComponentModifier{Key: key, Value: val})
			}
		}
	}
	flush()

	return model.Component{
		Type:  model.ComponentType("flow"),
		Chain: chain,
	}
}
