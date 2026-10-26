package board

import (
	"fmt"
	"math"
	"strconv"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

type shapeKind int

const (
	shapeCloud shapeKind = iota
	shapeCog
	shapePerson
	shapeStore
)

var nodeSize = map[shapeKind]point{shapeCloud: {190, 110}, shapeCog: {46, 46}, shapePerson: {40, 46}, shapeStore: {40, 46}}

const noteW, noteH = 84.0, 56.0

// shapeOf picks the shape a node is drawn with.
func shapeOf(n view.Node) shapeKind {
	switch n.Kind {
	case view.NodeContext:
		switch n.Participant {
		case model.ParticipantBoundedContext, model.ParticipantDomain:
			return shapeCloud
		case model.ParticipantDataStore:
			return shapeStore
		}
	case view.NodeStore:
		return shapeStore
	case view.NodeActor:
		if n.ActorType == model.ActorTypeUser || n.ActorType == "" {
			return shapePerson
		}
	}
	return shapeCog
}

// noteKind selects a sticky note's colour.
type noteKind int

const (
	noteNeutral noteKind = iota
	noteQuery
	noteCommand
	noteEvent
)

var noteFill = map[noteKind]string{noteNeutral: "#ffffff", noteQuery: "#d6e46c", noteCommand: "#4a90e2", noteEvent: "#f2c33c"}

// cloudNotches are the Miro-style cloud's notch positions (fractions of w×h,
// clockwise from top-left), each with the bulge (sagitta / chord) of the lobe
// running to the next notch.
var cloudNotches = [][3]float64{{-.36, -.197, .24}, {-.164, -.413, .22}, {.128, -.376, .24}, {.364, -.197, .46},
	{.379, .25, .24}, {.128, .395, .25}, {-.143, .387, .26}, {-.399, .132, .53}}

// lobe returns the arc radius of the lobe from notch i to the next, and its
// endpoints scaled to w×h.
func lobe(i int, w, h float64) (a, b point, r, k float64) {
	n := cloudNotches[i]
	m := cloudNotches[(i+1)%len(cloudNotches)]
	a, b, k = point{n[0] * w, n[1] * h}, point{m[0] * w, m[1] * h}, n[2]
	c := math.Hypot(b.X-a.X, b.Y-a.Y)
	s := k * c
	return a, b, (c*c/4 + s*s) / (2 * s), k
}

// cloudPath is the cloud outline as SVG path data, centred on 0,0.
func cloudPath(w, h float64) string {
	a, _, _, _ := lobe(0, w, h)
	d := "M" + num(a.X) + "," + num(a.Y)
	for i := range cloudNotches {
		_, b, r, k := lobe(i, w, h)
		large := 0
		if k > .5 {
			large = 1
		}
		d += fmt.Sprintf("A%s,%s 0 %d,1 %s,%s", num(r), num(r), large, num(b.X), num(b.Y))
	}
	return d + "Z"
}

// cloudOutline is the cloud as a polygon, each lobe sampled at 12 points.
func cloudOutline(w, h float64) []point {
	var pts []point
	for i := range cloudNotches {
		a, b, r, k := lobe(i, w, h)
		// Centre of the arc: sweep 1 (clockwise on screen) puts it left of
		// a→b when the lobe is minor and right of it when major.
		mx, my := (a.X+b.X)/2, (a.Y+b.Y)/2
		c := math.Hypot(b.X-a.X, b.Y-a.Y)
		off := math.Sqrt(math.Max(r*r-c*c/4, 0))
		if k > .5 {
			off = -off
		}
		cx, cy := mx+off*(a.Y-b.Y)/c, my+off*(b.X-a.X)/c
		a0, a1 := math.Atan2(a.Y-cy, a.X-cx), math.Atan2(b.Y-cy, b.X-cx)
		if a1 < a0 {
			a1 += 2 * math.Pi
		}
		for j := 0; j < 12; j++ {
			t := a0 + (a1-a0)*float64(j)/12
			pts = append(pts, point{cx + r*math.Cos(t), cy + r*math.Sin(t)})
		}
	}
	return pts
}

// at opens a group translated to the centre; the caller closes it.
func (s *svg) at(c point) { s.open("g", "transform", "translate("+num(c.X)+","+num(c.Y)+")") }

func (s *svg) drawCloud(c point, w, h float64) {
	s.at(c)
	s.el("path", "d", cloudPath(w, h), "fill", "#cdcdfa", "stroke", "#6130b8",
		"stroke-width", num(math.Max(1, math.Min(w, h)/45)), "stroke-linejoin", "round")
	s.close("g")
}

// drawCog draws the chunky cog: disc, 8 square teeth, white ring, hub.
func (s *svg) drawCog(c point, r float64) {
	s.at(c)
	s.el("circle", "r", num(r*0.74), "fill", "#111")
	for i := 0; i < 8; i++ {
		s.el("rect", "x", num(-r*0.19), "y", num(-r), "width", num(r*0.38), "height", num(r*0.4),
			"rx", num(r*0.06), "fill", "#111", "transform", "rotate("+strconv.Itoa(i*45)+")")
	}
	s.el("circle", "r", num(r*0.44), "fill", "#fff")
	s.el("circle", "r", num(r*0.26), "fill", "#111")
	s.close("g")
}

func (s *svg) drawPerson(c point, size float64) {
	s.at(c)
	s.el("circle", "cy", num(-size*0.28), "r", num(size*0.22), "fill", "#111")
	s.el("path", "fill", "#111", "d", fmt.Sprintf("M%s,%sC%s,%s %s,%s %s,%sZ",
		num(-size*0.46), num(size*0.42), num(-size*0.46), num(size*0.02),
		num(size*0.46), num(size*0.02), num(size*0.46), num(size*0.42)))
	s.close("g")
}

// drawStore draws a flat cylinder of the same visual weight as the cog.
func (s *svg) drawStore(c point, size float64) {
	w, ry := size*0.8, size*0.13
	top, bot := -size*0.34, size*0.34
	s.at(c)
	s.el("path", "fill", "#111", "d", fmt.Sprintf("M%s,%sL%s,%sA%s,%s 0 0,1 %s,%sL%s,%sZ",
		num(-w/2), num(top), num(-w/2), num(bot), num(w/2), num(ry), num(w/2), num(bot), num(w/2), num(top)))
	s.el("ellipse", "cy", num(top), "rx", num(w/2), "ry", num(ry), "fill", "#111", "stroke", "#fff", "stroke-width", "1.5")
	s.close("g")
}

// drawNote draws a sticky note; number 0 draws no number badge, and an
// inferred note is faded.
func (s *svg) drawNote(c point, kind noteKind, number int, label string, inferred bool) {
	attrs := []string{"transform", "translate(" + num(c.X) + "," + num(c.Y) + ")"}
	if inferred {
		attrs = append(attrs, "opacity", "0.55")
	}
	s.open("g", attrs...)
	s.el("rect", "x", num(-noteW/2), "y", num(-noteH/2), "width", num(noteW), "height", num(noteH),
		"fill", noteFill[kind], "stroke", "rgba(0,0,0,.25)", "stroke-width", "0.8", "filter", "url(#shadow)")
	s.text(0, -noteH/2+14, label, 8.5, 700, "middle", 18, "#1a1a1a")
	if number > 0 {
		s.el("circle", "cy", num(-noteH/2), "r", "8", "fill", "#fff", "stroke", "#8a8a8a", "stroke-width", "0.8")
		s.text(0, -noteH/2, strconv.Itoa(number), 8, 400, "middle", 99, "#d33")
	}
	s.close("g")
}

// drawLegend draws the notation panel with its top-left corner at p. The
// data store and step rows are optional.
func (s *svg) drawLegend(p point, withStore, withStep bool) {
	type row struct {
		draw  func(c point)
		label string
	}
	miniNote := func(kind noteKind, word string) func(point) {
		return func(c point) {
			s.at(c)
			s.el("rect", "x", "-38", "y", "-21", "width", "76", "height", "42", "fill", noteFill[kind], "filter", "url(#shadow)")
			s.text(0, 0, word, 11, 400, "middle", 10, "#1a1a1a")
			s.close("g")
		}
	}
	rows := []row{
		{func(c point) { s.drawPerson(c, 44) }, "Actor / User / Persona"},
		{func(c point) { s.drawCloud(c, 62, 52) }, "Bounded Context"},
		{func(c point) { s.drawCog(c, 21) }, "System"},
	}
	if withStore {
		rows = append(rows, row{func(c point) { s.drawStore(c, 40) }, "Data store"})
	}
	rows = append(rows,
		row{miniNote(noteQuery, "Get Pizza"), "Query"},
		row{miniNote(noteCommand, "Slice Pineapple"), "Command"},
		row{miniNote(noteEvent, "Pizza Baked"), "Event"})
	if withStep {
		rows = append(rows, row{func(c point) { s.drawNote(c, noteNeutral, 1, "", false) }, "Step"})
	}
	rows = append(rows, row{func(c point) {
		s.at(c)
		s.el("path", "d", "M-30,12L30,-8", "stroke", "#666", "stroke-dasharray", "2 2", "marker-end", "url(#arrow)")
		s.close("g")
	}, "Direction of message"})

	const rowH = 70.0
	height := 90 + float64(len(rows))*rowH - 20
	s.at(p)
	s.el("rect", "width", "260", "height", num(height), "fill", "#e6e6e6", "stroke", "#9a9a9a", "stroke-width", "1.5")
	s.text(130, 30, "Notation", 18, 700, "middle", 99, "#1a1a1a")
	for i, r := range rows {
		cy := 90 + float64(i)*rowH
		r.draw(point{50, cy})
		s.text(105, cy, r.label, 13, 400, "start", 16, "#1a1a1a")
	}
	s.text(105, 90+float64(len(rows)-1)*rowH+20, "From producer to consumer", 8, 400, "start", 99, "#1a1a1a")
	s.close("g")
}
