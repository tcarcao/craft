package board

import (
	"math"
	"reflect"
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
