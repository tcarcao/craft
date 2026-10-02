package craft

import "github.com/tcarcao/craft/v2/internal/lang"

type EdgeClass = lang.EdgeClass

const (
	EdgeDirectional = lang.EdgeDirectional
	EdgeSymmetric   = lang.EdgeSymmetric
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

func toEdgeVerbInfo(v lang.EdgeVerb) EdgeVerbInfo {
	return EdgeVerbInfo{
		Verb:           v.Verb,
		Class:          v.Class,
		UpstreamRole:   v.UpstreamRole,
		DownstreamRole: v.DownstreamRole,
		Symmetric:      v.Symmetric(),
	}
}

// EdgeVerbs returns metadata for all context_map relationship verbs, in
// presentation order (directional patterns first).
func EdgeVerbs() []EdgeVerbInfo {
	verbs := lang.EdgeVerbs()
	infos := make([]EdgeVerbInfo, 0, len(verbs))
	for _, v := range verbs {
		infos = append(infos, toEdgeVerbInfo(v))
	}
	return infos
}

// LookupEdgeVerb returns the metadata for verb, and whether it was found.
func LookupEdgeVerb(verb string) (EdgeVerbInfo, bool) {
	v, ok := lang.LookupEdgeVerb(verb)
	if !ok {
		return EdgeVerbInfo{}, false
	}
	return toEdgeVerbInfo(v), true
}
