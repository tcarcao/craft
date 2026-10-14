// Package visualizer draws diagrams of a Craft model. Render is its whole
// interface: a resolved model and a Request in, the diagram's bytes out.
//
// Inside, internal/interaction derives who talks to whom once, view turns
// that into what the requested diagram shows, and the plantuml or mermaid
// writer spells it out. Turning source into an image happens behind the
// Rasterizer seam.
package visualizer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/markdown"
	"github.com/tcarcao/craft/v2/internal/visualizer/mermaid"
	"github.com/tcarcao/craft/v2/internal/visualizer/plantuml"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// View is what a diagram shows.
type View string

const (
	ViewC4           View = "c4"           // C4 container diagram
	ViewFlow         View = "flow"         // detailed domain diagram: numbered steps
	ViewArchitecture View = "architecture" // contexts grouped by service, unlabeled edges
	ViewSequence     View = "sequence"     // sequence diagram per use case
	ViewSurface      View = "surface"      // operation table per surface (Markdown only)
)

// Backend is the diagram language.
type Backend string

const (
	PlantUML Backend = "plantuml"
	Mermaid  Backend = "mermaid"
)

// Format is what Render returns.
type Format string

const (
	FormatSource   Format = "source"   // diagram source text
	FormatMarkdown Format = "markdown" // Mermaid source in a fenced block under a heading
	FormatPNG      Format = "png"
	FormatSVG      Format = "svg"
	FormatPDF      Format = "pdf"
)

// C4Options are the options of a C4 diagram; see view.C4Options.
type C4Options struct {
	Boundaries    bool
	ShowDatabases bool
	FocusServices []string
	FocusContexts []string
}

// Request says which diagram to draw and how to return it.
type Request struct {
	View    View
	Backend Backend
	Format  Format
	// UseCases keeps only the named use cases (by name or slug). Not valid
	// for C4, which always shows the whole architecture. Events are still
	// paired across the whole model.
	UseCases []string
	// C4 must be set for ViewC4 and only for it.
	C4 *C4Options
	// Title, if set, heads the diagram: a PlantUML title, a Mermaid comment
	// (front matter for a journey), or the markdown heading.
	Title string
	// Journey draws the spine of this journey slug (sequence only): its
	// member use cases, in spine order, on one shared stage. Title defaults
	// to "Journey: <slug>".
	Journey string
}

// Output is a drawn diagram.
type Output struct {
	Data        []byte
	ContentType string
	Ext         string // file extension without the dot
}

// ErrUnsupported is returned (wrapped) for a request that asks for something
// the chosen view or backend cannot do. Nothing is silently dropped.
var ErrUnsupported = errors.New("unsupported request")

// Rasterizer turns diagram source into an image or a PDF.
type Rasterizer interface {
	Rasterize(ctx context.Context, backend Backend, src []byte, format Format) ([]byte, error)
}

// Renderer draws diagrams. Its zero value is not usable; call New.
type Renderer struct {
	raster Rasterizer
}

// New returns a Renderer that uses r for PNG, SVG and PDF output.
func New(r Rasterizer) *Renderer { return &Renderer{raster: r} }

// Render draws one diagram of doc, which must come from analysis (resolved
// participants). It validates req first and returns ErrUnsupported for any
// option the view or backend cannot honour.
func (r *Renderer) Render(doc *craft.CraftDoc, req Request) (Output, error) {
	if err := validate(req); err != nil {
		return Output{}, err
	}
	its := interaction.Derive(doc)
	if req.View == ViewSurface {
		md := markdown.Surfaces(view.BuildSurfaces(doc, its))
		return Output{Data: []byte(md), ContentType: "text/markdown; charset=utf-8", Ext: "md"}, nil
	}
	shown := doc
	if req.Journey != "" {
		j := craft.BuildJourney(doc, req.Journey)
		if len(j.Members) == 0 {
			return Output{}, fmt.Errorf("%w: no use case has journey %q", ErrUnsupported, req.Journey)
		}
		shown, its = spine(doc, its, j)
		if req.Title == "" {
			req.Title = "Journey: " + req.Journey
		}
	}
	if len(req.UseCases) > 0 {
		var missing []string
		shown, missing = filterUseCases(doc, req.UseCases)
		if len(missing) > 0 {
			available := make([]string, 0, len(doc.UseCases))
			for _, uc := range doc.UseCases {
				available = append(available, Slugify(uc.Name))
			}
			return Output{}, fmt.Errorf("no use case matches %v (available: %s)", missing, strings.Join(available, ", "))
		}
		its = keepUseCases(its, doc, shown)
	}

	src := write(shown, its, req)
	switch req.Format {
	case FormatSource:
		src = withTitle(src, req)
		ext := "puml"
		if req.Backend == Mermaid {
			ext = "mmd"
		}
		return Output{Data: []byte(src), ContentType: "text/plain; charset=utf-8", Ext: ext}, nil
	case FormatMarkdown:
		title := req.Title
		if title == "" {
			title = defaultTitle(shown, req.View)
		}
		if req.Journey != "" {
			src = withTitle(src, req)
		}
		md := "# " + title + "\n\n```mermaid\n" + src + "\n```\n"
		return Output{Data: []byte(md), ContentType: "text/markdown; charset=utf-8", Ext: "md"}, nil
	}

	if r.raster == nil {
		return Output{}, fmt.Errorf("%w: no rasterizer configured for %s output", ErrUnsupported, req.Format)
	}
	data, err := r.raster.Rasterize(context.Background(), req.Backend, []byte(withTitle(src, req)), req.Format)
	if err != nil {
		return Output{}, err
	}
	return Output{Data: data, ContentType: contentTypes[req.Format], Ext: string(req.Format)}, nil
}

