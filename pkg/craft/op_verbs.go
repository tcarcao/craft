package craft

import "github.com/tcarcao/craft/v2/internal/syntax"

// ProtocolVerbs returns the operation-annotation protocol verbs Craft
// recognises, in sorted order.
func ProtocolVerbs() []string {
	return syntax.ProtocolVerbs()
}
