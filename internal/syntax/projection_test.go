package syntax_test

import (
	"encoding/json"
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

func TestProjectFromTree_ParityWithProject(t *testing.T) {
	fixtures := []string{
		`actor user Alice`,
		`actor system BackendAPI`,
		`actors { user Alice  system Bob }`,
		`domain Ordering { Cart Checkout }`,
		`service order-service { contexts: [Cart] }`,
		"use_case \"Pay\" { when Customer submits PaymentForm\n    PaymentService asks Bank }",
	}
	for _, src := range fixtures {
		label := src
		if len(label) > 40 {
			label = label[:40]
		}
		t.Run(label, func(t *testing.T) {
			treeA, _ := syntax.Parse(src)
			astFile := syntax.Lower(treeA)
			legacyDoc := syntax.Project(astFile)

			tree, _ := syntax.Parse(src)
			newDoc := syntax.ProjectFromTree(tree)

			legacyJSON, _ := json.Marshal(legacyDoc)
			newJSON, _ := json.Marshal(newDoc)
			if string(legacyJSON) != string(newJSON) {
				t.Errorf("parity failure:\nlegacy: %s\nnew:    %s", legacyJSON, newJSON)
			}
		})
	}
}
