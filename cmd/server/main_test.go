package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDSL_V2Default(t *testing.T) {
	doc, err := parseDSL("actor user Foo", "v2")
	if err != nil {
		t.Fatalf("v2 parse failed: %v", err)
	}
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor in CraftDoc")
	}
}

func TestParseDSL_AntlrFallback(t *testing.T) {
	doc, err := parseDSL("actor user Foo", "antlr")
	if err != nil {
		t.Fatalf("antlr parse failed: %v", err)
	}
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor in CraftDoc")
	}
}

func TestParseDSL_EmptyParamDefaultsToV2(t *testing.T) {
	doc, err := parseDSL("actor user Foo", "")
	if err != nil {
		t.Fatalf("empty parserName (default) failed: %v", err)
	}
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor when parserName is empty (v2 default)")
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
