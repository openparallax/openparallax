package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/memory"
)

func (s *Server) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.handleUpdateSession)
	mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetMessages)
	mux.HandleFunc("GET /api/tools", s.handleListTools)
	mux.HandleFunc("GET /api/sessions/search", s.handleSearchSessions)
	mux.HandleFunc("POST /api/restart", s.handleRestart)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/audit", s.handleAudit)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	mux.HandleFunc("POST /api/settings/test-mcp", s.handleTestMCP)
	mux.HandleFunc("GET /api/memory/search", s.handleMemorySearch)
	mux.HandleFunc("GET /api/memory/{type}", s.handleReadMemory)
	mux.HandleFunc("GET /api/sub-agents", s.handleListSubAgents)
	mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/metrics/session/{id}", s.handleSessionMetrics)
	mux.HandleFunc("GET /api/metrics/daily", s.handleDailyTokens)
	mux.HandleFunc("GET /api/channels", s.handleListChannels)
	mux.HandleFunc("POST /api/channels/detach", s.handleDetachChannel)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	agentName := s.engine.Config().Identity.Name
	if agentName == "" {
		agentName = types.DefaultIdentity.Name
	}
	sessionCount, _ := s.engine.DB().SessionCount()

	s.log.Debug("api_status", "session_count", sessionCount)
	avatar := s.engine.Config().Identity.Avatar
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_name":    agentName,
		"agent_avatar":  avatar,
		"model":         s.engine.LLMModel(),
		"session_count": sessionCount,
		"workspace":     s.engine.Config().Workspace,
		"shield":        s.engine.ShieldStatus(),
		"sandbox":       s.engine.SandboxStatus(),
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	sessions, err := s.engine.DB().ListSessions()
	if err != nil {
		s.log.Error("api_list_sessions_failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Debug("api_list_sessions", "count", len(sessions))
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
	// OTR sessions never touch the database.
	if mode != types.SessionOTR {
		if err := s.engine.DB().InsertSession(sess); err != nil {
			s.log.Error("api_session_create_failed", "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if auditErr := s.engine.Audit().Log(audit.Entry{
			EventType: types.AuditSessionStarted,
			SessionID: sess.ID,
			Details:   "session created: " + string(sess.Mode),
		}); auditErr != nil {
			s.log.Warn("audit_write_failed", "event", "session_start", "session", sess.ID, "error", auditErr)
		}
	}

	s.log.Info("api_session_created", "session", sess.ID, "mode", sess.Mode)
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.engine.DB().GetSession(id)
	if err != nil {
		s.log.Debug("api_get_session_not_found", "session", id)
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	s.log.Debug("api_get_session", "session", id)
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.engine.DB().DeleteSession(id); err != nil {
		s.log.Error("api_session_delete_failed", "session", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("api_session_deleted", "session", id)
	if auditErr := s.engine.Audit().Log(audit.Entry{
		EventType: types.AuditSessionEnded,
		SessionID: id,
		Details:   "session deleted",
	}); auditErr != nil {
		s.log.Warn("audit_write_failed", "event", "session_end", "session", id, "error", auditErr)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.log.Warn("api_session_update_bad_request", "session", id, "error", err)
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := s.engine.DB().UpdateSessionTitle(id, body.Title); err != nil {
		s.log.Error("api_session_update_failed", "session", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("api_session_updated", "session", id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	messages, err := s.engine.DB().GetMessages(id)
	if err != nil {
		s.log.Error("api_get_messages_failed", "session", id, "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []types.Message{}
	}
	s.log.Debug("api_get_messages", "session", id, "count", len(messages))
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
		s.log.Error("api_memory_search_failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []memory.SearchResult{}
	}
	s.log.Debug("api_memory_search", "results", len(results))
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleReadMemory(w http.ResponseWriter, r *http.Request) {
	fileType := r.PathValue("type")
	content, err := s.engine.Memory().Read(memory.FileType(fileType))
	if err != nil {
		s.log.Debug("api_read_memory_not_found", "type", fileType)
		writeError(w, http.StatusNotFound, "memory file not found: "+fileType)
		return
	}
	s.log.Debug("api_read_memory", "type", fileType, "size", len(content))
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
		s.log.Error("api_search_sessions_failed", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []storage.SearchSessionResult{}
	}
	s.log.Debug("api_search_sessions", "results", len(results))
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleListTools(w http.ResponseWriter, _ *http.Request) {
	tools := s.engine.ToolList()
	if tools == nil {
		tools = []map[string]string{}
	}
	s.log.Debug("api_list_tools", "count", len(tools))
	writeJSON(w, http.StatusOK, tools)
}

func (s *Server) handleRestart(w http.ResponseWriter, _ *http.Request) {
	s.log.Info("restart_requested", "source", "web_ui")
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarting"})

	go func() {
		s.engine.Log().Info("engine_restart_initiated")
		os.Exit(75)
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

	s.log.Debug("api_logs", "lines", lines, "level", levelFilter, "event", eventFilter)
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
	malformed := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			malformed++
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
	if malformed > 0 {
		s.log.Warn("log_malformed_entries", "count", malformed)
	}

	total := len(allEntries)

	// Offset: number of entries to skip from the end (for lazy loading older entries).
	end := total
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < total {
			end = total - n
		}
	}

	start := 0
	if end > lines {
		start = end - lines
	}
	result := allEntries[start:end]

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

	s.log.Debug("api_audit", "lines", lines)
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
	auditMalformed := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			auditMalformed++
			continue
		}
		allEntries = append(allEntries, entry)
	}
	if auditMalformed > 0 {
		s.log.Warn("audit_malformed_entries", "count", auditMalformed)
	}

	chainValid := true
	chainBreakAt := -1
	prevHash := ""
	for i, entry := range allEntries {
		ph, _ := entry["previous_hash"].(string)
		if i > 0 && ph != prevHash {
			chainValid = false
			chainBreakAt = i
			s.log.Error("audit_chain_break_detected", "index", i,
				"expected_hash", prevHash, "actual_hash", ph)
			break
		}
		prevHash, _ = entry["hash"].(string)
	}

	total := len(allEntries)

	end := total
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < total {
			end = total - n
		}
	}

	start := 0
	if end > lines {
		start = end - lines
	}
	result := allEntries[start:end]

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

func (s *Server) handleListSubAgents(w http.ResponseWriter, _ *http.Request) {
	mgr := s.engine.SubAgentManager()
	if mgr == nil {
		s.log.Debug("api_list_sub_agents", "count", 0)
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	agents := mgr.List()
	s.log.Debug("api_list_sub_agents", "count", len(agents))
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	now := time.Now()
	var from, to string

	switch period {
	case "weekly":
		from = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "monthly":
		from = now.AddDate(0, -1, 0).Format("2006-01-02")
	case "yearly":
		from = now.AddDate(-1, 0, 0).Format("2006-01-02")
	default:
		from = now.Format("2006-01-02")
	}
	to = now.Format("2006-01-02")

	summary := s.engine.DB().GetMetricsSummary(from, to)
	s.log.Debug("api_metrics", "period", period, "from", from, "to", to)
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleSessionMetrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	usage := s.engine.DB().GetSessionTokenUsage(id)
	s.log.Debug("api_session_metrics", "session", id)
	writeJSON(w, http.StatusOK, usage)
}

func (s *Server) handleDailyTokens(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	to := time.Now().Format("2006-01-02")
	data := s.engine.DB().GetDailyTokens(from, to)
	if data == nil {
		data = []map[string]any{}
	}
	s.log.Debug("api_daily_tokens", "days", days, "entries", len(data))
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleListChannels(w http.ResponseWriter, _ *http.Request) {
	ctrl := s.engine.ChannelCtrl()
	if ctrl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"channels": []string{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": ctrl.AdapterNames()})
}

func (s *Server) handleDetachChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Channel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel name required"})
		return
	}

	ctrl := s.engine.ChannelCtrl()
	if ctrl == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "channel controller not available"})
		return
	}

	if err := ctrl.Detach(req.Channel); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	s.log.Info("channel_detached", "channel", req.Channel, "source", "api")
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached", "channel": req.Channel})
}
