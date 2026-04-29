// TEMPORARY: bridges *SyntaxNode to *ast.File for incremental migration.
// Delete once all consumers use typed views (Phase 4).
package syntax

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/internal/ast"
)

// Lower converts a SyntaxKindFile node to *ast.File.
// It reconstructs the full ast.File from the lossless syntax tree, including
// all source positions and computed IDs (scenario_N, action_N).
func Lower(node *SyntaxNode) *ast.File {
	if node == nil || node.Kind != SyntaxKindFile {
		return &ast.File{}
	}
	file := &ast.File{}
	// Global counter for scenario_N / action_N IDs across all use_cases,
	// matching ANTLR/parser numbering scheme.
	ucCounter := 0

	for _, child := range node.Children {
		n, ok := child.(*SyntaxNode)
		if !ok {
			continue
		}
		switch n.Kind {
		case SyntaxKindActorDecl:
			if a := lowerActorDecl(n); a != nil {
				file.Actors = append(file.Actors, a)
			}
		case SyntaxKindActorsBlock:
			actors, blockRange := lowerActorsBlock(n)
			file.Actors = append(file.Actors, actors...)
			if blockRange != nil {
				file.ActorBlocks = append(file.ActorBlocks, blockRange)
			}
		case SyntaxKindDomainDecl:
			if d := lowerDomainDecl(n, false); d != nil {
				file.Domains = append(file.Domains, d)
			}
		case SyntaxKindDomainsBlock:
			file.Domains = append(file.Domains, lowerDomainsBlock(n)...)
		case SyntaxKindServiceDecl:
			if s := lowerServiceDecl(n, false); s != nil {
				file.Services = append(file.Services, s)
			}
		case SyntaxKindServicesBlock:
			file.Services = append(file.Services, lowerServicesBlock(n)...)
		case SyntaxKindUseCaseDecl:
			if u := lowerUseCaseDecl(n, &ucCounter); u != nil {
				file.UseCases = append(file.UseCases, u)
			}
		case SyntaxKindArchDecl:
			if a := lowerArchDecl(n); a != nil {
				file.Archs = append(file.Archs, a)
			}
		case SyntaxKindExposureDecl:
			if e := lowerExposureDecl(n); e != nil {
				file.Exposures = append(file.Exposures, e)
			}
		}
	}
	return file
}

// lowerActorDecl reconstructs an *ast.ActorDecl from a SyntaxKindActorDecl node.
// The node may be a top-level `actor <type> <name>` (has keyword child) or an
// inline block entry `<type> <name>` (no actor keyword child).
func lowerActorDecl(n *SyntaxNode) *ast.ActorDecl {
	// Find the actor type token: one of user/system/service keywords or an ident.
	// Find the name token: the ident following the type.
	var typeKind SyntaxKind
	var typeTok, nameTok *SyntaxToken

	for _, child := range n.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		switch tok.Kind {
		case SyntaxKindKwUser, SyntaxKindKwSystem, SyntaxKindKwService:
			if typeTok == nil {
				typeKind = tok.Kind
				typeTok = tok
			}
		case SyntaxKindIdent:
			if typeTok == nil {
				// First ident is type (open taxonomy)
				typeTok = tok
				typeKind = SyntaxKindIdent
			} else {
				// Second ident is name
				nameTok = tok
			}
		case SyntaxKindKwActor:
			// skip the `actor` keyword itself
		}
	}

	if typeTok == nil {
		return nil
	}

	var at ast.ActorType
	switch typeKind {
	case SyntaxKindKwUser:
		at = ast.ActorTypeUser
	case SyntaxKindKwSystem:
		at = ast.ActorTypeSystem
	case SyntaxKindKwService:
		at = ast.ActorTypeService
	default:
		at = ast.ActorType(typeTok.Value)
	}

	if nameTok == nil {
		return nil
	}

	return &ast.ActorDecl{
		Name:   nameTok.Value,
		Type:   at,
		Line:   nameTok.Line,
		Column: nameTok.Col,
	}
}

