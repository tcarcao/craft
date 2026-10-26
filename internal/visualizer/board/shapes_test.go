package board

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

func TestNum(t *testing.T) {
	for in, want := range map[float64]string{1: "1", 1.25: "1.3", -0.04: "0", 12.345: "12.3"} {
		if got := num(in); got != want {
			t.Errorf("num(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestEsc(t *testing.T) {
	if got, want := esc(`a & <b> "c"`), "a &amp; &lt;b&gt; &#34;c&#34;"; got != want {
		t.Errorf("esc = %q, want %q", got, want)
	}
}

func TestWrap_LongWordOwnLine(t *testing.T) {
	got := wrap("get the averyveryverylongwordindeed now", 10)
	want := []string{"get the", "averyveryverylongwordindeed", "now"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrap = %q, want %q", got, want)
	}
}

func TestCloudOutline_FitsBox(t *testing.T) {
	var maxX, maxY float64
	for _, p := range cloudOutline(190, 110) {
		ax, ay := math.Abs(p.X), math.Abs(p.Y)
		if ax > 96 || ay > 56 {
			t.Fatalf("point %v outside the box", p)
		}
		maxX, maxY = math.Max(maxX, ax), math.Max(maxY, ay)
	}
	if maxX < 85.5 || maxY < 49.5 {
		t.Errorf("outline reaches only %v x %v", maxX, maxY)
	}
}

func TestShapeOf(t *testing.T) {
	ctx := func(p model.ParticipantKind) view.Node { return view.Node{Kind: view.NodeContext, Participant: p} }
	act := func(a model.ActorType) view.Node { return view.Node{Kind: view.NodeActor, ActorType: a} }
	for name, tc := range map[string]struct {
		n    view.Node
		want shapeKind
	}{
		"bounded context": {ctx(model.ParticipantBoundedContext), shapeCloud},
		"domain":          {ctx(model.ParticipantDomain), shapeCloud},
		"service":         {ctx(model.ParticipantService), shapeCog},
		"data store":      {ctx(model.ParticipantDataStore), shapeStore},
		"unresolved":      {ctx(""), shapeCog},
		"queue":           {view.Node{Kind: view.NodeQueue}, shapeCog},
		"external":        {view.Node{Kind: view.NodeExternal}, shapeCog},
		"store node":      {view.Node{Kind: view.NodeStore}, shapeStore},
		"user":            {act(model.ActorTypeUser), shapePerson},
		"undeclared":      {act(""), shapePerson},
		"system":          {act(model.ActorTypeSystem), shapeCog},
		"scheduler":       {act(model.ActorTypeScheduler), shapeCog},
		"tool":            {act(model.ActorTypeTool), shapeCog},
	} {
		if got := shapeOf(tc.n); got != tc.want {
			t.Errorf("%s: shapeOf = %v, want %v", name, got, tc.want)
		}
	}
}

func TestDrawStore_BottomBulgesDown(t *testing.T) {
	var s svg
	s.drawStore(point{}, 40)
	// Sweep 0 from bottom-left to bottom-right curves downward on screen.
	if !strings.Contains(s.b.String(), " 0 0,0 ") {
		t.Errorf("store bottom arc must use sweep 0: %s", s.b.String())
	}
}

func TestDrawNote_FirstLineAtFixedY(t *testing.T) {
	for _, label := range []string{"Get", "Get customer details now please ok"} {
		var s svg
		s.drawNote(point{}, noteQuery, 1, label, false)
		out := s.b.String()
		// The text element's y is its first line's y (tspan dy 0).
		var y float64
		fmt.Sscanf(out[strings.Index(out, "<text "):], `<text x="0" y="%f"`, &y)
		if want := -noteH/2 + 14; y != want {
			t.Errorf("%q: first line y = %v, want %v", label, y, want)
		}
	}
}

func TestText_EmptyEmitsNothing(t *testing.T) {
	var s svg
	s.text(0, 0, "", 8, 400, "middle", 10, "#000")
	if s.b.Len() != 0 {
		t.Errorf("empty text wrote %q", s.b.String())
	}
}
