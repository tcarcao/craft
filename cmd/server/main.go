// cmd/server/main.go
package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tcarcao/craft/v2/internal/visualizer"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

//go:embed templates
var content embed.FS

type Response struct {
	Success      bool
	Error        string
	Input        string
	C4           string
	Context      string
	Sequence     string
	WantC4       bool
	WantContext  bool
	WantSequence bool
}

type Server struct {
	tmpl     *template.Template
	renderer *visualizer.Renderer
	lastC4   []byte
}

func NewServer() (*Server, error) {
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		tmpl:     tmpl,
		renderer: visualizer.New(visualizer.ExecPlantUML{}),
	}, nil
}

func (s *Server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.tmpl.Execute(w, nil)
	}
}

func (s *Server) handleGenerate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		input := r.FormValue("dsl")
		generateC4 := r.FormValue("c4") == "on"
		generateContext := r.FormValue("context") == "on"
		generateSequence := r.FormValue("sequence") == "on"

		resp := Response{
			Success:      true,
			Input:        input,
			WantC4:       generateC4,
			WantContext:  generateContext,
			WantSequence: generateSequence,
		}

		doc, _ := parseDSL(input)

		if generateC4 {
			out, err := s.renderer.Render(doc, visualizer.Request{
				View: visualizer.ViewC4, Backend: visualizer.PlantUML, Format: visualizer.FormatPNG,
				C4: &visualizer.C4Options{Boundaries: true, ShowDatabases: true},
			})
			if err != nil {
				log.Printf("Error generating C4 diagram: %v", err)
			} else {
				s.lastC4 = out.Data
				resp.C4 = base64.StdEncoding.EncodeToString(out.Data)
			}
		}

		s.tmpl.Execute(w, resp)
	}
}

func (s *Server) handleViewDiagram() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		diagramType := vars["type"]

		var diagram []byte
		switch diagramType {
		case "c4":
			diagram = s.lastC4
		default:
			http.NotFound(w, r)
			return
		}

		if len(diagram) == 0 {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Write(diagram)
	}
}

func (s *Server) handleDownloadDiagram() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		diagramType := vars["type"]

		var diagram []byte
		var filename string

		switch diagramType {
		case "c4":
			diagram = s.lastC4
			filename = "c4-diagram.png"
		default:
			http.NotFound(w, r)
			return
		}

		if len(diagram) == 0 {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write(diagram)
	}
}

// Domain-specific preview request
type DomainPreviewRequest struct {
	DSL         string `json:"dsl"`
	DiagramType string `json:"diagramType,omitempty"` // domain, sequence
	DomainMode  string `json:"domainMode,omitempty"`  // detailed, architecture (only applies when diagramType is "domain")
	Format      string `json:"format,omitempty"`      // png, svg, pdf, puml (default: png)
}

// C4-specific preview request
type C4PreviewRequest struct {
	DSL            string     `json:"dsl"`
	FocusInfo      *FocusInfo `json:"focusInfo,omitempty"`
	BoundariesMode string     `json:"boundariesMode,omitempty"`
	ShowDatabases  *bool      `json:"showDatabases,omitempty"`
	Format         string     `json:"format,omitempty"` // png, svg, pdf, puml (default: png)
}

type FocusInfo struct {
	FocusedServiceNames []string `json:"focusedServiceNames"`
	FocusedContextNames []string `json:"focusedContextNames"`
	HasFocusedServices  bool     `json:"hasFocusedServices"`
	HasFocusedContexts  bool     `json:"hasFocusedContexts"`
}

type PreviewResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    string `json:"data,omitempty"` // base64 encoded diagram
	// Diagnostics lists every problem found in the DSL. A preview is always
	// drawn from whatever model exists, so errors here mean the diagram may be
	// incomplete rather than that the request failed.
	Diagnostics []craft.Diagnostic `json:"diagnostics,omitempty"`
}

// Domain-specific download request
type DomainDownloadRequest struct {
	DSL         string `json:"dsl"`
	DiagramType string `json:"diagramType,omitempty"` // domain, sequence
	DomainMode  string `json:"domainMode,omitempty"`  // detailed, architecture (only applies when diagramType is "domain")
	Format      string `json:"format"`                // png, svg, pdf, puml
	Filename    string `json:"filename,omitempty"`
}

