package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const boardDSL = `services {
  Svc { contexts: Orders, Billing }
}

use_case "Place Order" {
  when Business_User creates Order
    Orders asks Billing to charge card
}

use_case "Cancel Order" {
  when Business_User cancels Order
    Orders notifies "Order Cancelled"
}
`

func postBoard(t *testing.T, body map[string]string) (*httptest.ResponseRecorder, PreviewResponse) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/preview/board/domain", strings.NewReader(string(b)))
	rr := httptest.NewRecorder()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	server.handlePreviewBoardDomain()(rr, req)
	var resp PreviewResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	return rr, resp
}

func TestPreviewBoard_FirstUseCaseByDefault(t *testing.T) {
	rr, resp := postBoard(t, map[string]string{"dsl": boardDSL})
	if rr.Code != http.StatusOK || !strings.HasPrefix(resp.Data, "<svg") || !strings.Contains(resp.Data, "Place Order") {
		t.Fatalf("code=%d data=%.80s", rr.Code, resp.Data)
	}
}

func TestPreviewBoard_PicksUseCaseBySlug(t *testing.T) {
	rr, resp := postBoard(t, map[string]string{"dsl": boardDSL, "useCase": "cancel-order"})
	if rr.Code != http.StatusOK || !strings.Contains(resp.Data, "Cancel Order") {
		t.Fatalf("code=%d data=%.80s", rr.Code, resp.Data)
	}
}

func TestPreviewBoard_Rejects(t *testing.T) {
	cases := map[string]map[string]string{
		"unknown use case": {"dsl": boardDSL, "useCase": "nope"},
		"diagram type":     {"dsl": boardDSL, "diagramType": "sequence"},
		"format":           {"dsl": boardDSL, "format": "png"},
		"architecture":     {"dsl": boardDSL, "domainMode": "architecture"},
		"no use cases":     {"dsl": "actor user Foo"},
	}
	for name, body := range cases {
		if rr, _ := postBoard(t, body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rr.Code)
		}
	}
}

func TestPreviewBoard_ReportsDiagnostics(t *testing.T) {
	dsl := "services {\n  Svc { contexts: Missing }\n}\n" + boardDSL
	rr, resp := postBoard(t, map[string]string{"dsl": dsl})
	if rr.Code != http.StatusOK || len(resp.Diagnostics) == 0 {
		t.Fatalf("code=%d diagnostics=%v", rr.Code, resp.Diagnostics)
	}
}
