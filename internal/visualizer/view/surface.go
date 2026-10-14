package view

import (
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Surfaces is the surface tables of a model, ordered by subject.
type Surfaces struct{ Tables []SurfaceTable }

// SurfaceTable is one surface: its header facts and one row per operation, in
// source order.
type SurfaceTable struct {
	Subject string
	Callers []string
	Spec    string
	Notes   []model.Note
	Rows    []SurfaceRow
}

// SurfaceRow is one operation and the use cases that use it.
type SurfaceRow struct {
	Operation string
	Kind      model.SurfaceOpKind
	Offers    string   // connector + phrase
	UsedBy    []string // use-case names, sorted, deduplicated
}

// unobservedSuffix marks a use case that reaches an entry through a
// `receives` trigger: the boundary is called, but by no one the model knows.
const unobservedSuffix = " (unobserved caller)"

// BuildSurfaces computes coverage (spec §8.4): an entry is used by every use
// case with a Request to the surface's boundary whose operation text equals the
// entry's (whitespace collapsed); a receives trigger on the boundary with that
// operation counts as "<use case> (unobserved caller)". A surface on a service
// also covers requests to the service's contexts that have no surface of
// their own (model.SurfaceBoundary). Tables are ordered by subject, ignoring
// case.
func BuildSurfaces(doc *model.CraftDoc, its []interaction.Interaction) Surfaces {
	type usageKey struct{ boundary, op string }
	used := map[usageKey]map[string]bool{}
	add := func(k usageKey, name string) {
		if used[k] == nil {
			used[k] = map[string]bool{}
		}
		used[k][name] = true
	}
	hasSurface := map[string]bool{}
	for _, s := range doc.Surfaces {
		if s.Participant != nil {
			hasSurface[s.Participant.ID] = true
		}
	}
	for _, it := range its {
		if it.Operation == nil {
			continue
		}
		boundary, ok := model.SurfaceBoundary(model.Participant{Kind: it.To.Kind, ID: it.To.Key, Service: it.To.Service},
			func(id string) bool { return hasSurface[id] })
		if !ok {
			continue
		}
		k := usageKey{boundary, collapse(it.Operation.Text)}
		switch {
		case it.Kind == interaction.Request:
			add(k, it.UseCase)
		case it.Kind == interaction.Trigger && it.From == interaction.Unobserved:
			add(k, it.UseCase+unobservedSuffix)
		}
	}

	var v Surfaces
	for _, s := range doc.Surfaces {
		boundary := s.Subject
		if s.Participant != nil {
			boundary = s.Participant.ID
		}
		t := SurfaceTable{Subject: s.Subject, Callers: s.Callers, Spec: s.Spec, Notes: s.Notes}
		for _, op := range s.Operations {
			names := used[usageKey{boundary, collapse(op.Operation.Text)}]
			usedBy := make([]string, 0, len(names))
			for n := range names {
				usedBy = append(usedBy, n)
			}
			slices.Sort(usedBy)
			t.Rows = append(t.Rows, SurfaceRow{
				Operation: op.Operation.Text,
				Kind:      op.Kind,
				Offers:    strings.TrimSpace(op.Connector + " " + op.Phrase),
				UsedBy:    usedBy,
			})
		}
		v.Tables = append(v.Tables, t)
	}
	slices.SortStableFunc(v.Tables, func(a, b SurfaceTable) int {
		if c := strings.Compare(strings.ToLower(a.Subject), strings.ToLower(b.Subject)); c != 0 {
			return c
		}
		return strings.Compare(a.Subject, b.Subject)
	})
	return v
}

// collapse turns runs of whitespace into one space, so operation texts compare
// the way sema compares them.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
