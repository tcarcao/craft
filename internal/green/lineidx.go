package green

// TextSize is a byte offset or width.
// Named type prevents mixing with line numbers.
// All widths and offsets are in bytes. LSP UTF-16 character offsets are
// converted at the LineIndex boundary — nowhere else.
type TextSize int

// TextRange is a half-open byte interval [Start, End).
type TextRange struct {
	Start TextSize
	End   TextSize
}

func (r TextRange) Len() TextSize            { return r.End - r.Start }
func (r TextRange) Contains(p TextSize) bool { return p >= r.Start && p < r.End }

// LineIndex maps between byte offsets and 1-based (line, col) positions.
// Built once from the source string; immutable thereafter.
//
// It keeps the source it was built from, so UTF-16 conversions cannot be
// handed a mismatched string. Strings are immutable, so this is a header copy,
// not a duplicate of the text.
type LineIndex struct {
	// newlineOffsets[i] = byte offset of the '\n' that ends line i (0-indexed).
	newlineOffsets []TextSize
	src            string
}

// NewLineIndex builds a LineIndex from the source string in one O(n) pass.
func NewLineIndex(src string) LineIndex {
	var offsets []TextSize
	for i, ch := range src {
		if ch == '\n' {
			offsets = append(offsets, TextSize(i))
		}
	}
	return LineIndex{newlineOffsets: offsets, src: src}
}

// Src returns the source text the index was built from.
func (li LineIndex) Src() string { return li.src }

// UTF16Len returns the length of s in UTF-16 code units — the unit LSP uses
// for character offsets and for token lengths in semantic tokens.
func UTF16Len(s string) uint32 {
	var n uint32
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// LineCol converts a byte offset to a 1-based (line, col).
func (li LineIndex) LineCol(offset TextSize) (line, col int) {
	lo, hi := 0, len(li.newlineOffsets)
	for lo < hi {
		mid := (lo + hi) / 2
		if li.newlineOffsets[mid] < offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	line = lo + 1
	lineStart := TextSize(0)
	if lo > 0 {
		lineStart = li.newlineOffsets[lo-1] + 1
	}
	col = int(offset-lineStart) + 1
	return
}

// Offset converts a 1-based (line, col) to a byte offset.
func (li LineIndex) Offset(line, col int) TextSize {
	lineStart := TextSize(0)
	if line > 1 {
		idx := line - 2
		if idx < len(li.newlineOffsets) {
			lineStart = li.newlineOffsets[idx] + 1
		} else {
			// Line number beyond EOF — clamp to the start of the last known line.
			if len(li.newlineOffsets) > 0 {
				lineStart = li.newlineOffsets[len(li.newlineOffsets)-1] + 1
			}
		}
	}
	return lineStart + TextSize(col-1)
}

// OffsetFromUTF16 converts an LSP position to a byte offset. line is 1-based,
// utf16col is a 0-based count of UTF-16 code units within that line — the unit
// LSP uses for Position.Character. This is the inverse of UTF16Col and the only
// correct way to turn an LSP position into a byte offset.
//
// Do NOT use Offset(line, col) for LSP positions: it adds the column straight
// onto a byte line-start, which is only right for ASCII lines.
//
// A column past the end of the line clamps to the line end (before its
// newline); a line past EOF clamps to len(src).
func (li LineIndex) OffsetFromUTF16(line, utf16col int) TextSize {
	start, end := li.lineBounds(line)
	if utf16col <= 0 {
		return start
	}
	var n int
	for i, r := range li.src[start:end] {
		if n >= utf16col {
			return start + TextSize(i)
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return end
}

// lineBounds returns the byte range of the 1-based line, excluding its
// terminating newline. Out-of-range lines clamp to the end of the source.
func (li LineIndex) lineBounds(line int) (start, end TextSize) {
	n := TextSize(len(li.src))
	switch {
	case line <= 1:
		start = 0
	case line-2 < len(li.newlineOffsets):
		start = li.newlineOffsets[line-2] + 1
	default:
		start = n
	}
	if line-1 < len(li.newlineOffsets) && line >= 1 {
		end = li.newlineOffsets[line-1]
	} else {
		end = n
	}
	if start > n {
		start = n
	}
	if end > n {
		end = n
	}
	if end < start {
		end = start
	}
	return start, end
}

// UTF16Col converts a byte offset to a 0-based UTF-16 character index
// for use at the LSP boundary. For ASCII content this equals col-1.
// For non-ASCII it accounts for surrogate pairs.
func (li LineIndex) UTF16Col(offset TextSize) uint32 {
	// Clamp defensively: a caller handing us an offset past EOF must degrade to
	// the end of the last line, never panic. This is the LSP boundary, and a
	// panic here takes down the whole request handler.
	if offset > TextSize(len(li.src)) {
		offset = TextSize(len(li.src))
	}
	if offset < 0 {
		offset = 0
	}
	_, col := li.LineCol(offset)
	lineStart := offset - TextSize(col-1)
	if lineStart < 0 {
		lineStart = 0
	}
	return UTF16Len(li.src[lineStart:offset])
}

// LineCol16 is LineCol with the column measured in UTF-16 code units instead of
// bytes: a 1-based line and a 1-based UTF-16 column. Use this — not LineCol —
// whenever the column is destined for an LSP Position.Character, which is
// every column the server publishes.
func (li LineIndex) LineCol16(offset TextSize) (line, col int) {
	line, _ = li.LineCol(offset)
	return line, int(li.UTF16Col(offset)) + 1
}
