package analysis_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/model"
)

func workspaceOf(files map[string]string) analysis.Result {
	var results []analysis.FileResult
	for name, src := range files {
		results = append(results, analysis.File(name, src))
	}
	return analysis.Workspace(results)
}

func TestFile_StampsNameOnDiagnosticsAndUseCases(t *testing.T) {
	f := analysis.File("dir/a.craft", "use_case \"U\" {\n  when Bob does thing\n    Ghost thinks\n}\nservices {\n")
	if len(f.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for broken input")
	}
	for _, d := range f.Diagnostics {
		if d.SourceURI != "dir/a.craft" {
			t.Errorf("diagnostic %s has SourceURI %q, want the file name", d.Code, d.SourceURI)
		}
	}
	if got := f.Doc.UseCases[0].SourceURI; got != "dir/a.craft" {
		t.Errorf("use case SourceURI = %q, want the file name", got)
	}
}

func TestFile_MergesSameNameServiceBlocksWithinAFile(t *testing.T) {
	src := "services {\n  Svc {\n    contexts: A\n    language: go\n  }\n}\nservices {\n  Svc {\n    contexts: B, A\n    language: java\n    data-stores: db\n  }\n}\n"
	svcs := analysis.File("a.craft", src).Doc.Services
	if len(svcs) != 1 {
		t.Fatalf("want one merged service, got %d", len(svcs))
	}
	s := svcs[0]
	if strings.Join(s.Contexts, ",") != "A,B" || strings.Join(s.DataStores, ",") != "db" || s.Language != "go" {
		t.Errorf("merged service = %+v; want contexts A,B, data-stores db, first language go", s)
	}
}

// Across files the first declaration (in sorted file order) wins, as it does
// in sema, and the later one is reported.
func TestWorkspace_FirstDeclarationWinsAcrossFiles(t *testing.T) {
	res := workspaceOf(map[string]string{
		"b.craft": "domain Sales {\n  Billing\n}\nservices {\n  Svc {\n    contexts: Billing\n  }\n}\n",
		"a.craft": "domain Sales {\n  Ordering\n}\nservices {\n  Svc {\n    contexts: Ordering\n  }\n}\n",
	})
	if len(res.Doc.Services) != 1 || res.Doc.Services[0].Contexts[0] != "Ordering" {
		t.Errorf("services = %+v; want only a.craft's Svc", res.Doc.Services)
	}
	if len(res.Doc.Domains) != 1 || res.Doc.Domains[0].BoundedContexts[0] != "Ordering" {
		t.Errorf("domains = %+v; want only a.craft's Sales", res.Doc.Domains)
	}
	var dupDomain, dupService bool
	for _, d := range res.WorkspaceDiagnostics {
		if d.Code == "craft/sema/duplicate-name" && d.SourceURI == "b.craft" {
			dupDomain = dupDomain || strings.Contains(d.Message, "domain")
			dupService = dupService || strings.Contains(d.Message, "service")
		}
	}
	if !dupDomain || !dupService {
		t.Errorf("want duplicate-name warnings on b.craft for the domain and the service, got %+v", res.WorkspaceDiagnostics)
	}
}

func TestWorkspace_IsIndependentOfInputOrder(t *testing.T) {
	files := []analysis.FileResult{
		analysis.File("z.craft", "use_case \"Z\" {\n  when Bob does z\n    Nowhere thinks\n}\n"),
		analysis.File("a.craft", "use_case \"A\" {\n  when Bob does a\n    Ordering notifies \"x happened\"\n}\n"),
		analysis.File("m.craft", "domain D {\n  Ordering\n}\n"),
	}
	first := analysis.Workspace(files)
	reversed := analysis.Workspace([]analysis.FileResult{files[2], files[1], files[0]})
	for i := 0; i < 10; i++ {
		again := analysis.Workspace(files)
		if mustJSON(t, again) != mustJSON(t, first) {
			t.Fatal("repeated Workspace calls disagree")
		}
	}
	if mustJSON(t, reversed) != mustJSON(t, first) {
		t.Error("Workspace result depends on the order files are passed in")
	}
	if first.Doc.UseCases[0].Name != "A" {
		t.Errorf("use cases should follow sorted file order, got %q first", first.Doc.UseCases[0].Name)
	}
}

func TestWorkspace_DiagnosticsIncludePerFileAndWorkspaceSubset(t *testing.T) {
	res := workspaceOf(map[string]string{
		"a.craft": "services {\n  Svc {\n    contexts: Missing\n  }\n}\nuse_case \"U\" {\n",
	})
	codes := map[string]bool{}
	for _, d := range res.Diagnostics {
		codes[d.Code] = true
	}
	hasSyntax := false
	for c := range codes {
		hasSyntax = hasSyntax || strings.HasPrefix(c, "craft/syntax/")
	}
	if !hasSyntax || !codes["craft/sema/unresolved-reference"] {
		t.Errorf("Diagnostics should hold per-file (syntax) and workspace (resolution) findings, got %v", codes)
	}
	for _, d := range res.WorkspaceDiagnostics {
		if strings.HasPrefix(d.Code, "craft/syntax/") {
			t.Errorf("WorkspaceDiagnostics must not repeat per-file diagnostic %s", d.Code)
		}
	}
}

