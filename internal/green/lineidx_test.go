package green_test

import (
	"testing"

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
		got := li.UTF16Col(src, green.TextSize(i))
		want := uint32(i)
		if got != want {
			t.Errorf("UTF16Col(%d) = %d, want %d", i, got, want)
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