// C4-specific download request
type C4DownloadRequest struct {
	DSL            string     `json:"dsl"`
	FocusInfo      *FocusInfo `json:"focusInfo,omitempty"`
	BoundariesMode string     `json:"boundariesMode,omitempty"`
	ShowDatabases  *bool      `json:"showDatabases,omitempty"`
	Format         string     `json:"format"` // png, svg, pdf, puml
	Filename       string     `json:"filename,omitempty"`
}

// Request mapping: the HTTP API keeps its field names; these helpers turn
// them into one visualizer.Request each.

// domainView maps diagramType (domain|sequence) and domainMode
// (detailed|architecture) to a view.
func domainView(diagramType, domainMode string) visualizer.View {
	switch {
	case diagramType == "sequence":
		return visualizer.ViewSequence
	case domainMode == "architecture":
		return visualizer.ViewArchitecture
	}
	return visualizer.ViewFlow
}

// plantUMLFormat maps the API's format field; PNG is the default.
func plantUMLFormat(format string) visualizer.Format {
	switch format {
	case "puml":
		return visualizer.FormatSource
	case "svg":
		return visualizer.FormatSVG
	case "pdf":
		return visualizer.FormatPDF
	}
	return visualizer.FormatPNG
}

// c4Options maps the C4 request fields. Boundaries mode is the default, and
// databases are shown unless showDatabases is explicitly false.
func c4Options(boundariesMode string, showDatabases *bool, focus *FocusInfo) *visualizer.C4Options {
	opts := &visualizer.C4Options{Boundaries: boundariesMode != "transparent", ShowDatabases: true}
	if showDatabases != nil {
		opts.ShowDatabases = *showDatabases
	}
	if focus != nil {
		if focus.HasFocusedServices {
			opts.FocusServices = focus.FocusedServiceNames
		}
		if focus.HasFocusedContexts {
			opts.FocusContexts = focus.FocusedContextNames
		}
	}
	return opts
}

// render draws one diagram and writes an error response when it cannot: 400
// for a request the visualizer does not support, 500 otherwise. It reports
// whether the caller should carry on.
func (s *Server) render(w http.ResponseWriter, doc *craft.CraftDoc, req visualizer.Request) (visualizer.Output, bool) {
	out, err := s.renderer.Render(doc, req)
	switch {
	case errors.Is(err, visualizer.ErrUnsupported):
		respondWithError(w, http.StatusBadRequest, err.Error())
		return out, false
	case err != nil:
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
		return out, false
	}
	return out, true
}

func respondJSON(w http.ResponseWriter, resp PreviewResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request format")
		return false
	}
	return true
}

// handlePreviewDomain renders a domain or sequence diagram with PlantUML.
// Response.Data is base64.
func (s *Server) handlePreviewDomain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DomainPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: domainView(req.DiagramType, req.DomainMode), Backend: visualizer.PlantUML, Format: plantUMLFormat(req.Format),
		})
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: base64.StdEncoding.EncodeToString(out.Data), Diagnostics: diags})
	}
}

// handlePreviewMermaidDomain returns Mermaid source text (not a rendered image)
// for the given Craft DSL. Response.Data carries plain text, NOT base64.
// Clients are expected to render the source themselves (e.g. via mermaid.js
// in a webview).
func (s *Server) handlePreviewMermaidDomain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DomainPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: domainView("domain", req.DomainMode), Backend: visualizer.Mermaid, Format: visualizer.FormatSource,
		})
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: string(out.Data), Diagnostics: diags})
	}
}

// handlePreviewMermaidSequence returns Mermaid sequenceDiagram source for the
// given Craft DSL. Response.Data is plain text, NOT base64.
func (s *Server) handlePreviewMermaidSequence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DomainPreviewRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: visualizer.ViewSequence, Backend: visualizer.Mermaid, Format: visualizer.FormatSource,
		})
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: string(out.Data), Diagnostics: diags})
	}
}

