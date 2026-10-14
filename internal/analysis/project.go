package analysis

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// project builds one file's document model from its syntax tree. Names are
// copied as written; nothing is resolved here. sourceURI is stamped on every
// use case.
func project(root syntax.SyntaxNode, li green.LineIndex, sourceURI string) *model.CraftDoc {
	doc := &model.CraftDoc{UseCases: []model.UseCase{}}
	if root == (syntax.SyntaxNode{}) {
		return doc
	}
	file := syntax.AsFile(root)

	doc.Services = mergeServices(projectServices(file, li))

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

	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		var bcNames []string
		for _, bc := range d.BoundedContexts() {
			if bcTok := bc.Name(); bcTok != nil {
				bcNames = append(bcNames, bcTok.Text())
			}
		}
		doc.Domains = append(doc.Domains, model.Domain{
			Name:            syntax.StringAwareText(*nameTok),
			BoundedContexts: bcNames,
		})
	}
	doc.Domains = firstByName(doc.Domains, func(d model.Domain) string { return d.Name })

	doc.UseCases = projectUseCases(file, li, sourceURI)

	for _, a := range file.Archs() {
		name := ""
		if nameTok := a.Name(); nameTok != nil {
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
			var components []model.Component
			for _, c := range section.Components() {
				components = append(components, projectArchComponent(c))
			}
			switch kwTok.Kind() {
			case syntax.SyntaxKindKwPresentation:
				ab.Presentation = append(ab.Presentation, components...)
			case syntax.SyntaxKindKwGateway:
				ab.Gateway = append(ab.Gateway, components...)
			}
		}
		doc.Architectures = append(doc.Architectures, ab)
	}

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

	for _, cm := range file.ContextMaps() {
		for _, e := range cm.Edges() {
			left, verb, right := e.Left(), e.Verb(), e.Right()
			if left == "" || verb == "" || right == "" {
				continue // malformed — skip
			}
			doc.ContextMap = append(doc.ContextMap, model.Edge{Left: left, Verb: verb, Right: right})
		}
	}

	for _, g := range file.Glossaries() {
		for _, r := range g.Relations() {
			left, verb, right := r.Left(), r.Verb(), r.Right()
			if left == "" || verb == "" || right == "" {
				continue // malformed — skip
			}
			doc.Glossary = append(doc.Glossary, model.TermRelation{Left: left, Verb: verb, Right: right})
		}
	}

	return doc
}

// projectUseCases numbers scenarios, actions, blocks and notes from one counter
// shared across the file (scenario_N / action_N / block_N / note_N), in source
// order. An incomplete action line still consumes
// two numbers, so the IDs of the actions after it do not depend on whether
// it parsed.
func projectUseCases(file syntax.File, li green.LineIndex, sourceURI string) []model.UseCase {
	out := []model.UseCase{}
	counter := 0
	for _, uc := range file.UseCases() {
		if uc.Title() == nil {
			continue
		}
		ucLine, _ := li.LineCol(uc.Title().Offset())
		outUC := model.UseCase{
			Name:      uc.Name(),
			Scenarios: []model.Scenario{},
			Line:      ucLine,
			SourceURI: sourceURI,
		}
		if tb := uc.TagsBlock(); tb != nil {
			// Last write wins on a duplicate key; sema warns about it.
			tags := map[string]string{}
			tagValues := map[string][]string{}
			for _, ts := range tb.Tags() {
				keyTok := ts.Key()
				if keyTok == nil {
					continue
				}
				// Both forms: the joined string existing consumers read, and the
				// split list, the only one that tells `web, mobile` from
				// `"web, mobile"`.
				tags[keyTok.Text()] = ts.ValueText()
				tagValues[keyTok.Text()] = ts.Values()
			}
			if len(tags) > 0 {
				outUC.Tags = tags
				outUC.TagValues = tagValues
			}
		}
		// Use-case-level notes precede every scenario in source order, so they
		// take their numbers first.
		for _, n := range uc.Notes() {
			counter++
			outUC.Notes = append(outUC.Notes, projectNote(n, fmt.Sprintf("note_%d", counter), li))
		}
		for _, sc := range uc.Scenarios() {
			// An orphan `scenario "x"` (no `when`) is recovered as a scenario
			// node without a `when` token. It is not a real scenario, so it is
			// skipped before the counter moves: IDs of later scenarios are the
			// same as if the orphan line were absent.
			if sc.When() == nil {
				continue
			}
			counter++
			outSc := model.Scenario{
				ID:      fmt.Sprintf("scenario_%d", counter),
				Name:    sc.Name(),
				Trigger: projectTrigger(sc, li),
				Steps:   []model.Step{},
			}
			outSc.Steps = projectSteps(sc.Steps(), &counter, li)
			outUC.Scenarios = append(outUC.Scenarios, outSc)
		}
		out = append(out, outUC)
	}
	return out
}