// lowerActorsBlock reconstructs actors and ActorBlockRange from a SyntaxKindActorsBlock node.
func lowerActorsBlock(n *SyntaxNode) ([]*ast.ActorDecl, *ast.ActorBlockRange) {
	var actors []*ast.ActorDecl

	// Find `actors` keyword for block start line.
	kwTok := n.ChildToken(SyntaxKindKwActors)
	if kwTok == nil {
		return nil, nil
	}
	blockRange := &ast.ActorBlockRange{Line: kwTok.Line}

	// Find `}` token for end line.
	rBrace := n.ChildToken(SyntaxKindRBrace)
	if rBrace != nil {
		blockRange.EndLine = rBrace.Line
	}

	// Each ActorDecl child node is an inline actor entry.
	for _, child := range n.Children {
		node, ok := child.(*SyntaxNode)
		if !ok || node.Kind != SyntaxKindActorDecl {
			continue
		}
		if a := lowerActorDecl(node); a != nil {
			actors = append(actors, a)
		}
	}

	// Only return blockRange if the block was properly closed.
	if rBrace == nil {
		return actors, nil
	}
	return actors, blockRange
}

// lowerDomainDecl reconstructs a *ast.DomainDecl from a SyntaxKindDomainDecl node.
// isGrouped is true when the domain was inside a domains { } block.
func lowerDomainDecl(n *SyntaxNode, isGrouped bool) *ast.DomainDecl {
	// The domain name is the first Ident token (direct child).
	nameTok := n.ChildToken(SyntaxKindIdent)
	if nameTok == nil {
		return nil
	}

	// Find end line from the direct RBrace child.
	var endLine int
	rBrace := n.ChildToken(SyntaxKindRBrace)
	if rBrace != nil {
		endLine = rBrace.Line
	}

	// Collect BoundedContext child nodes.
	var contexts []ast.BoundedContextEntry
	for _, child := range n.Children {
		bcNode, ok := child.(*SyntaxNode)
		if !ok || bcNode.Kind != SyntaxKindBoundedContext {
			continue
		}
		bcTok := bcNode.ChildToken(SyntaxKindIdent)
		if bcTok == nil {
			continue
		}
		contexts = append(contexts, ast.BoundedContextEntry{
			Name:   bcTok.Value,
			Line:   bcTok.Line,
			Column: bcTok.Col,
		})
	}

	return &ast.DomainDecl{
		Name:            nameTok.Value,
		BoundedContexts: contexts,
		Line:            nameTok.Line,
		Column:          nameTok.Col,
		EndLine:         endLine,
		IsGrouped:       isGrouped,
	}
}

// lowerDomainsBlock reconstructs []*ast.DomainDecl from a SyntaxKindDomainsBlock node.
func lowerDomainsBlock(n *SyntaxNode) []*ast.DomainDecl {
	var domains []*ast.DomainDecl
	for _, child := range n.Children {
		domNode, ok := child.(*SyntaxNode)
		if !ok || domNode.Kind != SyntaxKindDomainDecl {
			continue
		}
		if d := lowerDomainDecl(domNode, true); d != nil {
			domains = append(domains, d)
		}
	}
	return domains
}

// lowerServiceDecl reconstructs a *ast.ServiceDecl from a SyntaxKindServiceDecl node.
// isGrouped is true when declared inside a services { } block.
func lowerServiceDecl(n *SyntaxNode, isGrouped bool) *ast.ServiceDecl {
	// Name token: first Ident or String token.
	var nameTok *SyntaxToken
	for _, child := range n.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		if tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString {
			// Skip keyword tokens that might be service keyword itself.
			if tok.Kind == SyntaxKindKwService {
				continue
			}
			nameTok = tok
			break
		}
	}
	if nameTok == nil {
		return nil
	}

	svc := &ast.ServiceDecl{
		Name:      nameTok.Value,
		Line:      nameTok.Line,
		Column:    nameTok.Col,
		IsGrouped: isGrouped,
	}

	// Find end line from RBrace.
	rBrace := n.ChildToken(SyntaxKindRBrace)
	if rBrace != nil {
		svc.EndLine = rBrace.Line
	}

	// The service body fields are encoded as tokens in document order.
	// We need to scan for field keywords (contexts, data-stores, language, deployment)
	// followed by colon, then values.
	// Strategy: collect all non-structural tokens and parse field sequences.
	lowerServiceBody(n, svc)

	return svc
}

