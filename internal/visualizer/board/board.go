package board

import (
	"math"
	"sort"
	"strconv"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

const (
	legendW   = 290.0 // legend panel plus gap
	legendY   = 70.0
	pad       = 28.0
	titleBand = 76.0
	minBoardW = 1000.0
	minBoardH = 620.0
	font      = "'Open Sans', Arial, Helvetica, sans-serif"
)

// Flow writes f as a complete SVG document: the notation legend beside the
// board, with title heading it ("" draws no title band).
func Flow(f view.Flow, title string) []byte {
	at := layout(f)
	r := route(f, at)
	ids := make([]string, 0, len(at))
	for id := range at {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Content bounds: nodes, notes and service boxes. A note or box may
	// reach past the nodes' top-left, so the content is shifted by (ox, oy)
	// to start at the margin.
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(a0, b0, a1, b1 float64) {
		x0, y0, x1, y1 = math.Min(x0, a0), math.Min(y0, b0), math.Max(x1, a1), math.Max(y1, b1)
	}
	withStore, withStep := false, false
	for _, id := range ids {
		grow(at[id].bounds())
		withStore = withStore || at[id].Shape == shapeStore
	}
	for _, n := range r.Notes {
		grow(n.X-noteW/2, n.Y-noteH/2-8, n.X+noteW/2, n.Y+noteH/2) // 8: the number badge
		withStep = withStep || n.Kind == noteNeutral
	}
	for _, b := range r.Boxes {
		grow(b.X0, b.Y0, b.X1, b.Y1)
	}
	if len(ids) == 0 {
		x0, y0, x1, y1 = 0, 0, 0, 0
	}
	ox, oy := boardMargin-x0, boardMargin-y0
	bw, bh := x1+ox+boardMargin, y1+oy+boardMargin

	titleH := 0.0
	if title != "" {
		titleH = titleBand
	}
	boardW := math.Max(bw, minBoardW)
	w := legendW + 2*pad + boardW
	h := titleH + math.Max(bh, minBoardH) + pad
	h = math.Max(h, legendY+legendHeight(withStore, withStep)+pad)

	var s svg
	s.open("svg", "xmlns", "http://www.w3.org/2000/svg", "width", num(w), "height", num(h),
		"viewBox", "0 0 "+num(w)+" "+num(h), "font-family", font)
	s.open("defs")
	s.open("filter", "id", "shadow", "x", "-20%", "y", "-20%", "width", "150%", "height", "160%")
	s.el("feDropShadow", "dx", "1", "dy", "1.5", "stdDeviation", "1", "flood-opacity", "0.18")
	s.close("filter")
	for _, m := range [][2]string{{"arrow", "#333"}, {"arrow-grey", "#9a9a9a"}} {
		s.open("marker", "id", m[0], "viewBox", "0 0 10 10", "refX", "9", "refY", "5",
			"markerWidth", "7", "markerHeight", "7", "orient", "auto-start-reverse")
		s.el("path", "d", "M0,1L10,5L0,9Z", "fill", m[1])
		s.close("marker")
	}
	s.close("defs")

	s.el("rect", "width", num(w), "height", num(h), "fill", "#fff")
	panelX := legendW + pad
	if title != "" {
		s.text(panelX+(w-panelX)/2, titleH/2+4, title, 30, 700, "middle", 1000, "#1a1a1a")
	}
	s.drawLegend(point{14, legendY}, withStore, withStep)

	s.open("g", "transform", "translate("+num(legendW+2*pad+(boardW-bw)/2+ox)+","+num(titleH+10+oy)+")")
	for _, b := range r.Boxes {
		s.el("rect", "x", num(b.X0), "y", num(b.Y0), "width", num(b.X1-b.X0), "height", num(b.Y1-b.Y0),
			"fill", "none", "stroke", "#8a8a8a", "stroke-width", "1.2", "stroke-dasharray", "4 4")
	}
	for _, id := range ids {
		p, c := at[id], point{at[id].X, at[id].Y}
		switch p.Shape {
		case shapeCloud:
			s.drawCloud(c, p.W, p.H)
			s.text(c.X, c.Y+2, p.Node.Text(), 15, 700, "middle", 10, "#1a1a1a")
		case shapePerson:
			s.drawPerson(c, p.H)
		case shapeStore:
			s.drawStore(c, p.W)
		default:
			s.drawCog(c, p.W/2)
		}
	}
	for _, a := range r.Arrows {
		stroke, marker := "#333", "url(#arrow)"
		if a.Inferred {
			stroke, marker = "#9a9a9a", "url(#arrow-grey)"
		}
		attrs := []string{"d", "M" + num(a.P0.X) + "," + num(a.P0.Y) + "C" + num(a.C1.X) + "," + num(a.C1.Y) + " " +
			num(a.C2.X) + "," + num(a.C2.Y) + " " + num(a.P3.X) + "," + num(a.P3.Y),
			"fill", "none", "stroke", stroke, "stroke-width", "1.4", "marker-end", marker}
		if a.Dashed {
			attrs = append(attrs, "stroke-dasharray", "6 5")
		}
		s.el("path", attrs...)
	}
	for _, b := range r.Badges {
		s.at(point{b.X, b.Y})
		s.el("circle", "r", "8", "fill", "#fff", "stroke", "#8a8a8a", "stroke-width", "0.8")
		s.text(0, 0, strconv.Itoa(b.Number), 8, 400, "middle", 99, "#d33")
		s.close("g")
	}
	for _, n := range r.Notes {
		s.drawNote(point{n.X, n.Y}, n.Kind, n.Number, n.Label, n.Inferred)
	}
	// Icon names go right of the icon, in the room layout kept for them.
	for _, id := range ids {
		if p := at[id]; p.Shape != shapeCloud {
			s.text(p.X+p.W/2+8, p.Y, p.Node.Text(), 11, 400, "start", 99, "#1a1a1a")
		}
	}
	s.close("g")
	s.close("svg")
	return []byte(s.b.String())
}
