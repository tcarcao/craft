package green_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
)

// SyntaxKind constants for testing — mirrors syntax package values.
const (
	kindFile      green.SyntaxKind = 1000
	kindActorDecl green.SyntaxKind = 1001
	kindKwActor   green.SyntaxKind = 21
	kindIdent     green.SyntaxKind = 5
)

func TestGreenToken_Width(t *testing.T) {
	tok := &green.GreenToken{Kind: kindIdent, Text: "hello"}
	if got := tok.Width(); got != 5 {
		t.Errorf("Width() = %d, want 5", got)
	}
}

func TestGreenToken_Width_UTF8(t *testing.T) {
	// "café" = 4 runes but 5 bytes (é is 2 bytes in UTF-8)
	tok := &green.GreenToken{Kind: kindIdent, Text: "café"}
	if got := tok.Width(); got != green.TextSize(len("café")) {
		t.Errorf("Width() = %d, want %d (byte count)", got, len("café"))
	}
}

func TestNewGreenNode_Width(t *testing.T) {
	tok1 := &green.GreenToken{Kind: kindKwActor, Text: "actor"} // 5 bytes
	tok2 := &green.GreenToken{Kind: kindIdent, Text: " Foo"}    // 4 bytes
	node := green.NewGreenNode(kindActorDecl, []green.GreenElement{tok1, tok2})
	if got := node.Width(); got != 9 {
		t.Errorf("Width() = %d, want 9", got)
	}
}

func TestNewGreenNode_ChildCount(t *testing.T) {
	tok := &green.GreenToken{Kind: kindKwActor, Text: "actor"}
	node := green.NewGreenNode(kindActorDecl, []green.GreenElement{tok})
	if len(node.Children) != 1 {
		t.Errorf("Children len = %d, want 1", len(node.Children))
	}
}

func TestNewGreenNode_NestedWidth(t *testing.T) {
	inner := green.NewGreenNode(kindActorDecl, []green.GreenElement{
		&green.GreenToken{Kind: kindIdent, Text: "abc"},
	})
	outer := green.NewGreenNode(kindFile, []green.GreenElement{inner})
	if got := outer.Width(); got != 3 {
		t.Errorf("Nested Width() = %d, want 3", got)
	}
}
