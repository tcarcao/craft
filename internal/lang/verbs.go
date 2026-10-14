package lang

import "sort"

// ── context_map relationship verbs ───────────────────────────────────────────

// EdgeClass says whether a context_map verb has a direction.
type EdgeClass string

const (
	EdgeDirectional EdgeClass = "directional" // LEFT is upstream, RIGHT downstream
	EdgeSymmetric   EdgeClass = "symmetric"   // no upstream/downstream
)

// EdgeVerb describes one context_map relationship verb (a DDD strategic
// pattern). For a directional verb the roles name the LEFT (upstream) and
// RIGHT (downstream) operands; a symmetric verb has neither.
type EdgeVerb struct {
	Verb           string
	Class          EdgeClass
	UpstreamRole   string
	DownstreamRole string
}

// Symmetric reports whether the verb has no direction.
func (v EdgeVerb) Symmetric() bool { return v.Class == EdgeSymmetric }

// edgeVerbs is the single source of truth for context_map verbs, in the order
// documentation and completion present them.
var edgeVerbs = []EdgeVerb{
	{Verb: "customer_supplier", Class: EdgeDirectional, UpstreamRole: "supplier", DownstreamRole: "customer"},
	{Verb: "conformist", Class: EdgeDirectional, UpstreamRole: "upstream", DownstreamRole: "conformist"},
	{Verb: "anticorruption_layer", Class: EdgeDirectional, UpstreamRole: "upstream", DownstreamRole: "downstream"},
	{Verb: "open_host_service", Class: EdgeDirectional, UpstreamRole: "host", DownstreamRole: "consumer"},
	{Verb: "published_language", Class: EdgeDirectional, UpstreamRole: "publisher", DownstreamRole: "consumer"},
	{Verb: "partnership", Class: EdgeSymmetric},
	{Verb: "shared_kernel", Class: EdgeSymmetric},
	{Verb: "separate_ways", Class: EdgeSymmetric},
}

// EdgeVerbs returns every context_map verb, in presentation order.
func EdgeVerbs() []EdgeVerb { return append([]EdgeVerb(nil), edgeVerbs...) }

// LookupEdgeVerb returns the description of verb, and whether it is one.
func LookupEdgeVerb(verb string) (EdgeVerb, bool) {
	for _, v := range edgeVerbs {
		if v.Verb == verb {
			return v, true
		}
	}
	return EdgeVerb{}, false
}

// IsEdgeVerb reports whether s is a context_map verb.
func IsEdgeVerb(s string) bool {
	_, ok := LookupEdgeVerb(s)
	return ok
}

// ── glossary relation verbs ──────────────────────────────────────────────────

// glossaryVerbs are the term relations a glossary block declares. All are
// symmetric, so there is no class or role metadata.
var glossaryVerbs = []string{"same_as", "contrasts", "distinct_from"}

// GlossaryVerbs returns the glossary relation verbs.
func GlossaryVerbs() []string { return append([]string(nil), glossaryVerbs...) }

// IsGlossaryVerb reports whether s is a glossary relation verb.
func IsGlossaryVerb(s string) bool {
	for _, v := range glossaryVerbs {
		if v == s {
			return true
		}
	}
	return false
}

// ── operation-annotation protocol verbs ──────────────────────────────────────

// OpVerbAddr and OpVerbSQL are the egress protocol verbs (spec §9.1).
const (
	OpVerbAddr = "ADDR" // egress to an address that has no operation id
	OpVerbSQL  = "SQL"  // a direct read or write of another party's tables
)

// protocolVerbs are the leading words of a `[VERB payload]` annotation that
// Craft recognises. Matching is exact and case-sensitive. Adding one is
// non-breaking: an unrecognised leading word already parses as payload.
var protocolVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "GRPC": true, "TOPIC": true, "QUERY": true,
}

// ProtocolVerbs returns the protocol verbs in sorted order.
func ProtocolVerbs() []string {
	out := make([]string, 0, len(protocolVerbs))
	for v := range protocolVerbs {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// IsProtocolVerb reports whether s is a protocol verb.
func IsProtocolVerb(s string) bool { return protocolVerbs[s] }
