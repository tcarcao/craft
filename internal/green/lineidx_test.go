package green_test

import (
	"testing"
	"unicode/utf8"

	"github.com/tcarcao/craft/v2/internal/green"
)

func TestLineIndex_LineCol(t *testing.T) {
	src := "hello\nworld\nfoo"
	li := green.NewLineIndex(src)

	tests := []struct {
		offset   green.TextSize
		wantLine int
		wantCol  int
	}{
		{0, 1, 1},  // 'h'
		{4, 1, 5},  // 'o'
		{5, 1, 6},  // '\n' — still on line 1
		{6, 2, 1},  // 'w'
		{11, 2, 6}, // '\n'
		{12, 3, 1}, // 'f'
		{14, 3, 3}, // 'o' (last char)
	}
	for _, tt := range tests {
		line, col := li.LineCol(tt.offset)
		if line != tt.wantLine || col != tt.wantCol {
			t.Errorf("LineCol(%d) = (%d,%d), want (%d,%d)",
				tt.offset, line, col, tt.wantLine, tt.wantCol)
		}
	}
}

func TestLineIndex_Offset(t *testing.T) {
	src := "hello\nworld\nfoo"
	li := green.NewLineIndex(src)

	tests := []struct {
		line, col  int
		wantOffset green.TextSize
	}{
		{1, 1, 0},
		{1, 5, 4},
		{2, 1, 6},
		{3, 1, 12},
		{3, 3, 14},
	}
	for _, tt := range tests {
		got := li.Offset(tt.line, tt.col)
		if got != tt.wantOffset {
			t.Errorf("Offset(%d,%d) = %d, want %d", tt.line, tt.col, got, tt.wantOffset)
		}
	}
}

func TestLineIndex_RoundTrip(t *testing.T) {
	src := "actor user Foo\nuse_case \"Bar\" {\n  when Foo\n}"
	li := green.NewLineIndex(src)
	for offset := green.TextSize(0); offset < green.TextSize(len(src)); offset++ {
		line, col := li.LineCol(offset)
		got := li.Offset(line, col)
		if got != offset {
			t.Errorf("RoundTrip(%d): LineCol→(%d,%d)→Offset=%d", offset, line, col, got)
		}
	}
}

func TestLineIndex_UTF16Col_ASCII(t *testing.T) {
	src := "hello world"
	li := green.NewLineIndex(src)
	// ASCII: UTF-16 col == byte index
	for i := range src {
		got := li.UTF16Col(green.TextSize(i))
		want := uint32(i)
		if got != want {
			t.Errorf("UTF16Col(%d) = %d, want %d", i, got, want)
		}
	}
}

func TestLineIndex_UTF16Col_NonASCII(t *testing.T) {
	// "café" is 5 bytes / 4 UTF-16 units; "🚀" is 4 bytes / 2 UTF-16 units.
	// Bytes: c0 a1 f2 é3-4 ' '5 x6 \n7 🚀8-11 ' '12 y13.
	src := "café x\n🚀 y"
	li := green.NewLineIndex(src)

	tests := []struct {
		offset green.TextSize
		want   uint32
	}{
		{0, 0},  // 'c'
		{3, 3},  // 'é' starts at byte 3, UTF-16 index 3
		{5, 4},  // ' ' after é
		{6, 5},  // 'x'
		{8, 0},  // '🚀' at start of line 2
		{12, 2}, // ' ' after the rocket — the pair counts as 2 units
		{13, 3}, // 'y'
	}
	for _, tt := range tests {
		if got := li.UTF16Col(tt.offset); got != tt.want {
			t.Errorf("UTF16Col(%d) = %d, want %d", tt.offset, got, tt.want)
		}
	}
}

// UTF16Col must never panic on an offset past EOF. A tree-width bug used to
// hand it one and take down the whole textDocument/semanticTokens/full handler.
func TestLineIndex_UTF16Col_OffsetPastEOF(t *testing.T) {
	src := "hello\nworld"
	li := green.NewLineIndex(src)
	if got := li.UTF16Col(green.TextSize(len(src) + 7)); got != 5 {
		t.Errorf("UTF16Col past EOF = %d, want 5 (clamped to end of last line)", got)
	}
}

func TestLineIndex_OffsetFromUTF16(t *testing.T) {
	// Bytes: c0 a1 f2 é3-4 ' '5 x6 \n7 | 🚀8-11 ' '12 y13 \n14 | p15 l16 a17 i18 n19.
	src := "café x\n🚀 y\nplain"
	li := green.NewLineIndex(src)

	tests := []struct {
		name       string
		line, col  int // 1-based line, 0-based UTF-16 column
		wantOffset green.TextSize
	}{
		{"start of file", 1, 0, 0},
		{"before multi-byte rune", 1, 3, 3},
		{"after 2-byte rune", 1, 4, 5},
		{"end of line 1", 1, 6, 7},
		{"start of line 2", 2, 0, 8},
		{"after surrogate pair", 2, 2, 12},
		{"line 3", 3, 3, 18},
		{"column past end of line clamps", 1, 99, 7},
		{"line past EOF clamps", 99, 0, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := li.OffsetFromUTF16(tt.line, tt.col); got != tt.wantOffset {
				t.Errorf("OffsetFromUTF16(%d,%d) = %d, want %d", tt.line, tt.col, got, tt.wantOffset)
			}
		})
	}
}

// OffsetFromUTF16 is the exact inverse of UTF16Col at every rune boundary.
func TestLineIndex_UTF16RoundTrip(t *testing.T) {
	src := "actor user Ação\nuse_case \"Café 🚀\" {\n  when Foo\n}"
	li := green.NewLineIndex(src)
	for offset := range len(src) + 1 {
		if offset < len(src) && !utf8.RuneStart(src[offset]) {
			continue // mid-rune offsets have no UTF-16 column
		}
		line, _ := li.LineCol(green.TextSize(offset))
		col := li.UTF16Col(green.TextSize(offset))
		got := li.OffsetFromUTF16(line, int(col))
		if got != green.TextSize(offset) {
			t.Errorf("round trip at %d: (line %d, utf16 %d) -> %d", offset, line, col, got)
		}
	}
}

func TestTextRange_Contains(t *testing.T) {
	r := green.TextRange{Start: 5, End: 10}
	if !r.Contains(5) {
		t.Error("should contain start")
	}
	if !r.Contains(9) {
		t.Error("should contain end-1")
	}
	if r.Contains(10) {
		t.Error("should not contain end (half-open)")
	}
	if r.Contains(4) {
		t.Error("should not contain before start")
	}
}