// lowerServiceBody populates service fields by scanning the token sequence.
// Field keywords are contextual idents: "contexts", "data-stores", "language", "deployment".
func lowerServiceBody(n *SyntaxNode, svc *ast.ServiceDecl) {
	tokens := n.Tokens()

	// Skip the name token (first non-brace content token).
	// We look for field patterns: <ident-fieldname> <colon> <values...>
	i := 0
	// Skip past name token and opening brace.
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindLBrace {
			i++
			break
		}
		i++
	}

	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind != SyntaxKindIdent {
			i++
			continue
		}
		fieldName := tok.Value
		// Next should be colon.
		if i+1 >= len(tokens) || tokens[i+1].Kind != SyntaxKindColon {
			i++
			continue
		}
		i += 2 // skip field name + colon

		switch fieldName {
		case "contexts":
			svc.Contexts, svc.ContextLines, i = collectIdentListWithLines(tokens, i)
		case "data-stores":
			svc.DataStores, i = collectIdentList(tokens, i)
		case "language":
			if i < len(tokens) && tokens[i].Kind == SyntaxKindIdent {
				svc.Language = tokens[i].Value
				i++
			}
		case "deployment":
			svc.DeploymentType, svc.DeploymentRules, i = collectDeploymentSpec(tokens, i)
		default:
			// Unknown field — skip to next ident that could be a field name.
			for i < len(tokens) {
				if tokens[i].Kind == SyntaxKindRBrace || tokens[i].Kind == SyntaxKindIdent {
					break
				}
				i++
			}
		}
	}
}

// isFieldSentinel returns true when tokens[i] is an ident followed by a colon —
// i.e., the start of a new field definition. Used to stop value collection early.
func isFieldSentinel(tokens []*SyntaxToken, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	return (tokens[i].Kind == SyntaxKindIdent) && tokens[i+1].Kind == SyntaxKindColon
}

// collectIdentList collects comma-separated idents starting at tokens[i].
// Stops at a field sentinel (ident followed by colon), `}`, or non-ident token.
// Returns the list and the new index.
func collectIdentList(tokens []*SyntaxToken, i int) ([]string, int) {
	var items []string
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind == SyntaxKindComma {
			i++
			continue
		}
		if (tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString) && !isFieldSentinel(tokens, i) {
			items = append(items, tok.Value)
			i++
		} else {
			break
		}
	}
	return items, i
}

// collectIdentListWithLines collects comma-separated idents and their lines.
// Stops at a field sentinel (ident followed by colon), `}`, or non-ident token.
func collectIdentListWithLines(tokens []*SyntaxToken, i int) ([]string, []int, int) {
	var items []string
	var lines []int
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind == SyntaxKindComma {
			i++
			continue
		}
		if (tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString) && !isFieldSentinel(tokens, i) {
			items = append(items, tok.Value)
			lines = append(lines, tok.Line)
			i++
		} else {
			break
		}
	}
	return items, lines, i
}

// collectDeploymentSpec collects deployment type and optional rules.
func collectDeploymentSpec(tokens []*SyntaxToken, i int) (string, []ast.DeploymentRule, int) {
	if i >= len(tokens) {
		return "", nil, i
	}
	dtTok := tokens[i]
	if dtTok.Kind != SyntaxKindIdent {
		return "", nil, i
	}
	dt := dtTok.Value
	i++

	if i >= len(tokens) || tokens[i].Kind != SyntaxKindLParen {
		return dt, nil, i
	}
	i++ // consume '('

	var rules []ast.DeploymentRule
	for i < len(tokens) && tokens[i].Kind != SyntaxKindRParen {
		if tokens[i].Kind != SyntaxKindPercentage {
			i++
			continue
		}
		pct := tokens[i].Value
		i++
		if i < len(tokens) && tokens[i].Kind == SyntaxKindArrow {
			i++
		}
		var target string
		if i < len(tokens) && tokens[i].Kind == SyntaxKindIdent {
			target = tokens[i].Value
			i++
		}
		rules = append(rules, ast.DeploymentRule{Percentage: pct, Target: target})
		if i < len(tokens) && tokens[i].Kind == SyntaxKindComma {
			i++
		}
	}
	if i < len(tokens) && tokens[i].Kind == SyntaxKindRParen {
		i++
	}
	return dt, rules, i
}

