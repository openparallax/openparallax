package web

import (
	"encoding/json"
	"net/http"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

func (s *Server) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.handleUpdateSession)
	mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetMessages)
	mux.HandleFunc("GET /api/memory/search", s.handleMemorySearch)
	mux.HandleFunc("GET /api/memory/{type}", s.handleReadMemory)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	agentName := s.engine.Config().Identity.Name
	if agentName == "" {
		agentName = types.DefaultIdentity.Name
	}
	sessionCount, _ := s.engine.DB().SessionCount()

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_name":    agentName,
		"model":         s.engine.LLMModel(),
		"session_count": sessionCount,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	sessions, err := s.engine.DB().ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	mode := types.SessionNormal
	if body.Mode == "otr" {
		mode = types.SessionOTR
	}

	sess := &types.Session{
		ID:   crypto.NewID(),
		Mode: mode,
	}
	if err := s.engine.DB().InsertSession(sess); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.engine.DB().GetSession(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.DB().DeleteSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.engine.DB().UpdateSessionTitle(id, body.Title); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messages, err := s.engine.DB().GetMessages(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	limit := 10
	results, err := s.engine.Memory().Search(query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleReadMemory(w http.ResponseWriter, r *http.Request) {
	fileType := r.PathValue("type")
	content, err := s.engine.Memory().Read(types.MemoryFileType(fileType))
	if err != nil {
		writeError(w, http.StatusNotFound, "memory file not found: "+fileType)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"type":    fileType,
		"content": content,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
