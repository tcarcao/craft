// Package board writes Craft's flow view as a Miro-style SVG board.
package board

import (
	"math"
	"strconv"
	"strings"
)

type point struct{ X, Y float64 }

// num formats a coordinate to one decimal, without a trailing ".0".
func num(v float64) string {
	r := math.Round(v*10) / 10
	if r == 0 {
		return "0" // avoids "-0"
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")

// esc escapes text for use as XML content or an attribute value.
func esc(s string) string { return escaper.Replace(s) }

// wrap breaks s greedily into lines of at most maxChars runes; a word longer
// than that keeps a line of its own.
func wrap(s string, maxChars int) []string {
	var lines []string
	for _, w := range strings.Fields(s) {
		if n := len(lines); n > 0 && len([]rune(lines[n-1]))+1+len([]rune(w)) <= maxChars {
			lines[n-1] += " " + w
		} else {
			lines = append(lines, w)
		}
	}
	return lines
}

// svg accumulates markup. Attributes are passed as alternating name, value
// pairs; values are escaped here.
type svg struct{ b strings.Builder }

func (s *svg) tag(name string, attrs []string) {
	s.b.WriteString("<" + name)
	for i := 0; i+1 < len(attrs); i += 2 {
		s.b.WriteString(" " + attrs[i] + `="` + esc(attrs[i+1]) + `"`)
	}
}

// el writes a self-closing element.
func (s *svg) el(name string, attrs ...string) {
	s.tag(name, attrs)
	s.b.WriteString("/>")
}

func (s *svg) open(name string, attrs ...string) {
	s.tag(name, attrs)
	s.b.WriteString(">")
}

func (s *svg) close(name string) { s.b.WriteString("</" + name + ">") }

// text writes one or more lines of text centred (or anchored) on x, with the
// block vertically centred on y.
func (s *svg) text(x, y float64, str string, size float64, weight int, anchor string, maxChars int, fill string) {
	lines := wrap(str, maxChars)
	lh := size * 1.25
	s.open("text", "x", num(x), "y", num(y-float64(len(lines)-1)*lh/2), "font-size", num(size),
		"font-weight", strconv.Itoa(weight), "text-anchor", anchor, "dominant-baseline", "central", "fill", fill)
	for i, l := range lines {
		dy := 0.0
		if i > 0 {
			dy = lh
		}
		s.open("tspan", "x", num(x), "dy", num(dy))
		s.b.WriteString(esc(l))
		s.close("tspan")
	}
	s.close("text")
}
