package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const sqliteVecVersion = "v0.1.6"

func getVectorExtCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-vector-ext",
		Short: "Download the sqlite-vec extension for native vector search",
		Long: `Downloads the prebuilt sqlite-vec shared library for your platform.
Once installed, memory search uses native in-database vector queries
instead of the builtin pure-Go cosine similarity. Both modes produce
identical results — native is faster at scale (50K+ chunks).

The extension is stored at ~/.openparallax/extensions/sqlite-vec.<ext>
and loaded automatically on next startup.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return downloadSqliteVec()
		},
	}
}

func downloadSqliteVec() error {
	var ext, arch string

	switch runtime.GOOS {
	case "linux":
		ext = "so"
	case "darwin":
		ext = "dylib"
	case "windows":
		ext = "dll"
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(homeDir, ".openparallax", "extensions")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("cannot create extensions directory: %w", err)
	}
	destPath := filepath.Join(destDir, "sqlite-vec."+ext)

	// Build download URL.
	// sqlite-vec publishes prebuilt binaries on GitHub releases.
	url := fmt.Sprintf(
		"https://github.com/asg017/sqlite-vec/releases/download/%s/sqlite-vec-%s-%s-%s.%s",
		sqliteVecVersion, sqliteVecVersion, runtime.GOOS, arch, ext,
	)

	fmt.Printf("Downloading sqlite-vec %s for %s/%s...\n", sqliteVecVersion, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("URL: %s\n", url)

	resp, err := http.Get(url) //nolint:gosec // URL is hardcoded, not user-supplied
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d (the prebuilt binary may not be available for your platform)", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	defer func() { _ = out.Close() }()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, 0o755)
	}

	fmt.Printf("Downloaded %d bytes to %s\n", written, destPath)
	fmt.Println("sqlite-vec will be loaded automatically on next startup.")
	fmt.Println("Memory search will use native in-database vector queries.")
	return nil
}