func TestWorkspace_Empty(t *testing.T) {
	res := analysis.Workspace(nil)
	if res.Doc == nil || res.Doc.UseCases == nil {
		t.Fatal("empty workspace must still return a document with a non-nil use case list")
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("empty workspace has diagnostics: %v", res.Diagnostics)
	}
}

func mustJSON(t *testing.T, r analysis.Result) string {
	t.Helper()
	b, err := json.Marshal(struct {
		Doc   *model.CraftDoc
		Diags []model.Diagnostic
	}{r.Doc, r.Diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The bug behind this work: `re/billing asks …` passed validation, but the
// model only carried the text "re/billing", which no service's contexts list
// matches, so diagrams dropped the edge. Every participant now carries its
// resolved identity and owning service.
func TestWorkspace_ResolvesParticipants(t *testing.T) {
	res := workspaceOf(map[string]string{
		"structure.craft": `actor user Customer

domain re {
    billing
    subscriptions
}

domain ops {
    billing
}

services {
    BillingService {
        contexts: subscriptions
        data-stores: billing_db
    }
}
`,
		"flows.craft": `use_case "Renew" {
    when Customer renews plan
        subscriptions asks re/billing for a charge
        re/billing asks billing_db to store the charge
        re/billing returns to subscriptions the receipt
        subscriptions notifies "Renewed"
    when subscriptions listens "Renewed"
        subscriptions updates the plan
}
`,
	})
	bc := func(id, svc string) *model.Participant {
		return &model.Participant{Kind: model.ParticipantBoundedContext, ID: id, Service: svc}
	}
	sc := res.Doc.UseCases[0].Scenarios
	acts := sc[0].Actions()
	checks := []struct {
		what      string
		got, want *model.Participant
	}{
		{"external trigger subject", sc[0].Trigger.Subject, &model.Participant{Kind: model.ParticipantActor, ID: "Customer"}},
		{"sync subject", acts[0].From, bc("re/subscriptions", "BillingService")},
		{"qualified sync target", acts[0].To, bc("re/billing", "")},
		{"data store target", acts[1].To, &model.Participant{Kind: model.ParticipantDataStore, ID: "billing_db", Service: "BillingService"}},
		{"return target", acts[2].To, bc("re/subscriptions", "BillingService")},
		{"async has no target", acts[3].To, nil},
		{"domain-listen subject", sc[1].Trigger.Subject, bc("re/subscriptions", "BillingService")},
	}
	for _, c := range checks {
		if (c.got == nil) != (c.want == nil) || (c.got != nil && *c.got != *c.want) {
			t.Errorf("%s = %+v, want %+v", c.what, c.got, c.want)
		}
	}
	if ids := res.Doc.Services[0].ContextIDs; len(ids) != 1 || ids[0] != "re/subscriptions" {
		t.Errorf("ContextIDs = %v, want [re/subscriptions]", ids)
	}
}

// An ambiguous bare name never resolves to a guessed owner: the participant
// stays nil and lint reports ambiguous-bc, an error that stops generate.
func TestWorkspace_AmbiguousParticipantStaysUnresolved(t *testing.T) {
	res := workspaceOf(map[string]string{
		"a.craft": "domain re {\n    billing\n}\ndomain ops {\n    billing\n}\nuse_case \"U\" {\n    when billing starts\n        billing charges card\n}\n",
	})
	a := res.Doc.UseCases[0].Scenarios[0].Actions()[0]
	if a.From != nil {
		t.Errorf("ambiguous subject resolved to %+v; want nil", a.From)
	}
	found := false
	for _, d := range res.Diagnostics {
		found = found || (d.Code == "craft/sema/ambiguous-bc" && d.Severity == model.SeverityError)
	}
	if !found {
		t.Error("want an ambiguous-bc error")
	}
}

func TestWorkspace_SurfacesFirstPerBoundaryWins(t *testing.T) {
	res := workspaceOf(map[string]string{
		"b.craft": "surface concierge {\n    query the logs  [getUserLogs]\n}\n",
		"a.craft": "domain obido {\n    concierge\n}\nsurface concierge {\n    callers: unknown\n    query the list  [getUserList]\n}\n",
		"c.craft": "surface ghost {\n    query x  [x]\n}\n",
	})
	ss := res.Doc.Surfaces
	if len(ss) != 2 {
		t.Fatalf("surfaces = %+v", ss)
	}
	if ss[0].SourceURI != "a.craft" || ss[0].Participant == nil || ss[0].Participant.ID != "obido/concierge" || ss[0].Operations[0].Operation.Text != "getUserList" {
		t.Errorf("first = %+v", ss[0])
	}
	if ss[1].Subject != "ghost" || ss[1].Participant != nil {
		t.Errorf("second = %+v", ss[1])
	}
}