// lowerServicesBlock reconstructs []*ast.ServiceDecl from a SyntaxKindServicesBlock node.
func lowerServicesBlock(n *SyntaxNode) []*ast.ServiceDecl {
	var services []*ast.ServiceDecl
	for _, child := range n.Children {
		svcNode, ok := child.(*SyntaxNode)
		if !ok || svcNode.Kind != SyntaxKindServiceDecl {
			continue
		}
		if s := lowerServiceDecl(svcNode, true); s != nil {
			services = append(services, s)
		}
	}
	return services
}

// lowerUseCaseDecl reconstructs a *ast.UseCaseDecl from a SyntaxKindUseCaseDecl node.
// counter is the shared global ID counter (pointer) for scenario_N / action_N IDs.
func lowerUseCaseDecl(n *SyntaxNode, counter *int) *ast.UseCaseDecl {
	// Name: first String token.
	nameTok := n.ChildToken(SyntaxKindString)
	if nameTok == nil {
		return nil
	}

	// use_case keyword for line number.
	ucTok := n.ChildToken(SyntaxKindKwUseCase)
	var ucLine int
	if ucTok != nil {
		ucLine = ucTok.Line
	}

	// End line from RBrace.
	var endLine int
	rBrace := n.ChildToken(SyntaxKindRBrace)
	if rBrace != nil {
		endLine = rBrace.Line
	}

	uc := &ast.UseCaseDecl{
		Name:    nameTok.Value,
		Line:    ucLine,
		EndLine: endLine,
	}

	// Parse scenarios.
	for _, child := range n.Children {
		scenNode, ok := child.(*SyntaxNode)
		if !ok || scenNode.Kind != SyntaxKindScenario {
			continue
		}
		if s := lowerScenario(scenNode, counter); s != nil {
			uc.Scenarios = append(uc.Scenarios, s)
		}
	}

	return uc
}

// lowerScenario reconstructs a *ast.ScenarioDecl from a SyntaxKindScenario node.
func lowerScenario(n *SyntaxNode, counter *int) *ast.ScenarioDecl {
	// Find the when token for its line.
	whenTok := n.ChildToken(SyntaxKindKwWhen)
	var whenLine int
	if whenTok != nil {
		whenLine = whenTok.Line
	}

	// Parse trigger.
	triggerNode := n.ChildNode(SyntaxKindTrigger)
	var trigger ast.TriggerDecl
	if triggerNode != nil {
		trigger = lowerTrigger(triggerNode, whenLine)
	}

	*counter++
	scenario := &ast.ScenarioDecl{
		ID:      fmt.Sprintf("scenario_%d", *counter),
		Trigger: trigger,
	}

	// Parse actions.
	for _, child := range n.Children {
		actionNode, ok := child.(*SyntaxNode)
		if !ok || actionNode.Kind != SyntaxKindAction {
			continue
		}
		if a := lowerAction(actionNode, counter); a != nil {
			scenario.Actions = append(scenario.Actions, a)
		}
	}

	return scenario
}