// handlePreviewMermaidC4 returns Mermaid C4Container source for the given
// Craft DSL. Response.Data is plain text, NOT base64. Focus, boundaries mode
// and database visibility apply exactly as on the PlantUML side.
func (s *Server) handlePreviewMermaidC4() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req C4PreviewRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: visualizer.ViewC4, Backend: visualizer.Mermaid, Format: visualizer.FormatSource,
			C4: c4Options(req.BoundariesMode, req.ShowDatabases, req.FocusInfo),
		})
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: string(out.Data), Diagnostics: diags})
	}
}

// handlePreviewC4 renders a C4 diagram to PNG. Response.Data is base64.
func (s *Server) handlePreviewC4() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req C4PreviewRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: visualizer.ViewC4, Backend: visualizer.PlantUML, Format: plantUMLFormat(req.Format),
			C4: c4Options(req.BoundariesMode, req.ShowDatabases, req.FocusInfo),
		})
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: base64.StdEncoding.EncodeToString(out.Data), Diagnostics: diags})
	}
}

// JourneyRequest names a journey over several files (a journey spans files,
// so the journey endpoints take files instead of one dsl string).
type JourneyRequest struct {
	Files  map[string]string `json:"files"`
	Slug   string            `json:"slug,omitempty"`
	Format string            `json:"format,omitempty"` // png, svg, pdf, puml (PlantUML preview only)
}

// JourneysResponse lists the journey slugs of the posted files.
type JourneysResponse struct {
	Success     bool               `json:"success"`
	Slugs       []string           `json:"slugs"`
	Diagnostics []craft.Diagnostic `json:"diagnostics,omitempty"`
}

// parseFiles analyses the posted files as one model.
func parseFiles(files map[string]string) (*craft.CraftDoc, []craft.Diagnostic) {
	in := make(map[string][]byte, len(files))
	for name, src := range files {
		in[name] = []byte(src)
	}
	doc, diags, _ := craft.ParseFiles(in)
	return doc, diags
}

// handleJourneys returns the journey slugs of the posted files, sorted.
func (s *Server) handleJourneys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req JourneyRequest
		if !decode(w, r, &req) {
			return
		}
		doc, diags := parseFiles(req.Files)
		slugs := craft.JourneySlugs(doc)
		if slugs == nil {
			slugs = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(JourneysResponse{Success: true, Slugs: slugs, Diagnostics: diags})
	}
}

// journeyPreview renders the spine of req.Slug over req.Files with backend
// and format. An empty or unknown slug answers 400.
func (s *Server) journeyPreview(w http.ResponseWriter, r *http.Request, backend visualizer.Backend, format func(string) visualizer.Format) (visualizer.Output, []craft.Diagnostic, bool) {
	var req JourneyRequest
	if !decode(w, r, &req) {
		return visualizer.Output{}, nil, false
	}
	if req.Slug == "" {
		respondWithError(w, http.StatusBadRequest, "slug is required: name the journey to draw (POST /journeys lists them)")
		return visualizer.Output{}, nil, false
	}
	doc, diags := parseFiles(req.Files)
	out, ok := s.render(w, doc, visualizer.Request{
		View: visualizer.ViewSequence, Backend: backend, Format: format(req.Format), Journey: req.Slug,
	})
	return out, diags, ok
}

// handlePreviewJourney renders a journey spine with PlantUML. Response.Data
// is base64.
func (s *Server) handlePreviewJourney() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, diags, ok := s.journeyPreview(w, r, visualizer.PlantUML, plantUMLFormat)
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: base64.StdEncoding.EncodeToString(out.Data), Diagnostics: diags})
	}
}

