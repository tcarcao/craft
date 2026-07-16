package syntax

// EdgeClass classifies a context_map relationship verb as either directional
// (has a distinct upstream/downstream endpoint) or symmetric (no inherent
// direction between the two endpoints).
type EdgeClass string

const (
	EdgeDirectional EdgeClass = "directional"
	EdgeSymmetric   EdgeClass = "symmetric"
)

// EdgeVerbMeta describes a single context_map relationship verb: its class,
// and for directional verbs, the role labels for the upstream (LEFT operand)
// and downstream (RIGHT operand) endpoints. LEFT = upstream is the parser's
// convention (see edgeKeywords in parser.go). For symmetric verbs both role
// fields are "".
type EdgeVerbMeta struct {
	Verb           string
	Class          EdgeClass
	UpstreamRole   string // left role for directional (e.g. "supplier"); "" for symmetric
	DownstreamRole string // right role for directional (e.g. "customer"); "" for symmetric
}

// edgeVerbMetaByName is the single source of truth for verb class + role
// labels, keyed by verb. EdgeVerbMetas derives its returned slice from
// edgeKeywords (in parser.go) so the two can never drift out of order.
var edgeVerbMetaByName = map[string]EdgeVerbMeta{
	"customer_supplier": {
		Verb: "customer_supplier", Class: EdgeDirectional,
		UpstreamRole: "supplier", DownstreamRole: "customer",
	},
	"conformist": {
		Verb: "conformist", Class: EdgeDirectional,
		UpstreamRole: "upstream", DownstreamRole: "conformist",
	},
	"anticorruption_layer": {
		Verb: "anticorruption_layer", Class: EdgeDirectional,
		UpstreamRole: "upstream", DownstreamRole: "downstream",
	},
	"open_host_service": {
		Verb: "open_host_service", Class: EdgeDirectional,
		UpstreamRole: "host", DownstreamRole: "consumer",
	},
	"published_language": {
		Verb: "published_language", Class: EdgeDirectional,
		UpstreamRole: "publisher", DownstreamRole: "consumer",
	},
	"partnership": {
		Verb: "partnership", Class: EdgeSymmetric,
	},
	"shared_kernel": {
		Verb: "shared_kernel", Class: EdgeSymmetric,
	},
	"separate_ways": {
		Verb: "separate_ways", Class: EdgeSymmetric,
	},
}

// EdgeVerbMetas returns metadata for all context_map relationship verbs, in
// the same order as edgeKeywords. The order is derived from edgeKeywords
// directly (rather than hand-duplicated) so the two can never drift.
func EdgeVerbMetas() []EdgeVerbMeta {
	metas := make([]EdgeVerbMeta, 0, len(edgeKeywords))
	for _, k := range edgeKeywords {
		if m, ok := edgeVerbMetaByName[k]; ok {
			metas = append(metas, m)
		}
	}
	return metas
}

// LookupEdgeVerbMeta returns the metadata for verb, and whether it was found.
func LookupEdgeVerbMeta(verb string) (EdgeVerbMeta, bool) {
	m, ok := edgeVerbMetaByName[verb]
	return m, ok
}

// EdgeVerbSymmetric reports whether verb is a symmetric context_map
// relationship (no upstream/downstream distinction). Unknown verbs report
// false.
func EdgeVerbSymmetric(verb string) bool {
	m, ok := edgeVerbMetaByName[verb]
	return ok && m.Class == EdgeSymmetric
}
