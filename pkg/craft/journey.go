package craft

import (
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
)

type (
	Journey       = model.Journey
	JourneyMember = model.JourneyMember
	JourneyEdge   = model.JourneyEdge
)

// BuildJourney orders the use cases whose journey tag is slug by the
// hand-offs between them (publish→listen, store write→read, continues
// links), as spec §17.2 says: connected members first, and among ready ones
// a successor of the member placed last, then the namesake (the member
// whose title, or else whose file, is named after the journey), then file
// and line; unconnected members last, namesake first. The hub orders flows
// with it.
func BuildJourney(doc *CraftDoc, slug string) Journey { return journey.Build(doc, slug) }

// JourneySlugs returns every journey slug of doc, sorted.
func JourneySlugs(doc *CraftDoc) []string { return journey.Slugs(doc) }
