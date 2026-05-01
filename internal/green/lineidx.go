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
type LineIndex struct {
	// newlineOffsets[i] = byte offset of the '\n' that ends line i (0-indexed).
	newlineOffsets []TextSize
}

// NewLineIndex builds a LineIndex from the source string in one O(n) pass.
func NewLineIndex(src string) LineIndex {
	var offsets []TextSize
	for i, ch := range src {
		if ch == '\n' {
			offsets = append(offsets, TextSize(i))
		}
	}
	return LineIndex{newlineOffsets: offsets}
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

// UTF16Col converts a byte offset to a 0-based UTF-16 character index
// for use at the LSP boundary. For ASCII content this equals col-1.
// For non-ASCII it accounts for surrogate pairs.
func (li LineIndex) UTF16Col(src string, offset TextSize) uint32 {
	_, col := li.LineCol(offset)
	lineStart := offset - TextSize(col-1)
	segment := src[lineStart:offset]
	var n uint32
	for _, r := range segment {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
