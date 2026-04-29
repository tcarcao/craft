package syntax

import "testing"

func TestSyntaxKind_SentinelUnder1000(t *testing.T) {
	if int(syntaxKindTokenSentinel) >= 1000 {
		t.Errorf("token sentinel %d >= 1000: node/token boundary broken", syntaxKindTokenSentinel)
	}
}
