package craft_test

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/pkg/craft"
)

// TestProtocolVerbs asserts ProtocolVerbs() matches the OpVerb* constants
// exactly, in the sorted order the internal set already guarantees. This
// fails if the two lists ever drift apart, unlike a check that only asserts
// the result is non-empty.
func TestProtocolVerbs(t *testing.T) {
	want := []string{
		craft.OpVerbDELETE,
		craft.OpVerbGET,
		craft.OpVerbGRPC,
		craft.OpVerbHEAD,
		craft.OpVerbOPTIONS,
		craft.OpVerbPATCH,
		craft.OpVerbPOST,
		craft.OpVerbPUT,
		craft.OpVerbQUERY,
		craft.OpVerbTOPIC,
	}
	if got := craft.ProtocolVerbs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ProtocolVerbs() = %v, want %v", got, want)
	}
}