// lowerTrigger reconstructs an ast.TriggerDecl from a SyntaxKindTrigger node.
func lowerTrigger(n *SyntaxNode, whenLine int) ast.TriggerDecl {
	tokens := n.Tokens()
	if len(tokens) == 0 {
		return ast.TriggerDecl{Description: "when", Line: whenLine}
	}

	first := tokens[0]

	// event trigger: when "<EventName>"  (first token is a string)
	if first.Kind == SyntaxKindString {
		desc := fmt.Sprintf("when %q", first.Value)
		return ast.TriggerDecl{
			TriggerType:   "event",
			Event:         first.Value,
			EventColumn:   first.Col,
			EventIsString: true,
			Description:   desc,
			Line:          whenLine,
		}
	}

	// Subject token (actor/domain name).
	subject := first.Value
	subjectCol := first.Col

	if len(tokens) < 2 {
		return ast.TriggerDecl{
			TriggerType: "external",
			Actor:       subject,
			ActorColumn: subjectCol,
			Description: "when " + subject,
			Line:        whenLine,
		}
	}

	second := tokens[1]
	verb := second.Value

	// domain_listen: <domain> listens <event>
	if second.Kind == SyntaxKindKwListens {
		var event string
		var eventCol int
		var isString bool
		if len(tokens) >= 3 {
			eventTok := tokens[2]
			event = eventTok.Value
			eventCol = eventTok.Col
			isString = eventTok.Kind == SyntaxKindString
		}
		desc := fmt.Sprintf("when %s listens %q", subject, event)
		return ast.TriggerDecl{
			TriggerType:   "domain_listen",
			Context:       subject,
			ActorColumn:   subjectCol,
			Event:         event,
			EventColumn:   eventCol,
			EventIsString: isString,
			Description:   desc,
			Line:          whenLine,
		}
	}

	// external: when <actor> <verb> [connector_word] <phrase>
	// tokens[1] is verb; tokens[2..] may start with connector_word then phrase.
	phraseStart := 2
	var connector string
	if len(tokens) > 2 && isConnectorWord(tokens[2].Value) && tokens[2].Line == whenLine {
		connector = tokens[2].Value
		phraseStart = 3
	}
	_ = connector // connector is consumed into phrase in original parser

	var phraseParts []string
	for _, tok := range tokens[phraseStart:] {
		phraseParts = append(phraseParts, tok.Value)
	}
	phrase := strings.Join(phraseParts, " ")

	fullDesc := fmt.Sprintf("when %s %s %s", subject, verb, phrase)
	return ast.TriggerDecl{
		TriggerType: "external",
		Actor:       subject,
		ActorColumn: subjectCol,
		Verb:        verb,
		Phrase:      phrase,
		Description: fullDesc,
		Line:        whenLine,
	}
}

// lowerAction reconstructs an *ast.ActionDecl from a SyntaxKindAction node.
func lowerAction(n *SyntaxNode, counter *int) *ast.ActionDecl {
	tokens := n.Tokens()
	if len(tokens) == 0 {
		return nil
	}

	// An error node has only an Error token — skip.
	if tokens[0].Kind == SyntaxKindError {
		// Still increment counter twice to match parser's counter behavior for
		// "no verb" case (counter++ twice in parseAction).
		*counter++
		*counter++
		return nil
	}

	subjectTok := tokens[0]
	subject := subjectTok.Value
	subjectCol := subjectTok.Col
	actionLine := subjectTok.Line

	if len(tokens) < 2 {
		// No verb — minimal internal action. Parser increments counter twice.
		*counter++
		*counter++
		return &ast.ActionDecl{
			ActionType:    "internal_action",
			ActionID:      *counter,
			Context:       subject,
			ContextColumn: subjectCol,
			Description:   subject,
			Line:          actionLine,
		}
	}

	verbTok := tokens[1]
	verb := verbTok.Value

	*counter++
	id := *counter

	switch verb {
	case "asks":
		return lowerAsksAction(tokens, id, subject, subjectCol, actionLine)
	case "notifies":
		return lowerNotifiesAction(tokens, id, subject, subjectCol, actionLine)
	case "returns":
		return lowerReturnsAction(tokens, id, subject, subjectCol, actionLine)
	default:
		// internal_action: <domain> <verb> [connector_word] <phrase>
		phraseStart := 2
		var connector string
		if len(tokens) > 2 && isConnectorWord(tokens[2].Value) && tokens[2].Line == actionLine {
			connector = tokens[2].Value
			phraseStart = 3
		}
		var phraseParts []string
		for _, tok := range tokens[phraseStart:] {
			phraseParts = append(phraseParts, tok.Value)
		}
		phrase := strings.Join(phraseParts, " ")

		desc := subject + " " + verb
		if connector != "" {
			desc += " " + connector
		}
		if phrase != "" {
			desc += " " + phrase
		}
		return &ast.ActionDecl{
			ActionType:    "internal_action",
			ActionID:      id,
			Context:       subject,
			ContextColumn: subjectCol,
			Verb:          verb,
			Connector:     connector,
			Phrase:        phrase,
			Description:   desc,
			Line:          actionLine,
		}
	}
}

