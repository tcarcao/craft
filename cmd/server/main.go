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
	"github.com/tcarcao/archdsl/internal/parser"
	"github.com/tcarcao/archdsl/internal/visualizer"
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
	tmpl         *template.Template
	viz          *visualizer.Visualizer
	lastC4       []byte
	lastContext  []byte
	lastSequence []byte
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
			diagram, err := s.viz.GenerateC4(arch, "boundaries")
			if err != nil {
				log.Printf("Error generating C4 diagram: %v", err)
			} else {
				s.lastC4 = diagram
				resp.C4 = base64.StdEncoding.EncodeToString(diagram)
			}
		}

		if generateContext {
			diagram, err := s.viz.GenerateContextMap(arch)
			if err != nil {
				log.Printf("Error generating Context Map: %v", err)
			} else {
				s.lastContext = diagram
				resp.Context = base64.StdEncoding.EncodeToString(diagram)
			}
		}

		if generateSequence {
			diagram, err := s.viz.GenerateSequence(arch)
			if err != nil {
				log.Printf("Error generating Sequence diagram: %v", err)
			} else {
				s.lastSequence = diagram
				resp.Sequence = base64.StdEncoding.EncodeToString(diagram)
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
		case "context":
			diagram = s.lastContext
		case "sequence":
			diagram = s.lastSequence
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
		case "context":
			diagram = s.lastContext
			filename = "context-map.png"
		case "sequence":
			diagram = s.lastSequence
			filename = "sequence-diagram.png"
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

		// Generate Model diagram
		diagram, err := s.viz.GenerateDomainDiagram(model)
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

		// Generate C4 diagram with focus information and boundaries mode
		var diagram []byte
		if req.FocusInfo != nil && (req.FocusInfo.HasFocusedServices || req.FocusInfo.HasFocusedSubDomains) {
			diagram, err = s.viz.GenerateC4WithFocusAndSubDomains(arch, req.FocusInfo.FocusedServiceNames, req.FocusInfo.FocusedSubDomainNames, boundariesMode)
		} else {
			diagram, err = s.viz.GenerateC4(arch, boundariesMode)
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

func (s *Server) handlePreviewContext() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		p := parser.NewParser()

		arch, err := p.ParseString(req.DSL)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
			return
		}

		diagram, err := s.viz.GenerateContextMap(arch)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
			return
		}

		response := PreviewResponse{
			Success: true,
			Data:    base64.StdEncoding.EncodeToString(diagram),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (s *Server) handlePreviewSequence() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req PreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		p := parser.NewParser()

		arch, err := p.ParseString(req.DSL)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
			return
		}

		diagram, err := s.viz.GenerateSequence(arch)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Diagram generation failed: %v", err))
			return
		}

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
	r.HandleFunc("/preview/context", server.handlePreviewContext()).Methods("POST")
	r.HandleFunc("/preview/sequence", server.handlePreviewSequence()).Methods("POST")

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
