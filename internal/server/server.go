// Package server exposes the engine as a local JSON API and serves the
// embedded web UI.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/internal/engine"
)

// Server routes HTTP requests to the engine.
type Server struct {
	engine   *engine.Engine
	dataPath string
	ui       fs.FS
}

// New returns a Server. ui is the built web app; nil disables the UI.
func New(eng *engine.Engine, dataPath string, ui fs.FS) *Server {
	return &Server{engine: eng, dataPath: dataPath, ui: ui}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/season", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.engine.Season()) })
	mux.HandleFunc("GET /api/rules", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.engine.Rules()) })
	mux.HandleFunc("GET /api/project", s.handleProject)
	mux.HandleFunc("POST /api/optimize", s.handleOptimize)
	mux.HandleFunc("GET /api/prices", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.engine.Prices()) })
	mux.HandleFunc("GET /api/backtest", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.engine.Backtest(queryInt(r, "sims", 3000)))
	})
	mux.HandleFunc("GET /api/hindsight", s.handleHindsight)
	mux.HandleFunc("POST /api/review", s.handleReview)
	mux.HandleFunc("POST /api/sync", s.handleSync)
	if s.ui != nil {
		mux.Handle("/", spaHandler(s.ui))
	}
	return mux
}

// spaHandler serves the built app and falls back to index.html for client
// routes.
func spaHandler(ui fs.FS) http.Handler {
	server := http.FileServer(http.FS(ui))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(ui, path); err != nil {
			r.URL.Path = "/"
		}
		server.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func queryInt(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return v
	}
	return def
}

func queryList(r *http.Request, name string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	view, err := s.engine.Project(engine.ProjectInput{
		Round: queryInt(r, "round", 0),
		Sims:  queryInt(r, "sims", 0),
		Seed:  uint64(queryInt(r, "seed", 0)),
		Conditions: engine.Conditions{
			Quali: queryList(r, "quali"), Grid: queryList(r, "grid"),
			Back: queryList(r, "back"), FP3: queryList(r, "fp3"),
		},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, view)
}

func (s *Server) handleOptimize(w http.ResponseWriter, r *http.Request) {
	var in engine.OptimizeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad request body: %w", err))
		return
	}
	view, err := s.engine.Optimize(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, view)
}

func (s *Server) handleHindsight(w http.ResponseWriter, r *http.Request) {
	view, err := s.engine.Hindsight(queryInt(r, "round", 0), queryInt(r, "top", 5))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, view)
}

func (s *Server) handleReview(w http.ResponseWriter, r *http.Request) {
	var in engine.ReviewInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad request body: %w", err))
		return
	}
	view, err := s.engine.Review(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, view)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	d, err := dataset.Sync(s.engine.Data().Season)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if err := dataset.Save(d, s.dataPath); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.engine.SetData(d)
	writeJSON(w, s.engine.Season())
}
