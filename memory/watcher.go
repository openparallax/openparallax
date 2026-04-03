package memory

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/openparallax/openparallax/internal/logging"
)

// StartWatcher monitors workspace memory files for changes and triggers
// async reindex when files are modified. Uses a 1.5-second debounce
// to coalesce rapid successive writes into a single reindex.
func StartWatcher(ctx context.Context, workspacePath string, indexer *Indexer, log *logging.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	watchPaths := []string{
		filepath.Join(workspacePath, "MEMORY.md"),
		filepath.Join(workspacePath, "USER.md"),
		filepath.Join(workspacePath, "HEARTBEAT.md"),
		filepath.Join(workspacePath, "memory"),
	}

	for _, p := range watchPaths {
		if _, statErr := os.Stat(p); statErr == nil {
			if addErr := watcher.Add(p); addErr != nil && log != nil {
				log.Warn("watcher_add_failed", "path", p, "error", addErr)
			}
		}
	}

	go func() {
		debounce := time.NewTimer(0)
		debounce.Stop()
		defer func() { _ = watcher.Close() }()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					debounce.Reset(1500 * time.Millisecond)
				}
			case <-debounce.C:
				if log != nil {
					log.Info("watcher_reindex_triggered")
				}
				indexer.IndexWorkspace(ctx, workspacePath)
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
