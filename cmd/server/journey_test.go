package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// journeyFiles is two files, each with one use case tagged journey j, joined
// by an event hand-off.
func journeyFiles(t *testing.T, extra map[string]any) *strings.Reader {
	t.Helper()
	body := map[string]any{
		"files": map[string]string{
			"a.craft": "actor user Bob\n\nuse_case \"A\" {\n    tags {\n        journey: j\n    }\n    when Bob does it\n        svc notifies x.Done\n}\n",
			"b.craft": "use_case \"B\" {\n    tags {\n        journey: j\n    }\n    when other listens x.Done\n        other thinks\n}\n",
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReader(string(b))
}

func serve(t *testing.T, h func(*Server) http.HandlerFunc, path string, body *strings.Reader) *httptest.ResponseRecorder {
	t.Helper()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	rr := httptest.NewRecorder()
	h(srv)(rr, httptest.NewRequest(http.MethodPost, path, body))
	return rr
}

func TestHandleJourneys_ListsSlugs(t *testing.T) {
	rr := serve(t, (*Server).handleJourneys, "/journeys", journeyFiles(t, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	var resp JourneysResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || len(resp.Slugs) != 1 || resp.Slugs[0] != "j" {
		t.Errorf("got %+v, want success with slugs [j]", resp)
	}
}

func TestHandleJourneys_NoJourneysIsEmptyList(t *testing.T) {
	rr := serve(t, (*Server).handleJourneys, "/journeys", strings.NewReader(`{"files": {"a.craft": "actor user Bob\n"}}`))
	if !strings.Contains(rr.Body.String(), `"slugs":[]`) {
		t.Errorf("want an empty slug list, got %s", rr.Body.String())
	}
}

func TestHandlePreviewMermaidJourney_ReturnsTitledSource(t *testing.T) {
	rr := serve(t, (*Server).handlePreviewMermaidJourney, "/preview/mermaid/journey", journeyFiles(t, map[string]any{"slug": "j"}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || !strings.HasPrefix(resp.Data, "---\ntitle: \"Journey: j\"") {
		t.Errorf("want titled Mermaid source, got %+v", resp)
	}
}

func TestHandlePreviewMermaidJourney_UnknownSlugIs400(t *testing.T) {
	rr := serve(t, (*Server).handlePreviewMermaidJourney, "/preview/mermaid/journey", journeyFiles(t, map[string]any{"slug": "nope"}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "nope") {
		t.Errorf("error should name the slug: %s", rr.Body.String())
	}
}

func TestHandlePreviewJourney_PlantUMLSource(t *testing.T) {
	rr := serve(t, (*Server).handlePreviewJourney, "/preview/journey", journeyFiles(t, map[string]any{"slug": "j", "format": "puml"}))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	src, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "title Journey: j") {
		t.Errorf("want a PlantUML journey title, got:\n%s", src)
	}
}

func TestHandlePreviewJourney_MissingSlugIs400(t *testing.T) {
	rr := serve(t, (*Server).handlePreviewJourney, "/preview/journey", journeyFiles(t, nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}
