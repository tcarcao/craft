package syntax

import "sort"

// protocolVerbs is the recognised set of operation-annotation protocol verbs.
// Matching is exact and case-sensitive: only the uppercase spelling is a verb,
// so a lowercase `post` inside a bracket is payload, not a verb.
//
// Extending this set is additive and non-breaking: an unrecognised leading word
// already parses as opaque payload, so promoting it to a verb later only adds
// structure, it never invalidates a file that already parsed.
var protocolVerbs = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"PATCH":   true,
	"DELETE":  true,
	"HEAD":    true,
	"OPTIONS": true,
	"GRPC":    true,
	"TOPIC":   true,
	"QUERY":   true,
}

// isProtocolVerb reports whether s is a recognised protocol verb.
func isProtocolVerb(s string) bool { return protocolVerbs[s] }

// ProtocolVerbs returns the recognised protocol verbs in sorted order. Exported
// for LSP completion.
func ProtocolVerbs() []string {
	out := make([]string, 0, len(protocolVerbs))
	for v := range protocolVerbs {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
