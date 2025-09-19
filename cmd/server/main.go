// cmd/server/main.go
package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/internal/visualizer"
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
	tmpl   *template.Template
	viz    *visualizer.Visualizer
	lastC4 []byte
}

func NewServer() (*Server, error) {
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		return nil, err
	}

	return &Server{
		tmpl: tmpl,
		viz:  visualizer.New(),
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

		p := parser.NewParser()

		arch, err := p.ParseString(input)
		if err != nil {
			s.respondWithError(w, err, input, generateC4, generateContext, generateSequence)
			return
		}

		if generateC4 {
			diagram, err := s.viz.GenerateC4(arch, "boundaries", true)
			if err != nil {
				log.Printf("Error generating C4 diagram: %v", err)
			} else {
				s.lastC4 = diagram
				resp.C4 = base64.StdEncoding.EncodeToString(diagram)
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

func (s *Server) respondWithError(w http.ResponseWriter, err error, input string, c4, context, sequence bool) {
	s.tmpl.Execute(w, Response{
		Success:      false,
		Error:        err.Error(),
		Input:        input,
		WantC4:       c4,
		WantContext:  context,
		WantSequence: sequence,
	})
}

type PreviewRequest struct {
	DSL            string     `json:"dsl"`
	FocusInfo      *FocusInfo `json:"focusInfo,omitempty"`
	BoundariesMode string     `json:"boundariesMode,omitempty"`
	ShowDatabases  *bool      `json:"showDatabases,omitempty"`
	DomainMode     string     `json:"domainMode,omitempty"`     // detailed, architecture
}

type FocusInfo struct {
	FocusedServiceNames    []string `json:"focusedServiceNames"`
	FocusedSubDomainNames  []string `json:"focusedSubDomainNames"`
	HasFocusedServices     bool     `json:"hasFocusedServices"`
	HasFocusedSubDomains   bool     `json:"hasFocusedSubDomains"`
}

type PreviewResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    string `json:"data,omitempty"` // base64 encoded diagram
}

type DownloadRequest struct {
	DSL            string     `json:"dsl"`
	FocusInfo      *FocusInfo `json:"focusInfo,omitempty"`
	BoundariesMode string     `json:"boundariesMode,omitempty"`
	ShowDatabases  *bool      `json:"showDatabases,omitempty"`
	DomainMode     string     `json:"domainMode,omitempty"`     // detailed, architecture
	Format         string     `json:"format"`         // png, svg, pdf, puml
	DiagramType    string     `json:"diagramType"`    // c4, domain, context, sequence
	Filename       string     `json:"filename,omitempty"`
}

func (s *Server) handlePreviewDomain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		// Parse DSL
		p := parser.NewParser()

		model, err := p.ParseString(req.DSL)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
			return
		}

		// Parse domain mode, default to "detailed" if not provided or invalid
		domainMode := visualizer.DomainModeDetailed
		if req.DomainMode == string(visualizer.DomainModeArchitecture) {
			domainMode = visualizer.DomainModeArchitecture
		}

		// Generate Model diagram with mode
		diagram, err := s.viz.GenerateDomainDiagramWithMode(model, domainMode)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
			return
		}

		// Encode and respond
		response := PreviewResponse{
			Success: true,
			Data:    base64.StdEncoding.EncodeToString(diagram),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (s *Server) handlePreviewC4() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		// Parse DSL
		p := parser.NewParser()

		arch, err := p.ParseString(req.DSL)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
			return
		}

		fmt.Println(req.FocusInfo)

		// Parse boundaries mode, default to "boundaries" if not provided or invalid
		boundariesMode := visualizer.C4ModeBoundaries
		if req.BoundariesMode == string(visualizer.C4ModeTransparent) {
			boundariesMode = visualizer.C4ModeTransparent
		}

		// Parse database visibility, default to true if not provided
		showDatabases := true
		if req.ShowDatabases != nil {
			showDatabases = *req.ShowDatabases
		}

		// Generate C4 diagram with focus information, boundaries mode, and database visibility
		var diagram []byte
		if req.FocusInfo != nil && (req.FocusInfo.HasFocusedServices || req.FocusInfo.HasFocusedSubDomains) {
			diagram, err = s.viz.GenerateC4WithFocusAndSubDomains(arch, req.FocusInfo.FocusedServiceNames, req.FocusInfo.FocusedSubDomainNames, boundariesMode, showDatabases)
		} else {
			diagram, err = s.viz.GenerateC4(arch, boundariesMode, showDatabases)
		}

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
			return
		}

		// Encode and respond
		response := PreviewResponse{
			Success: true,
			Data:    base64.StdEncoding.EncodeToString(diagram),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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

func (s *Server) handleDownloadDiagramWithFormat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req DownloadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		// Parse DSL
		p := parser.NewParser()
		model, err := p.ParseString(req.DSL)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
			return
		}

		// Convert format string to SupportedFormat
		var format visualizer.SupportedFormat
		switch req.Format {
		case "png":
			format = visualizer.FormatPNG
		case "svg":
			format = visualizer.FormatSVG
		case "pdf":
			format = visualizer.FormatPDF
		case "puml":
			format = visualizer.FormatPUML
		default:
			format = visualizer.FormatPNG
		}

		var diagram []byte
		var contentType string
		var defaultFilename string

		// Generate diagram based on type
		switch req.DiagramType {
		case "c4":
			// Parse boundaries mode
			boundariesMode := visualizer.C4ModeBoundaries
			if req.BoundariesMode == string(visualizer.C4ModeTransparent) {
				boundariesMode = visualizer.C4ModeTransparent
			}

			// Parse database visibility, default to true if not provided
			showDatabases := true
			if req.ShowDatabases != nil {
				showDatabases = *req.ShowDatabases
			}

			// Generate C4 diagram with focus and format
			if req.FocusInfo != nil && (req.FocusInfo.HasFocusedServices || req.FocusInfo.HasFocusedSubDomains) {
				diagram, contentType, err = s.viz.GenerateC4WithFocusSubDomainsAndFormat(model, req.FocusInfo.FocusedServiceNames, req.FocusInfo.FocusedSubDomainNames, boundariesMode, showDatabases, format)
			} else {
				diagram, contentType, err = s.viz.GenerateC4WithFormat(model, boundariesMode, showDatabases, format)
			}
			defaultFilename = "c4-diagram"

		case "domain":
			// Parse domain mode, default to "detailed" if not provided or invalid
			domainMode := visualizer.DomainModeDetailed
			if req.DomainMode == string(visualizer.DomainModeArchitecture) {
				domainMode = visualizer.DomainModeArchitecture
			}
			
			diagram, contentType, err = s.viz.GenerateDomainDiagramWithModeAndFormat(model, domainMode, format)
			
			// Set filename based on mode
			if domainMode == visualizer.DomainModeArchitecture {
				defaultFilename = "architecture-diagram"
			} else {
				defaultFilename = "domain-diagram"
			}
		default:
			respondWithError(w, http.StatusBadRequest, "Invalid diagram type")
			return
		}

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
			return
		}

		// Determine filename
		filename := req.Filename
		if filename == "" {
			extension := string(format)
			if format == visualizer.FormatPUML {
				extension = "puml"
			}
			filename = fmt.Sprintf("%s.%s", defaultFilename, extension)
		}

		// Set response headers
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Write(diagram)
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
	r.HandleFunc("/preview/c4", server.handlePreviewC4()).Methods("POST")
	
	r.HandleFunc("/download", server.handleDownloadDiagramWithFormat()).Methods("POST")

	// CORS middleware for VSCode extension
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Add middleware for logging
	r.Use(loggingMiddleware)

	log.Printf("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
	})
}
