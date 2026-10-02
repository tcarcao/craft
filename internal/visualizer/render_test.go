package visualizer

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRasterizer struct {
	gotBackend Backend
	gotFormat  Format
	gotSrc     string
}

func (f *fakeRasterizer) Rasterize(_ context.Context, b Backend, src []byte, format Format) ([]byte, error) {
	f.gotBackend, f.gotFormat, f.gotSrc = b, format, string(src)
	return []byte("IMAGE"), nil
}

const renderModel = `actor user Customer

domain shop {
    Ordering
    Billing
    Shipping
}

services {
    ShopService {
        contexts: Ordering, Billing, Shipping
    }
}

use_case "Checkout" {
    when Customer places order
        Ordering asks Billing to charge the card
        Ordering notifies "Order Placed"
}

use_case "Ship" {
    when Shipping listens "Order Placed"
        Shipping books a courier
}
`

func TestRender_RejectsWhatItCannotDo(t *testing.T) {
	doc := resolvedDoc(t, renderModel)
	c4 := &C4Options{Boundaries: true}
	cases := map[string]Request{
		"unknown view":             {View: "pie", Backend: PlantUML, Format: FormatSource},
		"mermaid to png":           {View: ViewFlow, Backend: Mermaid, Format: FormatPNG},
		"plantuml markdown":        {View: ViewFlow, Backend: PlantUML, Format: FormatMarkdown},
		"c4 without options":       {View: ViewC4, Backend: PlantUML, Format: FormatSource},
		"c4 options on a sequence": {View: ViewSequence, Backend: PlantUML, Format: FormatSource, C4: c4},
		"use-case filter on c4":    {View: ViewC4, Backend: Mermaid, Format: FormatSource, C4: c4, UseCases: []string{"checkout"}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(&fakeRasterizer{}).Render(doc, req)
			if !errors.Is(err, ErrUnsupported) {
				t.Errorf("err = %v, want ErrUnsupported", err)
			}
		})
	}
}

func TestRender_RasterizesThroughTheSeam(t *testing.T) {
	fake := &fakeRasterizer{}
	out, err := New(fake).Render(resolvedDoc(t, renderModel), Request{
		View: ViewC4, Backend: PlantUML, Format: FormatSVG, C4: &C4Options{}, Title: "Shop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Data) != "IMAGE" || out.ContentType != "image/svg+xml" || out.Ext != "svg" {
		t.Errorf("output = %q %q %q", out.Data, out.ContentType, out.Ext)
	}
	if fake.gotFormat != FormatSVG || !strings.HasPrefix(fake.gotSrc, "@startuml\ntitle Shop\n") {
		t.Errorf("rasterizer got format %q and source:\n%s", fake.gotFormat, fake.gotSrc)
	}
}

func TestRender_NoRasterizerIsAnError(t *testing.T) {
	_, err := New(nil).Render(resolvedDoc(t, renderModel), Request{View: ViewFlow, Backend: PlantUML, Format: FormatPNG})
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v, want ErrUnsupported", err)
	}
}

// Filtering keeps only the named use cases, but events still pair across the
// whole model: the "Ship" listener is fed from Ordering's queue even though
// "Checkout" is filtered out.
func TestRender_FilterKeepsCrossUseCasePairing(t *testing.T) {
	out, err := New(nil).Render(resolvedDoc(t, renderModel), Request{
		View: ViewFlow, Backend: PlantUML, Format: FormatSource, UseCases: []string{"ship"},
	})
	if err != nil {
		t.Fatal(err)
	}
	src := string(out.Data)
	if !strings.Contains(src, "Ordering_events --> Shipping : 1. Order Placed") {
		t.Errorf("listener should be fed by the publisher outside the filter:\n%s", src)
	}
	if strings.Contains(src, "charge the card") {
		t.Errorf("filtered-out use case leaked into the diagram:\n%s", src)
	}
}

func TestRender_UnknownUseCaseListsTheAvailableOnes(t *testing.T) {
	_, err := New(nil).Render(resolvedDoc(t, renderModel), Request{
		View: ViewSequence, Backend: Mermaid, Format: FormatSource, UseCases: []string{"refund"},
	})
	if err == nil || !strings.Contains(err.Error(), "refund") || !strings.Contains(err.Error(), "checkout, ship") {
		t.Errorf("err = %v; want it to name the missing use case and list checkout, ship", err)
	}
}

func TestRender_MarkdownAndTitles(t *testing.T) {
	doc := resolvedDoc(t, renderModel)
	md, err := New(nil).Render(doc, Request{View: ViewSequence, Backend: Mermaid, Format: FormatMarkdown, UseCases: []string{"Checkout"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(md.Data), "# Checkout\n\n```mermaid\nsequenceDiagram\n") || md.Ext != "md" {
		t.Errorf("markdown = %q (ext %q)", md.Data, md.Ext)
	}
	src, err := New(nil).Render(doc, Request{View: ViewFlow, Backend: Mermaid, Format: FormatSource, Title: "Flows"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(src.Data), "%% Flows\nflowchart LR\n") || src.Ext != "mmd" {
		t.Errorf("mermaid source = %q (ext %q)", src.Data, src.Ext)
	}
}