// lowerAsksAction reconstructs a sync_action from its token slice.
// tokens[0]=subject, tokens[1]=asks, tokens[2]=target, tokens[3]=to/for, tokens[4..]=phrase
func lowerAsksAction(tokens []*SyntaxToken, id int, subject string, subjectCol int, line int) *ast.ActionDecl {
	var target string
	var targetCol int
	i := 2
	if i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindIdent {
			target = tok.Value
			targetCol = tok.Col
			i++
		}
	}

	var connector string
	if i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindKwTo || (tok.Kind == SyntaxKindIdent && tok.Value == "for") {
			connector = tok.Value
			i++
		}
	}

	var phraseParts []string
	for _, tok := range tokens[i:] {
		phraseParts = append(phraseParts, tok.Value)
	}
	phrase := strings.Join(phraseParts, " ")

	desc := subject + " asks " + target + " " + connector
	if phrase != "" {
		desc += " " + phrase
	}

	return &ast.ActionDecl{
		ActionType:          "sync_action",
		ActionID:            id,
		Context:             subject,
		ContextColumn:       subjectCol,
		TargetContext:       target,
		TargetContextColumn: targetCol,
		Connector:           connector,
		Phrase:              phrase,
		Description:         desc,
		Line:                line,
	}
}

// lowerNotifiesAction reconstructs an async_action from its token slice.
// tokens[0]=subject, tokens[1]=notifies, tokens[2]=event
func lowerNotifiesAction(tokens []*SyntaxToken, id int, subject string, subjectCol int, line int) *ast.ActionDecl {
	var event string
	var eventCol int
	var eventIsString bool

	if len(tokens) >= 3 {
		eventTok := tokens[2]
		event = eventTok.Value
		eventCol = eventTok.Col
		eventIsString = eventTok.Kind == SyntaxKindString
	}

	desc := fmt.Sprintf("%s notifies %q", subject, event)
	return &ast.ActionDecl{
		ActionType:    "async_action",
		ActionID:      id,
		Context:       subject,
		ContextColumn: subjectCol,
		Event:         event,
		EventColumn:   eventCol,
		EventIsString: eventIsString,
		Description:   desc,
		Line:          line,
	}
}

// lowerReturnsAction reconstructs a return_action from its token slice.
// tokens[0]=subject, tokens[1]=returns, then optional `to <target>`, then phrase.
func lowerReturnsAction(tokens []*SyntaxToken, id int, subject string, subjectCol int, line int) *ast.ActionDecl {
	i := 2
	var target string
	var targetCol int
	var connector string

	// Optional `to <target>`
	if i < len(tokens) && tokens[i].Kind == SyntaxKindKwTo {
		i++ // skip `to`
		if i < len(tokens) {
			target = tokens[i].Value
			targetCol = tokens[i].Col
			i++
		}
	}

	// Optional connector_word
	if i < len(tokens) && isConnectorWord(tokens[i].Value) && tokens[i].Line == line {
		connector = tokens[i].Value
		i++
	}

	var phraseParts []string
	for _, tok := range tokens[i:] {
		phraseParts = append(phraseParts, tok.Value)
	}
	phrase := strings.Join(phraseParts, " ")

	var desc string
	if target != "" {
		desc = fmt.Sprintf("%s returns %s to %s", subject, phrase, target)
	} else {
		desc = fmt.Sprintf("%s returns %s", subject, phrase)
	}

	return &ast.ActionDecl{
		ActionType:          "return_action",
		ActionID:            id,
		Context:             subject,
		ContextColumn:       subjectCol,
		TargetContext:        target,
		TargetContextColumn:  targetCol,
		Connector:           connector,
		Phrase:              phrase,
		Description:         desc,
		Line:                line,
	}
}