// handlePreviewMermaidJourney returns the Mermaid source of a journey spine.
// Response.Data is plain text, NOT base64.
func (s *Server) handlePreviewMermaidJourney() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, diags, ok := s.journeyPreview(w, r, visualizer.Mermaid, func(string) visualizer.Format { return visualizer.FormatSource })
		if !ok {
			return
		}
		respondJSON(w, PreviewResponse{Success: true, Data: string(out.Data), Diagnostics: diags})
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	log.Printf("[%d] %s", code, message)
	response := PreviewResponse{
		Success: false,
		Error:   message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

// parseDSL analyzes a Craft DSL string as a model of one file. Previews are
// tolerant: the caller always draws whatever (possibly partial) model comes
// back, and returns the diagnostics alongside it.
func parseDSL(src string) (*craft.CraftDoc, []craft.Diagnostic) {
	doc, diags, _ := craft.Parse("preview.craft", []byte(src))
	return doc, diags
}

// download writes a rendered diagram as an attachment, named filename or
// defaultName plus the format's extension.
func download(w http.ResponseWriter, out visualizer.Output, filename, defaultName string) {
	if filename == "" {
		filename = defaultName + "." + out.Ext
	}
	w.Header().Set("Content-Type", out.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(out.Data)
}

func (s *Server) handleDownloadDomainDiagram() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DomainDownloadRequest
		if !decode(w, r, &req) {
			return
		}
		// Downloads carry the raw file only; diagnostics go with previews.
		doc, _ := parseDSL(req.DSL)
		v := domainView(req.DiagramType, req.DomainMode)
		out, ok := s.render(w, doc, visualizer.Request{View: v, Backend: visualizer.PlantUML, Format: plantUMLFormat(req.Format)})
		if !ok {
			return
		}
		name := map[visualizer.View]string{
			visualizer.ViewSequence:     "sequence-diagram",
			visualizer.ViewArchitecture: "architecture-diagram",
		}[v]
		if name == "" {
			name = "domain-diagram"
		}
		download(w, out, req.Filename, name)
	}
}

func (s *Server) handleDownloadC4Diagram() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req C4DownloadRequest
		if !decode(w, r, &req) {
			return
		}
		doc, _ := parseDSL(req.DSL)
		out, ok := s.render(w, doc, visualizer.Request{
			View: visualizer.ViewC4, Backend: visualizer.PlantUML, Format: plantUMLFormat(req.Format),
			C4: c4Options(req.BoundariesMode, req.ShowDatabases, req.FocusInfo),
		})
		if !ok {
			return
		}
		download(w, out, req.Filename, "c4-diagram")
	}
}

func main() {
	server, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()

	// Route handlers
	r.HandleFunc("/", server.handleIndex()).Methods("GET")
	r.HandleFunc("/", server.handleGenerate()).Methods("POST")
	r.HandleFunc("/diagram/{type}", server.handleViewDiagram()).Methods("GET")
	r.HandleFunc("/download/{type}", server.handleDownloadDiagram()).Methods("GET")

	r.HandleFunc("/preview/domain", server.handlePreviewDomain()).Methods("POST")
	r.HandleFunc("/preview/mermaid/domain", server.handlePreviewMermaidDomain()).Methods("POST")
	r.HandleFunc("/preview/mermaid/sequence", server.handlePreviewMermaidSequence()).Methods("POST")
	r.HandleFunc("/preview/mermaid/c4", server.handlePreviewMermaidC4()).Methods("POST")
	r.HandleFunc("/preview/c4", server.handlePreviewC4()).Methods("POST")
	r.HandleFunc("/journeys", server.handleJourneys()).Methods("POST")
	r.HandleFunc("/preview/journey", server.handlePreviewJourney()).Methods("POST")
	r.HandleFunc("/preview/mermaid/journey", server.handlePreviewMermaidJourney()).Methods("POST")

	r.HandleFunc("/download/domain", server.handleDownloadDomainDiagram()).Methods("POST")
	r.HandleFunc("/download/c4", server.handleDownloadC4Diagram()).Methods("POST")

	// Add middleware for logging
	r.Use(loggingMiddleware)

	log.Printf("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", cors(r)))
}

// cors wraps the whole router rather than being a mux middleware: mux only
// runs middleware on a matched route, so a browser preflight (OPTIONS) on a
// POST-only route got a bare 405 and every browser caller failed.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Chrome asks before a public page (e.g. a Confluence macro) calls a
		// server on a private network; this answers yes.
		if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
