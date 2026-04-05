// Package heartbeat implements the cron-based task scheduler that reads
// HEARTBEAT.md and fires tasks at scheduled times through the engine pipeline.
package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/logging"
)

// TaskFn is the function called when a cron entry fires.
type TaskFn func(task string)

// CronEntry is a parsed cron expression with its associated task.
type CronEntry struct {
	Minute  string
	Hour    string
	Day     string
	Month   string
	Weekday string
	Task    string
	lastRun time.Time
}

// Matches returns true if the entry matches the given time.
func (e *CronEntry) Matches(t time.Time) bool {
	return matchField(e.Minute, t.Minute()) &&
		matchField(e.Hour, t.Hour()) &&
		matchField(e.Day, t.Day()) &&
		matchField(e.Month, int(t.Month())) &&
		matchField(e.Weekday, int(t.Weekday()))
}

// FiredThisMinute returns true if the entry already fired this minute.
func (e *CronEntry) FiredThisMinute(t time.Time) bool {
	return e.lastRun.Truncate(time.Minute).Equal(t.Truncate(time.Minute))
}

// MarkFired records that the entry fired at this time.
func (e *CronEntry) MarkFired(t time.Time) {
	e.lastRun = t
}

// Loop manages the heartbeat cron scheduler.
type Loop struct {
	workspacePath string
	onFire        TaskFn
	log           *logging.Logger
	entries       []*CronEntry
	ticker        *time.Ticker
	mu            sync.Mutex
}

// NewLoop creates a heartbeat loop.
func NewLoop(workspacePath string, onFire TaskFn, log *logging.Logger) *Loop {
	l := &Loop{
		workspacePath: workspacePath,
		onFire:        onFire,
		log:           log,
	}
	l.Reload()
	return l
}

// Start begins the 60-second tick loop.
func (l *Loop) Start(ctx context.Context) {
	l.ticker = time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case t := <-l.ticker.C:
				l.checkAndFire(t)
			case <-ctx.Done():
				l.ticker.Stop()
				return
			}
		}
	}()
}

// Reload re-parses HEARTBEAT.md for cron entries.
func (l *Loop) Reload() {
	l.mu.Lock()
	defer l.mu.Unlock()

	path := filepath.Join(l.workspacePath, "HEARTBEAT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if l.log != nil && !os.IsNotExist(err) {
			l.log.Warn("heartbeat_reload_failed", "path", path, "error", err)
		}
		return
	}

	l.entries = ParseCronEntries(string(data))
	if l.log != nil {
		// Count candidate lines (start with "- `") to detect parse failures.
		candidates := 0
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "- `") {
				candidates++
			}
		}
		if skipped := candidates - len(l.entries); skipped > 0 {
			l.log.Warn("heartbeat_parse_skipped", "skipped", skipped, "parsed", len(l.entries))
		}
		l.log.Info("heartbeat_reload", "entries", len(l.entries))
	}
}

// EntryCount returns the number of loaded cron entries.
func (l *Loop) EntryCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

func (l *Loop) checkAndFire(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Matches(now) && !entry.FiredThisMinute(now) {
			entry.MarkFired(now)
			if l.log != nil {
				l.log.Info("heartbeat_fire", "task", entry.Task)
			}
			if l.onFire != nil {
				go l.onFire(entry.Task)
			}
		}
	}
}

// cronLineRe matches cron entries in HEARTBEAT.md:
// - `0 9 * * *` — Task description here
var cronLineRe = regexp.MustCompile("^-\\s*`([^`]+)`\\s*[—–-]\\s*(.+)$")

// ParseCronEntries parses cron entries from HEARTBEAT.md content.
func ParseCronEntries(content string) []*CronEntry {
	var entries []*CronEntry
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		matches := cronLineRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		cronExpr := strings.TrimSpace(matches[1])
		task := strings.TrimSpace(matches[2])

		fields := strings.Fields(cronExpr)
		if len(fields) != 5 {
			continue
		}

		entries = append(entries, &CronEntry{
			Minute:  fields[0],
			Hour:    fields[1],
			Day:     fields[2],
			Month:   fields[3],
			Weekday: fields[4],
			Task:    task,
		})
	}
	return entries
}

// matchField checks if a cron field matches a time value.
// Supports: "*", exact numbers, "*/N" step values, and comma-separated values.
func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}

	// Step: */N
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return false
		}
		return value%step == 0
	}

	// Comma-separated
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if n, err := strconv.Atoi(part); err == nil && n == value {
			return true
		}
	}

	return false
}