// lowerArchDecl reconstructs an *ast.ArchDecl from a SyntaxKindArchDecl node.
func lowerArchDecl(n *SyntaxNode) *ast.ArchDecl {
	archTok := n.ChildToken(SyntaxKindKwArch)
	if archTok == nil {
		return nil
	}

	arch := &ast.ArchDecl{Line: archTok.Line}

	// Optional name: first Ident token after the arch keyword.
	for _, child := range n.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		if tok.Kind == SyntaxKindIdent {
			arch.Name = tok.Value
			break
		}
	}

	// End line from RBrace.
	rBrace := n.ChildToken(SyntaxKindRBrace)
	if rBrace != nil {
		arch.EndLine = rBrace.Line
	}

	// Parse arch sections.
	for _, child := range n.Children {
		sectionNode, ok := child.(*SyntaxNode)
		if !ok || sectionNode.Kind != SyntaxKindArchSection {
			continue
		}
		// Determine section type from first token.
		labelTok := sectionNode.ChildToken(SyntaxKindKwPresentation, SyntaxKindKwGateway, SyntaxKindIdent)
		if labelTok == nil {
			continue
		}
		components := lowerArchSection(sectionNode)
		switch labelTok.Kind {
		case SyntaxKindKwPresentation:
			arch.Presentation = components
			arch.PresentationLine = labelTok.Line
		case SyntaxKindKwGateway:
			arch.Gateway = components
			arch.GatewayLine = labelTok.Line
		default:
			// Unknown label — discard.
		}
	}

	return arch
}

// lowerArchSection extracts []*ast.ArchComponent from a SyntaxKindArchSection node.
func lowerArchSection(n *SyntaxNode) []*ast.ArchComponent {
	var components []*ast.ArchComponent
	for _, child := range n.Children {
		compNode, ok := child.(*SyntaxNode)
		if !ok || compNode.Kind != SyntaxKindArchComponent {
			continue
		}
		comp := lowerArchComponent(compNode)
		if comp != nil {
			components = append(components, comp)
		}
	}
	return components
}

// lowerArchComponent reconstructs an *ast.ArchComponent from a SyntaxKindArchComponent node.
func lowerArchComponent(n *SyntaxNode) *ast.ArchComponent {
	tokens := n.Tokens()
	if len(tokens) == 0 {
		return nil
	}

	// Detect flow vs simple by looking for GT token.
	hasGT := false
	for _, tok := range tokens {
		if tok.Kind == SyntaxKindGT {
			hasGT = true
			break
		}
	}

	if !hasGT {
		return lowerSimpleComponent(n)
	}

	// Flow chain: split on GT tokens.
	return lowerFlowComponent(n)
}

// lowerSimpleComponent reconstructs a simple arch component (name + optional modifiers).
func lowerSimpleComponent(n *SyntaxNode) *ast.ArchComponent {
	nameTok := n.ChildToken(SyntaxKindIdent)
	if nameTok == nil {
		return nil
	}
	comp := &ast.ArchComponent{Name: nameTok.Value, Type: "simple"}

	// Collect modifiers from ArchModifier child nodes.
	for _, child := range n.Children {
		modNode, ok := child.(*SyntaxNode)
		if !ok || modNode.Kind != SyntaxKindArchModifier {
			continue
		}
		tokens := modNode.Tokens()
		if len(tokens) == 0 {
			continue
		}
		key := tokens[0].Value
		var value string
		// tokens: [key, colon?, value?] or [key]
		// Colon is SyntaxKindColon; value follows.
		if len(tokens) >= 2 {
			// Find the colon and then value.
			for j, tok := range tokens {
				if tok.Kind == SyntaxKindColon && j+1 < len(tokens) {
					value = tokens[j+1].Value
					break
				}
			}
		}
		comp.Modifiers = append(comp.Modifiers, ast.ArchModifier{Key: key, Value: value})
	}

	return comp
}

