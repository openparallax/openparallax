package storage

import "time"

// LLMUsageEntry records token usage for a single LLM call.
type LLMUsageEntry struct {
	SessionID           string
	MessageID           string
	Provider            string
	Model               string
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	ToolDefTokens       int
	Rounds              int
	DurationMs          int64
}

// InsertLLMUsage records a single LLM call's token usage.
func (db *DB) InsertLLMUsage(entry LLMUsageEntry) error {
	_, err := db.conn.Exec(`INSERT INTO llm_usage
		(session_id, message_id, provider, model, input_tokens, output_tokens,
		 cache_read_tokens, cache_creation_tokens, tool_def_tokens, rounds, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.SessionID, entry.MessageID, entry.Provider, entry.Model,
		entry.InputTokens, entry.OutputTokens,
		entry.CacheReadTokens, entry.CacheCreationTokens,
		entry.ToolDefTokens, entry.Rounds, entry.DurationMs,
	)
	return err
}

// IncrementDailyMetric adds delta to a named metric for today.
func (db *DB) IncrementDailyMetric(metric string, delta int) {
	date := time.Now().Format("2006-01-02")
	_, _ = db.conn.Exec(`INSERT INTO metrics_daily (date, metric, value) VALUES (?, ?, ?)
		ON CONFLICT(date, metric) DO UPDATE SET value = value + ?`,
		date, metric, delta, delta,
	)
}

// SessionTokenUsage returns aggregated token usage for a specific session.
type SessionTokenUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
	ToolDefTokens       int `json:"tool_def_tokens"`
	TotalRounds         int `json:"total_rounds"`
	LLMCalls            int `json:"llm_calls"`
	TotalDurationMs     int `json:"total_duration_ms"`
}

// GetSessionTokenUsage returns aggregated token usage for a session.
func (db *DB) GetSessionTokenUsage(sessionID string) SessionTokenUsage {
	var u SessionTokenUsage
	_ = db.conn.QueryRow(`SELECT
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cache_creation_tokens), 0),
		COALESCE(SUM(tool_def_tokens), 0),
		COALESCE(SUM(rounds), 0),
		COUNT(*),
		COALESCE(SUM(duration_ms), 0)
		FROM llm_usage WHERE session_id = ?`, sessionID).Scan(
		&u.InputTokens, &u.OutputTokens,
		&u.CacheReadTokens, &u.CacheCreationTokens,
		&u.ToolDefTokens, &u.TotalRounds,
		&u.LLMCalls, &u.TotalDurationMs,
	)
	return u
}

// MetricsSummary holds aggregated metrics for a time range.
type MetricsSummary struct {
	Period        string         `json:"period"`
	TokenUsage    map[string]int `json:"token_usage"`
	DailyMetrics  map[string]int `json:"daily_metrics"`
	TopTools      []ToolMetric   `json:"top_tools,omitempty"`
	ShieldSummary map[string]int `json:"shield_summary"`
	SessionCount  int            `json:"session_count"`
	MessageCount  int            `json:"message_count"`
}

// ToolMetric holds usage stats for a single tool.
type ToolMetric struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	AvgMs    int     `json:"avg_ms"`
	FailRate float64 `json:"fail_rate"`
}

// GetMetricsSummary returns aggregated metrics for a date range.
func (db *DB) GetMetricsSummary(from, to string) MetricsSummary {
	summary := MetricsSummary{
		Period:        from + " to " + to,
		TokenUsage:    make(map[string]int),
		DailyMetrics:  make(map[string]int),
		ShieldSummary: make(map[string]int),
	}

	// Token totals.
	var input, output, cacheRead, cacheCreation, toolDef, llmCalls int
	_ = db.conn.QueryRow(`SELECT
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cache_creation_tokens), 0),
		COALESCE(SUM(tool_def_tokens), 0),
		COUNT(*)
		FROM llm_usage WHERE timestamp >= ? AND timestamp <= ?`, from, to+" 23:59:59").Scan(
		&input, &output, &cacheRead, &cacheCreation, &toolDef, &llmCalls,
	)
	summary.TokenUsage["input"] = input
	summary.TokenUsage["output"] = output
	summary.TokenUsage["cache_read"] = cacheRead
	summary.TokenUsage["cache_creation"] = cacheCreation
	summary.TokenUsage["tool_def"] = toolDef
	summary.TokenUsage["llm_calls"] = llmCalls
	summary.TokenUsage["total"] = input + output

	// Daily metrics.
	rows, err := db.conn.Query(`SELECT metric, SUM(value) FROM metrics_daily
		WHERE date >= ? AND date <= ? GROUP BY metric`, from, to)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var metric string
			var value int
			if rows.Scan(&metric, &value) == nil {
				summary.DailyMetrics[metric] = value
			}
		}
	}

	// Shield summary from daily metrics.
	for _, k := range []string{"shield_allow", "shield_block", "shield_escalate",
		"shield_t0", "shield_t1", "shield_t2", "rate_limit_hit", "budget_exhausted"} {
		if v, ok := summary.DailyMetrics[k]; ok {
			summary.ShieldSummary[k] = v
		}
	}

	// Session and message counts for the period.
	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM sessions
		WHERE created_at >= ? AND created_at <= ?`, from, to+" 23:59:59").Scan(&summary.SessionCount)
	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM messages
		WHERE timestamp >= ? AND timestamp <= ?`, from, to+" 23:59:59").Scan(&summary.MessageCount)

	return summary
}

// GetDailyTokens returns per-day token totals for charting.
func (db *DB) GetDailyTokens(from, to string) []map[string]any {
	rows, err := db.conn.Query(`SELECT
		date(timestamp) as day,
		SUM(input_tokens) as input,
		SUM(output_tokens) as output,
		COUNT(*) as calls
		FROM llm_usage
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY day ORDER BY day`, from, to+" 23:59:59")
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var result []map[string]any
	for rows.Next() {
		var day string
		var input, output, calls int
		if rows.Scan(&day, &input, &output, &calls) == nil {
			result = append(result, map[string]any{
				"date": day, "input_tokens": input,
				"output_tokens": output, "calls": calls,
				"total": input + output,
			})
		}
	}
	return result
}