var contentTypes = map[Format]string{
	FormatPNG: "image/png",
	FormatSVG: "image/svg+xml",
	FormatPDF: "application/pdf",
}

func validate(req Request) error {
	if req.View == ViewSurface {
		return validateSurface(req)
	}
	switch req.View {
	case ViewC4, ViewFlow, ViewArchitecture, ViewSequence:
	default:
		return fmt.Errorf("%w: unknown view %q", ErrUnsupported, req.View)
	}
	switch req.Backend {
	case PlantUML, Mermaid:
	default:
		return fmt.Errorf("%w: unknown backend %q", ErrUnsupported, req.Backend)
	}
	switch req.Format {
	case FormatSource:
	case FormatMarkdown:
		if req.Backend != Mermaid {
			return fmt.Errorf("%w: markdown output is only available for mermaid", ErrUnsupported)
		}
	case FormatPNG, FormatSVG, FormatPDF:
		if req.Backend != PlantUML {
			return fmt.Errorf("%w: %s output is only available for plantuml", ErrUnsupported, req.Format)
		}
	default:
		return fmt.Errorf("%w: unknown format %q", ErrUnsupported, req.Format)
	}
	if req.Journey != "" {
		if req.View != ViewSequence {
			return fmt.Errorf("%w: a journey is drawn as a sequence; ask for the sequence view", ErrUnsupported)
		}
		if len(req.UseCases) > 0 {
			return fmt.Errorf("%w: `journey` draws a sequence over its own members; drop the use-case filter", ErrUnsupported)
		}
	}
	if req.View == ViewC4 {
		if req.C4 == nil {
			return fmt.Errorf("%w: a c4 diagram needs C4 options", ErrUnsupported)
		}
		if len(req.UseCases) > 0 {
			return fmt.Errorf("%w: a c4 diagram always shows every use case", ErrUnsupported)
		}
	} else if req.C4 != nil {
		return fmt.Errorf("%w: C4 options (focus, boundaries, databases) only apply to the c4 view", ErrUnsupported)
	}
	return nil
}

// validateSurface checks a surface request: a Markdown table, from either
// backend, over every use case.
func validateSurface(req Request) error {
	switch req.Backend {
	case PlantUML, Mermaid:
	default:
		return fmt.Errorf("%w: unknown backend %q", ErrUnsupported, req.Backend)
	}
	if req.Format != FormatMarkdown {
		return fmt.Errorf("%w: the surface view is a Markdown table; ask for markdown output", ErrUnsupported)
	}
	if len(req.UseCases) > 0 {
		return fmt.Errorf("%w: the surface view always covers every use case", ErrUnsupported)
	}
	if req.C4 != nil {
		return fmt.Errorf("%w: C4 options (focus, boundaries, databases) only apply to the c4 view", ErrUnsupported)
	}
	return nil
}