// lowerFlowComponent reconstructs a flow arch component from a node with GT tokens.
func lowerFlowComponent(n *SyntaxNode) *ast.ArchComponent {
	// The flow component is a chain of simple components.
	// We parse by collecting name/modifier groups separated by GT.
	// The SyntaxNode for a flow component has its children flattened
	// (GT tokens + ident tokens + modifier nodes mixed).

	// Build chain by splitting on GT tokens.
	var chain []*ast.ArchComponent

	// Current component being built.
	var currentName string
	var currentMods []ast.ArchModifier

	flush := func() {
		if currentName == "" {
			return
		}
		chain = append(chain, &ast.ArchComponent{
			Name:      currentName,
			Type:      "simple",
			Modifiers: currentMods,
		})
		currentName = ""
		currentMods = nil
	}

	for _, child := range n.Children {
		switch c := child.(type) {
		case *SyntaxToken:
			switch c.Kind {
			case SyntaxKindGT:
				flush()
			case SyntaxKindIdent:
				if currentName == "" {
					currentName = c.Value
				}
			case SyntaxKindLBracket, SyntaxKindRBracket, SyntaxKindComma, SyntaxKindColon:
				// structural, ignore
			}
		case *SyntaxNode:
			if c.Kind == SyntaxKindArchModifier {
				tokens := c.Tokens()
				if len(tokens) == 0 {
					continue
				}
				key := tokens[0].Value
				var value string
				for j, tok := range tokens {
					if tok.Kind == SyntaxKindColon && j+1 < len(tokens) {
						value = tokens[j+1].Value
						break
					}
				}
				currentMods = append(currentMods, ast.ArchModifier{Key: key, Value: value})
			}
		}
	}
	flush()

	if len(chain) == 0 {
		return nil
	}
	return &ast.ArchComponent{Type: "flow", Chain: chain}
}

// lowerExposureDecl reconstructs an *ast.ExposureDecl from a SyntaxKindExposureDecl node.
func lowerExposureDecl(n *SyntaxNode) *ast.ExposureDecl {
	kwTok := n.ChildToken(SyntaxKindKwExposure)
	if kwTok == nil {
		return nil
	}

	nameTok := n.ChildToken(SyntaxKindIdent)
	if nameTok == nil {
		return nil
	}

	exp := &ast.ExposureDecl{
		Name: nameTok.Value,
		Line: kwTok.Line,
	}

	// Parse field tokens in order.
	// Fields: to: [...], contexts: [...], through: [...]
	// The `through:` field is stored in a DeploymentRule child node.
	tokens := n.Tokens()
	// Skip past exposure keyword, name, and opening brace.
	i := 0
	for i < len(tokens) {
		if tokens[i].Kind == SyntaxKindLBrace {
			i++
			break
		}
		i++
	}

	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind != SyntaxKindIdent && tok.Kind != SyntaxKindKwTo &&
			tok.Kind != SyntaxKindKwContexts && tok.Kind != SyntaxKindKwThrough {
			i++
			continue
		}
		fieldName := tok.Value
		// Next should be colon.
		if i+1 >= len(tokens) || tokens[i+1].Kind != SyntaxKindColon {
			i++
			continue
		}
		i += 2

		switch fieldName {
		case "to":
			exp.To, i = collectExposureIdentList(tokens, i)
		case "contexts":
			exp.Contexts, i = collectExposureIdentList(tokens, i)
		case "through":
			exp.Through, i = collectExposureIdentList(tokens, i)
		default:
			// skip
		}
	}

	return exp
}

// collectExposureIdentList collects ident tokens stopping at known field names or `}`.
func collectExposureIdentList(tokens []*SyntaxToken, i int) ([]string, int) {
	var items []string
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind == SyntaxKindComma {
			i++
			continue
		}
		// Stop if we see a field keyword followed by colon.
		if (tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindKwTo ||
			tok.Kind == SyntaxKindKwContexts || tok.Kind == SyntaxKindKwThrough) &&
			i+1 < len(tokens) && tokens[i+1].Kind == SyntaxKindColon {
			break
		}
		if tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString {
			items = append(items, tok.Value)
		}
		i++
	}
	return items, i
}
