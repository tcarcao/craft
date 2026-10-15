package analysis

import (
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
)

// resolveParticipants records on the merged document what every use-case
// name and service context resolves to, so consumers read the answer instead
// of matching names themselves. It uses the same resolver as sema's checks
// and the LSP, so a model that validates draws exactly what it says.
func resolveParticipants(doc *model.CraftDoc, ws sema.WorkspaceSymbols) {
	displayNames := doc.DisplayNames()
	resolve := func(name string) *model.Participant {
		if name == "" {
			return nil
		}
		p, _ := sema.ResolveParticipant(ws, "", name)
		if p.Kind == "" {
			return nil
		}
		if p.Kind == model.ParticipantBoundedContext || p.Kind == model.ParticipantActor {
			p.DisplayName = displayNames[p.ID]
		}
		return &p
	}
	// A store hand-off keeps its participant only when the name is a
	// declared data store.
	resolveStore := func(s *model.StoreRef) {
		if s == nil {
			return
		}
		if p := resolve(s.Name); p != nil && p.Kind == model.ParticipantDataStore {
			s.Participant = p
		}
	}

	for i := range doc.UseCases {
		for j := range doc.UseCases[i].Scenarios {
			sc := &doc.UseCases[i].Scenarios[j]
			switch sc.Trigger.Type {
			case lang.TriggerExternal:
				sc.Trigger.Subject = resolve(sc.Trigger.Actor)
			case lang.TriggerDomainListen, lang.TriggerReceive:
				sc.Trigger.Subject = resolve(sc.Trigger.Context)
			}
			resolveStore(sc.Trigger.Store)
			sc.EachAction(func(a *model.Action) {
				a.From = resolve(a.Context)
				resolveStore(a.Store)
				if a.Type == lang.ActionSync || a.Type == lang.ActionReturn {
					a.To = resolve(a.TargetContext)
				}
			})
			// A link resolves to its target only when one use case of the
			// workspace answers to the title (spec §15.2); sema.LintModel
			// reports the other outcomes.
			sc.EachLink(func(l *model.Link) {
				if matches, _ := model.ResolveUseCaseTitle(doc, l.Title); len(matches) == 1 {
					l.Target = &matches[0]
				}
			})
		}
	}

	// A boundary keeps its first surface in sorted file order (ruling 6);
	// sema reports the others as duplicate-name.
	var surfaces []model.Surface
	seenSurface := map[string]bool{}
	for _, s := range doc.Surfaces {
		s.Participant = resolve(s.Subject)
		key := "?" + s.Subject
		if s.Participant != nil {
			key = s.Participant.ID
		}
		if seenSurface[key] {
			continue
		}
		seenSurface[key] = true
		surfaces = append(surfaces, s)
	}
	doc.Surfaces = surfaces

	for i := range doc.Services {
		svc := &doc.Services[i]
		if len(svc.Contexts) == 0 {
			continue
		}
		ids := make([]string, len(svc.Contexts))
		resolved := false
		for k, ctx := range svc.Contexts {
			if p := resolve(ctx); p != nil && (p.Kind == model.ParticipantBoundedContext || p.Kind == model.ParticipantDomain) {
				ids[k] = p.ID
				resolved = true
			}
		}
		if resolved {
			svc.ContextIDs = ids
		}
	}
}
