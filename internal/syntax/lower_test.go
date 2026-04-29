package syntax_test

import (
	"encoding/json"
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestLower_ParityWithParse(t *testing.T) {
	fixtures := []string{
		`actor user Alice`,
		`actor system BackendAPI`,
		`actors { user Alice  system Bob }`,
		`domain Ordering { Cart Checkout }`,
		`domains { Ordering { Cart } Payments { Billing } }`,
		`service order-service { contexts: [Cart] }`,
		`use_case "Pay" { when Customer asks PaymentService }`,
	}
	for _, src := range fixtures {
		t.Run(src, func(t *testing.T) {
			// Legacy: Parse() output (current gold standard)
			legacyFile, _ := syntax.Parse(src)

			// New: ParseTree() + Lower()
			tree, _, _ := syntax.ParseTree(src)
			loweredFile := syntax.Lower(tree)

			legacyJSON, _ := json.Marshal(legacyFile)
			loweredJSON, _ := json.Marshal(loweredFile)
			if string(legacyJSON) != string(loweredJSON) {
				t.Errorf("parity failure:\nlegacy:  %s\nlowered: %s",
					legacyJSON, loweredJSON)
			}
		})
	}
}