// projectSteps projects syntax steps in source order. Blocks take a number
// from the same counter as actions (block_N), so IDs stay in source order.
func projectSteps(steps []syntax.StepDecl, counter *int, li green.LineIndex) []model.Step {
	out := []model.Step{}
	for _, st := range steps {
		if n, ok := st.Note(); ok {
			*counter++
			note := projectNote(n, fmt.Sprintf("note_%d", *counter), li)
			out = append(out, model.Step{Note: &note})
			continue
		}
		if a, ok := st.Action(); ok {
			if a.IsIncomplete() {
				*counter += 2
				continue
			}
			*counter++
			act := projectAction(a, fmt.Sprintf("action_%d", *counter), li)
			out = append(out, model.Step{Action: &act})
			continue
		}
		if blk, ok := st.Block(); ok {
			out = append(out, model.Step{Block: projectBlock(blk, counter, li)})
		}
		// A step that is none of note, action or block is unreachable by
		// construction: the parser only builds step nodes of those kinds.
		// Should it ever happen, the node is dropped without consuming a
		// counter value, and nothing panics.
	}
	return out
}

// projectNote projects one note or finding line. id is "" in a surface.
func projectNote(n syntax.NoteDecl, id string, li green.LineIndex) model.Note {
	out := model.Note{ID: id, Kind: n.Kind(), Text: n.Text()}
	if tok := n.KeywordToken(); tok != nil {
		out.Line, out.Column = li.LineCol16(tok.Offset())
	}
	return out
}

// fanOutGroups splits the body of a par written without `and` into its
// concurrent branches: one per step that is not a note. Notes do not count as
// steps (spec §4.1 rule 2): each joins the branch of the step before it, and
// notes before the first step join the first branch. A body of notes only is
// one branch. Source order is kept, so IDs stay in source order.
func fanOutGroups(steps []syntax.StepDecl) [][]syntax.StepDecl {
	var groups [][]syntax.StepDecl
	var lead []syntax.StepDecl
	for _, st := range steps {
		if _, isNote := st.Note(); isNote {
			if len(groups) == 0 {
				lead = append(lead, st)
			} else {
				groups[len(groups)-1] = append(groups[len(groups)-1], st)
			}
			continue
		}
		groups = append(groups, append(lead, st))
		lead = nil
	}
	if len(groups) == 0 {
		return [][]syntax.StepDecl{lead}
	}
	return groups
}

// firstNonNote returns the index of the first step of group that is not a
// note, or 0 when every step is one.
func firstNonNote(group []syntax.StepDecl) int {
	for i, st := range group {
		if _, isNote := st.Note(); !isNote {
			return i
		}
	}
	return 0
}

// projectBlock projects one alt/opt block. The block takes its number before
// its branches, so it precedes the actions inside it in ID order.
func projectBlock(b syntax.BlockDecl, counter *int, li green.LineIndex) *model.Block {
	*counter++
	mb := &model.Block{ID: fmt.Sprintf("block_%d", *counter), Kind: b.Kind(), Branches: []model.Branch{}}
	if b.IsFanOut() {
		br := b.Branches()[0]
		if steps := br.Steps(); len(steps) > 0 {
			// Fan-out: one concurrent branch per body step, every one carrying
			// the header's label (spec §4.3). The block keeps the header's
			// position; each branch takes the line of its step.
			mb.Split = true
			if tok := br.HeaderToken(); tok != nil {
				mb.Line, mb.Column = li.LineCol16(tok.Offset())
			}
			for _, group := range fanOutGroups(steps) {
				mbr := model.Branch{Label: br.Label()}
				if toks := group[firstNonNote(group)].Node().Tokens(); len(toks) > 0 {
					mbr.Line, _ = li.LineCol(toks[0].Offset())
				}
				mbr.Steps = projectSteps(group, counter, li)
				mb.Branches = append(mb.Branches, mbr)
			}
			return mb
		}
	}
	for i, br := range b.Branches() {
		mbr := model.Branch{Label: br.Label()}
		// A branch without a header token keeps Line 0 (and, for the first
		// branch, the block keeps Line/Column 0): there is no position to report.
		if tok := br.HeaderToken(); tok != nil {
			line, col := li.LineCol16(tok.Offset())
			mbr.Line = line
			if i == 0 {
				mb.Line, mb.Column = line, col
			}
		}
		mbr.Steps = projectSteps(br.Steps(), counter, li)
		mb.Branches = append(mb.Branches, mbr)
	}
	return mb
}

