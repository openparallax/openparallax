package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// fetchFile downloads a URL to a local file using atomic write (tmp + rename).
// If force is false and the file already exists, it is skipped and returns nil.
func fetchFile(url, destPath string, timeout time.Duration, force bool) error {
	if !force {
		if _, err := os.Stat(destPath); err == nil {
			return nil // Already exists.
		}
	}

	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url) //nolint:gosec // URLs constructed from hardcoded templates
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	written, err := io.Copy(f, resp.Body)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if written == 0 {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("downloaded 0 bytes")
	}

	return os.Rename(tmpPath, destPath)
}

// openparallaxHome returns the path to ~/.openparallax, creating it if needed.
func openparallaxHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".openparallax"), nil
}
