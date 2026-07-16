package syntax_test

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestGlossaryVerbs(t *testing.T) {
	got := syntax.GlossaryVerbs()
	want := []string{"same_as", "contrasts", "distinct_from"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GlossaryVerbs() = %v, want %v", got, want)
	}
}