func projectTrigger(sc syntax.ScenarioDecl, li green.LineIndex) model.Trigger {
	t := sc.Trigger()
	kind := t.Kind()
	var actor, context, event, verb, connector, phrase, ref string
	var op *model.Operation
	schedKind, schedule, scheduled := t.Schedule()
	switch kind {
	case lang.TriggerEvent:
		event = t.EventValue()
	case lang.TriggerDomainListen:
		context = t.ContextName()
		event = t.EventValue()
		ref = t.EventRef()
	case lang.TriggerReceive:
		context = t.ContextName()
		connector = t.Connector()
		phrase = t.PhraseText()
		if text := t.OpText(); text != "" {
			op = &model.Operation{Verb: t.OpVerb(), Payload: t.OpPayload(), Text: text}
		}
	default: // external
		if scheduled {
			// A schedule runs as the conventional scheduler actor and has no
			// verb or phrase.
			actor = lang.SchedulerActor
			break
		}
		actor = t.ExternalSubjectText()
		verb = t.ExternalVerbText()
		connector = t.Connector()
		phrase = t.PhraseText()
	}
	var description string
	switch {
	case scheduled:
		description = fmt.Sprintf(`when %s "%s"`, schedKind, schedule)
	case kind == lang.TriggerEvent:
		description = fmt.Sprintf("when %q", event)
	case kind == lang.TriggerDomainListen:
		description = fmt.Sprintf("when %s listens %q", context, event)
	case kind == lang.TriggerReceive:
		description = strings.TrimSpace(fmt.Sprintf("when %s receives %s", context, joinConnector(connector, phrase)))
		if op != nil {
			description += " [" + op.Text + "]"
		}
	default:
		// No trimming: an empty phrase leaves a trailing space, as it always has.
		description = fmt.Sprintf("when %s %s %s", actor, verb, joinConnector(connector, phrase))
	}
	out := model.Trigger{
		Type:         kind,
		Actor:        actor,
		Verb:         verb,
		Connector:    connector,
		Phrase:       phrase,
		Context:      context,
		Event:        event,
		Ref:          ref,
		Schedule:     schedule,
		ScheduleKind: schedKind,
		Operation:    op,
		Description:  description,
		Column:       t.ActorCol(li),
	}
	if when := sc.When(); when != nil {
		out.Line, _ = li.LineCol(when.Offset())
	}
	return out
}

// joinConnector puts a trigger's connector word back in front of its phrase.
func joinConnector(connector, phrase string) string {
	if connector == "" {
		return phrase
	}
	if phrase == "" {
		return connector
	}
	return connector + " " + phrase
}

func projectAction(a syntax.ActionDecl, id string, li green.LineIndex) model.Action {
	kind := a.Kind()
	subject := a.SubjectName()
	target := a.TargetName()
	event := a.EventValue()
	connector := a.DescriptionConnector()
	phrase := a.PhraseText()

	var verb, description string
	switch kind {
	case lang.ActionSync:
		// The connector is always included, so an empty one leaves a double space.
		description = subject + " asks " + target + " " + connector
		if phrase != "" {
			description += " " + phrase
		}
	case lang.ActionAsync:
		description = fmt.Sprintf("%s notifies %q", subject, event)
	case lang.ActionReturn:
		if target != "" {
			description = fmt.Sprintf("%s returns %s to %s", subject, phrase, target)
		} else {
			description = fmt.Sprintf("%s returns %s", subject, phrase)
		}
	default: // internal_action
		verb = a.VerbValue()
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
	var operation *model.Operation
	if text := a.OpText(); text != "" {
		operation = &model.Operation{Verb: a.OpVerb(), Payload: a.OpPayload(), Text: text}
	}
	return model.Action{
		ID:            id,
		Type:          kind,
		Context:       subject,
		Verb:          verb,
		TargetContext: target,
		Event:         event,
		Ref:           a.ObjectRef(),
		Operation:     operation,
		Connector:     connector,
		Phrase:        phrase,
		Description:   description,
		Line:          actionLine,
		Column:        a.SubjectCol(li),
	}
}

// projectServices returns one Service per service block, in source order.
// Same-name blocks are combined afterwards by mergeServices.
func projectServices(file syntax.File, li green.LineIndex) []model.Service {
	var out []model.Service
	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		line, _ := li.LineCol(nameTok.Offset())
		s := model.Service{
			Name:       syntax.StringAwareText(*nameTok),
			Contexts:   svc.Contexts(),
			DataStores: svc.DataStores(),
			Language:   svc.Language(),
			CatalogRef: svc.CatalogRef(),
			Repo:       svc.Repo(),
			Deployment: model.DeploymentStrategy{Type: svc.DeploymentType()},
			Line:       line,
		}
		for _, r := range svc.DeploymentRules() {
			s.Deployment.Rules = append(s.Deployment.Rules, model.DeploymentRule{
				Percentage: r.Percentage,
				Target:     r.Target,
			})
		}
		out = append(out, s)
	}
	return out
}

func projectArchComponent(c syntax.ArchComponent) model.Component {
	if !c.IsFlow() {
		comp := model.Component{Type: model.ComponentTypeSimple}
		if nameTok := c.Name(); nameTok != nil {
			comp.Name = nameTok.Text()
		}
		for _, m := range c.Modifiers() {
			var mod model.ComponentModifier
			if k := m.Key(); k != nil {
				mod.Key = k.Text()
			}
			if v := m.Value(); v != nil {
				mod.Value = syntax.StringAwareText(*v)
			}
			comp.Modifiers = append(comp.Modifiers, mod)
		}
		return comp
	}
	var chain []model.Component
	for _, step := range c.FlowSteps() {
		link := model.Component{Name: step.Name, Type: model.ComponentTypeSimple}
		for _, m := range step.Modifiers {
			link.Modifiers = append(link.Modifiers, model.ComponentModifier{Key: m.Key, Value: m.Value})
		}
		chain = append(chain, link)
	}
	return model.Component{Type: model.ComponentTypeFlow, Chain: chain}
}
