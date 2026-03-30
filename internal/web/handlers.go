package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/storage"
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
	mux.HandleFunc("GET /api/artifacts", s.handleListArtifacts)
	mux.HandleFunc("GET /api/tools", s.handleListTools)
	mux.HandleFunc("GET /api/sessions/search", s.handleSearchSessions)
	mux.HandleFunc("POST /api/restart", s.handleRestart)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/audit", s.handleAudit)
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
		"workspace":     s.engine.Config().Workspace,
		"shield":        s.engine.ShieldStatus(),
	})
}

func (s *Server) handleListArtifacts(w http.ResponseWriter, _ *http.Request) {
	artifacts, err := s.engine.DB().ListArtifacts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if artifacts == nil {
		artifacts = []types.Artifact{}
	}
	writeJSON(w, http.StatusOK, artifacts)
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

func (s *Server) handleSearchSessions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}
	results, err := s.engine.DB().SearchSessions(query, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []storage.SearchSessionResult{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleListTools(w http.ResponseWriter, _ *http.Request) {
	tools := s.engine.ToolList()
	writeJSON(w, http.StatusOK, tools)
}

func (s *Server) handleRestart(w http.ResponseWriter, _ *http.Request) {
	s.log.Info("restart_requested", "source", "web_ui")
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})

	go func() {
		s.engine.Log().Info("engine_restart_initiated")
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	lines := 200
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}
	if lines > 1000 {
		lines = 1000
	}

	levelFilter := r.URL.Query().Get("level")
	eventFilter := r.URL.Query().Get("event")

	logPath := s.engine.LogPath()
	f, err := os.Open(logPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"entries":     []any{},
			"total_lines": 0,
			"has_more":    false,
		})
		return
	}
	defer func() { _ = f.Close() }()

	var allEntries []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if levelFilter != "" {
			if lvl, ok := entry["level"].(string); !ok || lvl != levelFilter {
				continue
			}
		}
		if eventFilter != "" {
			if evt, ok := entry["event"].(string); !ok || !strings.Contains(evt, eventFilter) {
				continue
			}
		}
		allEntries = append(allEntries, entry)
	}

	total := len(allEntries)
	start := 0
	if total > lines {
		start = total - lines
	}
	result := allEntries[start:]

	writeJSON(w, http.StatusOK, map[string]any{
		"entries":     result,
		"total_lines": total,
		"has_more":    start > 0,
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	lines := 100
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			lines = n
		}
	}

	auditPath := s.engine.AuditPath()
	f, err := os.Open(auditPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"entries":       []any{},
			"total_entries": 0,
			"chain_valid":   true,
			"has_more":      false,
		})
		return
	}
	defer func() { _ = f.Close() }()

	var allEntries []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		allEntries = append(allEntries, entry)
	}

	chainValid := true
	chainBreakAt := -1
	prevHash := ""
	for i, entry := range allEntries {
		ph, _ := entry["previous_hash"].(string)
		if i > 0 && ph != prevHash {
			chainValid = false
			chainBreakAt = i
			break
		}
		prevHash, _ = entry["hash"].(string)
	}

	total := len(allEntries)
	start := 0
	if total > lines {
		start = total - lines
	}
	result := allEntries[start:]

	resp := map[string]any{
		"entries":       result,
		"total_entries": total,
		"chain_valid":   chainValid,
		"has_more":      start > 0,
	}
	if !chainValid {
		resp["chain_break_at"] = chainBreakAt
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