func write(doc *craft.CraftDoc, its []interaction.Interaction, req Request) string {
	switch req.View {
	case ViewC4:
		v := view.BuildC4(doc, its, view.C4Options{
			Boundaries:    req.C4.Boundaries,
			ShowDatabases: req.C4.ShowDatabases,
			FocusServices: req.C4.FocusServices,
			FocusContexts: req.C4.FocusContexts,
		})
		if req.Backend == Mermaid {
			return mermaid.C4(v)
		}
		return plantuml.C4(v)
	case ViewArchitecture:
		v := view.BuildArchitecture(doc, its)
		if req.Backend == Mermaid {
			return mermaid.Architecture(v)
		}
		return plantuml.Architecture(v)
	case ViewSequence:
		v := view.BuildSequenceWith(doc, its, view.SequenceOptions{SharedStage: req.Journey != ""})
		if req.Backend == Mermaid {
			return mermaid.Sequence(v)
		}
		return plantuml.Sequence(v)
	default:
		v := view.BuildFlow(doc, its)
		if req.Backend == Mermaid {
			return mermaid.Flow(v)
		}
		return plantuml.Flow(v)
	}
}

// withTitle heads source with req.Title: a PlantUML `title` line, or a
// Mermaid comment (Mermaid has no title directive for every diagram type).
// A journey is always a sequence, which takes Mermaid front matter.
func withTitle(src string, req Request) string {
	if req.Title == "" {
		return src
	}
	if req.Backend == Mermaid && req.Journey != "" {
		return "---\ntitle: \"" + strings.ReplaceAll(req.Title, `"`, `'`) + "\"\n---\n" + src
	}
	if req.Backend == Mermaid {
		return "%% " + req.Title + "\n" + src
	}
	const marker = "@startuml\n"
	if i := strings.Index(src, marker); i >= 0 {
		at := i + len(marker)
		return src[:at] + "title " + req.Title + "\n" + src[at:]
	}
	return src
}

func defaultTitle(doc *craft.CraftDoc, v View) string {
	if len(doc.UseCases) == 1 {
		return doc.UseCases[0].Name
	}
	switch v {
	case ViewC4:
		return "C4"
	case ViewSequence:
		return "Sequence"
	}
	return "Domain"
}

// spine keeps a journey's members, in spine order, and renumbers the
// interactions' UseCaseIndex to index the result. Interactions are derived
// over the whole model first, so a listener still pairs with publishers
// outside the spine.
func spine(doc *craft.CraftDoc, its []interaction.Interaction, j craft.Journey) (*craft.CraftDoc, []interaction.Interaction) {
	shown := *doc
	shown.UseCases = make([]craft.UseCase, len(j.Members))
	index := make(map[int]int, len(j.Members))
	for k, m := range j.Members {
		shown.UseCases[k] = doc.UseCases[m.UseCase]
		index[m.UseCase] = k
	}
	var out []interaction.Interaction
	for _, it := range its {
		if k, ok := index[it.UseCaseIndex]; ok {
			it.UseCaseIndex = k
			out = append(out, it)
		}
	}
	return &shown, out
}

// keepUseCases keeps the interactions of shown's use cases, a filtered copy
// of doc, and renumbers their UseCaseIndex to index shown.UseCases. The
// filter keeps doc's order and keeps or drops all use cases of one name
// together, so matching names in order pairs each kept use case exactly.
func keepUseCases(its []interaction.Interaction, doc, shown *craft.CraftDoc) []interaction.Interaction {
	index := map[int]int{} // doc index → shown index
	j := 0
	for i, uc := range doc.UseCases {
		if j < len(shown.UseCases) && shown.UseCases[j].Name == uc.Name {
			index[i] = j
			j++
		}
	}
	var out []interaction.Interaction
	for _, it := range its {
		if k, ok := index[it.UseCaseIndex]; ok {
			it.UseCaseIndex = k
			out = append(out, it)
		}
	}
	return out
}

// ExecPlantUML rasterizes PlantUML source by running the plantuml command.
type ExecPlantUML struct {
	// Command defaults to "plantuml"; Timeout defaults to one minute.
	Command string
	Timeout time.Duration
}

// Rasterize implements Rasterizer.
func (e ExecPlantUML) Rasterize(ctx context.Context, backend Backend, src []byte, format Format) ([]byte, error) {
	if backend != PlantUML {
		return nil, fmt.Errorf("%w: ExecPlantUML cannot rasterize %s", ErrUnsupported, backend)
	}
	command, timeout := e.Command, e.Timeout
	if command == "" {
		command = "plantuml"
	}
	if timeout == 0 {
		timeout = time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, "-pipe", "-t"+string(format))
	cmd.Stdin = bytes.NewReader(src)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plantuml error: %v, stderr: %s", err, stderr.String())
	}
	return out, nil
}
