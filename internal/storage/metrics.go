package storage

import (
	"fmt"
	"time"
)

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
	Period        string             `json:"period"`
	TokenUsage    map[string]int     `json:"token_usage"`
	DailyMetrics  map[string]int     `json:"daily_metrics"`
	TopTools      []ToolMetric       `json:"top_tools,omitempty"`
	ShieldSummary map[string]int     `json:"shield_summary"`
	SessionCount  int                `json:"session_count"`
	MessageCount  int                `json:"message_count"`
	Performance   PerformanceMetrics `json:"performance"`
	Lifetime      LifetimeMetrics    `json:"lifetime"`
}

// PerformanceMetrics holds latency and efficiency stats for a time range.
type PerformanceMetrics struct {
	AvgLatencyMs    int     `json:"avg_latency_ms"`
	P50LatencyMs    int     `json:"p50_latency_ms"`
	P95LatencyMs    int     `json:"p95_latency_ms"`
	P99LatencyMs    int     `json:"p99_latency_ms"`
	AvgRoundsPerMsg float64 `json:"avg_rounds_per_msg"`
	AvgTokensPerMsg int     `json:"avg_tokens_per_msg"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

// LifetimeMetrics holds all-time counters independent of date range.
type LifetimeMetrics struct {
	TotalSessions     int     `json:"total_sessions"`
	TotalMessages     int     `json:"total_messages"`
	TotalTokens       int     `json:"total_tokens"`
	AvgMsgsPerSession float64 `json:"avg_msgs_per_session"`
}

// ToolMetric holds usage stats for a single tool.
type ToolMetric struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
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
	// Use substr to extract the date portion — llm_usage.timestamp is SQLite
	// datetime('now') (UTC) while from/to are local dates.
	var input, output, cacheRead, cacheCreation, toolDef, llmCalls int
	_ = db.conn.QueryRow(`SELECT
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cache_creation_tokens), 0),
		COALESCE(SUM(tool_def_tokens), 0),
		COUNT(*)
		FROM llm_usage WHERE substr(timestamp, 1, 10) >= ? AND substr(timestamp, 1, 10) <= ?`, from, to).Scan(
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
	// Timestamps may be RFC3339 (with T and timezone) — use substr to compare date portion.
	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM sessions
		WHERE mode = 'normal' AND substr(created_at, 1, 10) >= ? AND substr(created_at, 1, 10) <= ?`, from, to).Scan(&summary.SessionCount)
	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM messages m
		JOIN sessions s ON s.id = m.session_id
		WHERE s.mode = 'normal' AND substr(m.timestamp, 1, 10) >= ? AND substr(m.timestamp, 1, 10) <= ?`, from, to).Scan(&summary.MessageCount)

	// Performance metrics from llm_usage.
	var avgLatency, avgTokens int
	var avgRounds float64
	_ = db.conn.QueryRow(`SELECT
		COALESCE(AVG(duration_ms), 0),
		COALESCE(AVG(CAST(rounds AS REAL)), 0),
		CASE WHEN COUNT(*) > 0 THEN COALESCE(SUM(input_tokens + output_tokens) / COUNT(*), 0) ELSE 0 END
		FROM llm_usage WHERE substr(timestamp, 1, 10) >= ? AND substr(timestamp, 1, 10) <= ?`, from, to).Scan(
		&avgLatency, &avgRounds, &avgTokens,
	)
	summary.Performance.AvgLatencyMs = avgLatency
	summary.Performance.AvgRoundsPerMsg = avgRounds
	summary.Performance.AvgTokensPerMsg = avgTokens

	// Cache hit rate.
	var totalInput, cacheHits int
	_ = db.conn.QueryRow(`SELECT
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0)
		FROM llm_usage WHERE substr(timestamp, 1, 10) >= ? AND substr(timestamp, 1, 10) <= ?`, from, to).Scan(
		&totalInput, &cacheHits,
	)
	if totalInput > 0 {
		summary.Performance.CacheHitRate = float64(cacheHits) / float64(totalInput)
	}

	// Latency percentiles (p50, p95, p99).
	summary.Performance.P50LatencyMs, summary.Performance.P95LatencyMs,
		summary.Performance.P99LatencyMs = db.getLatencyPercentiles(from, to)

	// Top tools from daily metrics (tool:name counters).
	toolRows, toolErr := db.conn.Query(`SELECT metric, SUM(value) FROM metrics_daily
		WHERE date >= ? AND date <= ? AND metric LIKE 'tool:%'
		GROUP BY metric ORDER BY SUM(value) DESC LIMIT 8`, from, to)
	if toolErr == nil {
		defer func() { _ = toolRows.Close() }()
		for toolRows.Next() {
			var metric string
			var count int
			if toolRows.Scan(&metric, &count) == nil {
				// Strip "tool:" prefix.
				name := metric[5:]
				summary.TopTools = append(summary.TopTools, ToolMetric{Name: name, Count: count})
			}
		}
	}

	// Lifetime metrics (no date filter).
	summary.Lifetime = db.getLifetimeMetrics()

	return summary
}

// getLatencyPercentiles fetches all durations and computes p50/p95/p99.
func (db *DB) getLatencyPercentiles(from, to string) (p50, p95, p99 int) {
	rows, err := db.conn.Query(`SELECT duration_ms FROM llm_usage
		WHERE substr(timestamp, 1, 10) >= ? AND substr(timestamp, 1, 10) <= ?
		ORDER BY duration_ms`, from, to)
	if err != nil {
		return 0, 0, 0
	}
	defer func() { _ = rows.Close() }()

	var durations []int
	for rows.Next() {
		var d int
		if rows.Scan(&d) == nil {
			durations = append(durations, d)
		}
	}

	n := len(durations)
	if n == 0 {
		return 0, 0, 0
	}

	percentile := func(pct int) int {
		idx := (pct * n) / 100
		if idx >= n {
			idx = n - 1
		}
		return durations[idx]
	}

	return percentile(50), percentile(95), percentile(99)
}

// getLifetimeMetrics returns all-time counters independent of date range.
func (db *DB) getLifetimeMetrics() LifetimeMetrics {
	var m LifetimeMetrics

	// Total sessions ever created (from daily counter, survives deletions).
	_ = db.conn.QueryRow(`SELECT COALESCE(SUM(value), 0) FROM metrics_daily
		WHERE metric = 'sessions_created'`).Scan(&m.TotalSessions)
	// Fall back to current session count if counter not yet populated.
	if m.TotalSessions == 0 {
		_ = db.conn.QueryRow(`SELECT COUNT(*) FROM sessions WHERE mode = 'normal'`).Scan(&m.TotalSessions)
	}

	_ = db.conn.QueryRow(`SELECT COUNT(*) FROM messages m
		JOIN sessions s ON s.id = m.session_id
		WHERE s.mode = 'normal'`).Scan(&m.TotalMessages)

	_ = db.conn.QueryRow(`SELECT COALESCE(SUM(input_tokens + output_tokens), 0)
		FROM llm_usage`).Scan(&m.TotalTokens)

	if m.TotalSessions > 0 {
		m.AvgMsgsPerSession = float64(m.TotalMessages) / float64(m.TotalSessions)
	}

	return m
}

// PruneLLMUsage aggregates rows older than cutoffDays into metrics_daily
// summaries and deletes the raw rows. Returns the number of rows pruned.
func (db *DB) PruneLLMUsage(cutoffDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -cutoffDays).Format("2006-01-02")

	tx, err := db.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Aggregate input tokens by date.
	_, err = tx.Exec(`INSERT INTO metrics_daily (date, metric, value)
		SELECT substr(timestamp, 1, 10), 'tokens_input_archived', SUM(input_tokens)
		FROM llm_usage WHERE timestamp < ?
		GROUP BY substr(timestamp, 1, 10)
		ON CONFLICT(date, metric) DO UPDATE SET value = value + excluded.value`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("aggregate input tokens: %w", err)
	}

	// Aggregate output tokens by date.
	_, err = tx.Exec(`INSERT INTO metrics_daily (date, metric, value)
		SELECT substr(timestamp, 1, 10), 'tokens_output_archived', SUM(output_tokens)
		FROM llm_usage WHERE timestamp < ?
		GROUP BY substr(timestamp, 1, 10)
		ON CONFLICT(date, metric) DO UPDATE SET value = value + excluded.value`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("aggregate output tokens: %w", err)
	}

	// Aggregate LLM call count by date.
	_, err = tx.Exec(`INSERT INTO metrics_daily (date, metric, value)
		SELECT substr(timestamp, 1, 10), 'llm_calls_archived', COUNT(*)
		FROM llm_usage WHERE timestamp < ?
		GROUP BY substr(timestamp, 1, 10)
		ON CONFLICT(date, metric) DO UPDATE SET value = value + excluded.value`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("aggregate call count: %w", err)
	}

	// Delete the aggregated raw rows.
	result, err := tx.Exec("DELETE FROM llm_usage WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete pruned rows: %w", err)
	}

	pruned, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return 0, fmt.Errorf("commit: %w", commitErr)
	}

	return pruned, nil
}

// GetDailyTokens returns per-day token totals for charting.
func (db *DB) GetDailyTokens(from, to string) []map[string]any {
	rows, err := db.conn.Query(`SELECT
		substr(timestamp, 1, 10) as day,
		SUM(input_tokens) as input,
		SUM(output_tokens) as output,
		COUNT(*) as calls
		FROM llm_usage
		WHERE substr(timestamp, 1, 10) >= ? AND substr(timestamp, 1, 10) <= ?
		GROUP BY day ORDER BY day`, from, to)
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
