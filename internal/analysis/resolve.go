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
	resolve := func(name string) *model.Participant {
		if name == "" {
			return nil
		}
		p, _ := sema.ResolveParticipant(ws, "", name)
		if p.Kind == "" {
			return nil
		}
		return &p
	}

	for i := range doc.UseCases {
		for j := range doc.UseCases[i].Scenarios {
			sc := &doc.UseCases[i].Scenarios[j]
			switch sc.Trigger.Type {
			case lang.TriggerExternal:
				sc.Trigger.Subject = resolve(sc.Trigger.Actor)
			case lang.TriggerDomainListen:
				sc.Trigger.Subject = resolve(sc.Trigger.Context)
			}
			sc.EachAction(func(a *model.Action) {
				a.From = resolve(a.Context)
				if a.Type == lang.ActionSync || a.Type == lang.ActionReturn {
					a.To = resolve(a.TargetContext)
				}
			})
		}
	}

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
