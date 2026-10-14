package lang

import "strings"

// OpItem is one item of an operation bracket: its text, trimmed, and the
// byte offset of that text in the bracket body (spec §8.1).
type OpItem struct {
	Text  string
	Start int
}

// SplitOperationItems splits a bracket body into its items: at a comma that
// is followed by whitespace, at brace depth 0, outside a "…" string.
// `{latitude},{longitude}` stays one item. An empty item (`a, , b`, `a, `)
// is returned with empty Text so the parser can report it. A body without a
// separator is one item.
func SplitOperationItems(body string) []OpItem {
	var out []OpItem
	depth, inString, start := 0, false, 0
	emit := func(end int) {
		raw := body[start:end]
		lead := len(raw) - len(strings.TrimLeft(raw, " \t"))
		out = append(out, OpItem{Text: strings.TrimSpace(raw), Start: start + lead})
	}
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}' && depth > 0:
			depth--
		case c == ',' && depth == 0 && i+1 < len(body) && (body[i+1] == ' ' || body[i+1] == '\t'):
			emit(i)
			start = i + 1
		}
	}
	emit(len(body))
	return out
}

// ParseOperationItem splits one item into its protocol verb and payload, as
// the parser splits a whole bracket. An ADDR address loses its quotes.
func ParseOperationItem(text string) (verb, payload string) {
	text = strings.TrimSpace(text)
	head, rest, _ := strings.Cut(text, " ")
	if !IsProtocolVerb(head) {
		return "", text
	}
	payload = strings.TrimSpace(rest)
	if head == OpVerbAddr {
		payload = UnquoteAddress(payload)
	}
	return head, payload
}

// IsEgressItem reports whether an operation item is egress (`ADDR …` or
// `SQL …`): a hop to an address or to tables, not an operation of the
// target's surface.
func IsEgressItem(text string) bool {
	verb, _ := ParseOperationItem(text)
	return verb == OpVerbAddr || verb == OpVerbSQL
}

// UnquoteAddress drops the quotes of a quoted ADDR address.
func UnquoteAddress(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
