package craft

import "github.com/tcarcao/craft/v2/internal/syntax"

type EdgeClass = syntax.EdgeClass

const (
	EdgeDirectional = syntax.EdgeDirectional
	EdgeSymmetric   = syntax.EdgeSymmetric
)

// EdgeVerbInfo describes a single context_map relationship verb: its class,
// role labels for directional verbs (UpstreamRole for the LEFT operand,
// DownstreamRole for the RIGHT operand; both "" for symmetric verbs), and a
// convenience Symmetric flag equivalent to Class == EdgeSymmetric.
type EdgeVerbInfo struct {
	Verb           string
	Class          EdgeClass
	UpstreamRole   string
	DownstreamRole string
	Symmetric      bool
}

func toEdgeVerbInfo(m syntax.EdgeVerbMeta) EdgeVerbInfo {
	return EdgeVerbInfo{
		Verb:           m.Verb,
		Class:          m.Class,
		UpstreamRole:   m.UpstreamRole,
		DownstreamRole: m.DownstreamRole,
		Symmetric:      m.Class == EdgeSymmetric,
	}
}

// EdgeVerbs returns metadata for all context_map relationship verbs, in the
// same order as the internal edgeKeywords table.
func EdgeVerbs() []EdgeVerbInfo {
	metas := syntax.EdgeVerbMetas()
	infos := make([]EdgeVerbInfo, 0, len(metas))
	for _, m := range metas {
		infos = append(infos, toEdgeVerbInfo(m))
	}
	return infos
}

// LookupEdgeVerb returns the metadata for verb, and whether it was found.
func LookupEdgeVerb(verb string) (EdgeVerbInfo, bool) {
	m, ok := syntax.LookupEdgeVerbMeta(verb)
	if !ok {
		return EdgeVerbInfo{}, false
	}
	return toEdgeVerbInfo(m), true
}
