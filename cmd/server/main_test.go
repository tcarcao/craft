package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestParseDSL_V2Default(t *testing.T) {
	doc, _ := parseDSL("actor user Foo")
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor in CraftDoc")
	}
}

func TestParseDSL_EmptyInput(t *testing.T) {
	doc, diags := parseDSL("")
	if doc == nil {
		t.Error("expected non-nil CraftDoc for empty input")
	}
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty input, got %v", diags)
	}
}

// A preview always draws, and reports problems in the optional diagnostics
// field instead of failing; the existing fields keep their meaning.
func TestHandlePreviewMermaidDomain_ReportsDiagnostics(t *testing.T) {
	body := strings.NewReader(`{"dsl": "services {\n  Svc { contexts: Missing }\n}\n"}`)
	req := httptest.NewRequest(http.MethodPost, "/preview/mermaid/domain", body)
	rr := httptest.NewRecorder()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server.handlePreviewMermaidDomain()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Data == "" {
		t.Fatalf("preview should still succeed with data: %+v", resp)
	}
	found := false
	for _, d := range resp.Diagnostics {
		if d.Severity == "error" && strings.Contains(d.Message, "Missing") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error diagnostic about the undeclared context, got %+v", resp.Diagnostics)
	}
}

func TestPreviewResponse_OmitsEmptyDiagnostics(t *testing.T) {
	b, _ := json.Marshal(PreviewResponse{Success: true, Data: "x"})
	if strings.Contains(string(b), "diagnostics") {
		t.Errorf("diagnostics must be omitted when empty, got %s", b)
	}
}

func TestHandlePreviewDomain_DefaultsToV2(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	body := `{"dsl":"actor user Foo","diagramType":"domain","format":"puml"}`
	req := httptest.NewRequest(http.MethodPost, "/preview/domain", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.handlePreviewDomain()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePreviewMermaidDomain_ReturnsMermaidSource(t *testing.T) {
	body := strings.NewReader(`{
		"dsl": "actor user Bob\n\nservices {\n  Svc { contexts: BC1 }\n}\n\nuse_case \"Alpha\" {\n  when Bob does thing\n    BC1 thinks something\n}\n",
		"domainMode": "detailed"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/preview/mermaid/domain", body)
	rr := httptest.NewRecorder()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server.handlePreviewMermaidDomain()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success=false: %s", resp.Error)
	}
	if !strings.HasPrefix(resp.Data, "flowchart LR\n") {
		t.Fatalf("expected Mermaid flowchart header in data, got: %s", resp.Data)
	}
}

func TestHandlePreviewMermaidSequence_ReturnsMermaidSource(t *testing.T) {
	body := strings.NewReader(`{
		"dsl": "actor user Bob\n\nservices {\n  Svc { contexts: BC1 }\n}\n\nuse_case \"Alpha\" {\n  when Bob does thing\n    BC1 thinks something\n}\n"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/preview/mermaid/sequence", body)
	rr := httptest.NewRecorder()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server.handlePreviewMermaidSequence()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success=false: %s", resp.Error)
	}
	if !strings.HasPrefix(resp.Data, "sequenceDiagram\n") {
		t.Fatalf("expected Mermaid sequenceDiagram header in data, got: %s", resp.Data)
	}
}

func TestHandlePreviewMermaidC4_ReturnsMermaidSource(t *testing.T) {
	body := strings.NewReader(`{
		"dsl": "actor user Bob\n\nservices {\n  Svc { contexts: BC1 }\n}\n\nuse_case \"Alpha\" {\n  when Bob does thing\n    BC1 thinks something\n}\n",
		"boundariesMode": "boundaries",
		"showDatabases": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/preview/mermaid/c4", body)
	rr := httptest.NewRecorder()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server.handlePreviewMermaidC4()(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success=false: %s", resp.Error)
	}
	if !strings.HasPrefix(resp.Data, "C4Container\n") {
		t.Fatalf("expected Mermaid C4Container header in data, got: %s", resp.Data)
	}
}

func TestCORS_AnswersPreflightOnPostOnlyRoute(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/preview/c4", func(http.ResponseWriter, *http.Request) {}).Methods("POST")

	req := httptest.NewRequest(http.MethodOptions, "/preview/c4", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rr := httptest.NewRecorder()
	cors(r).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Private-Network"); got != "true" {
		t.Errorf("Allow-Private-Network = %q, want true", got)
	}
}

func TestBasePath_Normalises(t *testing.T) {
	for in, want := range map[string]string{"": "", "/": "", "craft": "/craft", "/craft/": "/craft", "/a/b": "/a/b"} {
		if got := basePath(in); got != want {
			t.Errorf("basePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// Behind a path-based ingress that keeps the prefix, every route answers
// under it and nothing answers at the root.
func TestNewHandler_RoutesUnderBasePath(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := newHandler(server, "/craft")

	cases := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/craft/health", http.StatusOK},
		{http.MethodPost, "/craft/preview/mermaid/domain", http.StatusOK},
		{http.MethodGet, "/health", http.StatusNotFound},
		{http.MethodPost, "/preview/mermaid/domain", http.StatusNotFound},
		{http.MethodGet, "/craftx/health", http.StatusNotFound},
	}
	for _, c := range cases {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(c.method, c.path, strings.NewReader(`{"dsl": ""}`)))
		if rr.Code != c.want {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, rr.Code, c.want)
		}
	}
}

// The server is an API: the HTML form and the shared last-render routes are gone.
func TestNewHandler_NoWebUI(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	h := newHandler(server, "")

	for _, path := range []string{"/", "/diagram/c4", "/download/c4"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code == http.StatusOK {
			t.Errorf("GET %s = 200, want no route", path)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Errorf("GET /health = %d %q, want 200 ok", rr.Code, rr.Body.String())
	}
}
