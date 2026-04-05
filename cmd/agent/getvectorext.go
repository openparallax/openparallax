package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

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
	ext := sqliteVecExt()
	if ext == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	arch := sqliteVecArch()
	if arch == "" {
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	opHome, err := openparallaxHome()
	if err != nil {
		return err
	}

	destDir := filepath.Join(opHome, "extensions")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("cannot create extensions directory: %w", err)
	}
	destPath := filepath.Join(destDir, "sqlite-vec."+ext)

	url := fmt.Sprintf(
		"https://github.com/asg017/sqlite-vec/releases/download/%s/sqlite-vec-%s-%s-%s.%s",
		sqliteVecVersion, sqliteVecVersion, runtime.GOOS, arch, ext,
	)

	fmt.Printf("Downloading sqlite-vec %s for %s/%s...\n", sqliteVecVersion, runtime.GOOS, runtime.GOARCH)

	if dlErr := fetchFile(url, destPath, 5*time.Minute, true); dlErr != nil {
		return fmt.Errorf("download failed: %w", dlErr)
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, 0o755)
	}

	info, _ := os.Stat(destPath)
	fmt.Printf("  ✓ sqlite-vec (%s)\n", formatBytes(info.Size()))
	fmt.Println("  sqlite-vec will be loaded automatically on next startup.")
	return nil
}

func sqliteVecExt() string {
	switch runtime.GOOS {
	case "linux":
		return "so"
	case "darwin":
		return "dylib"
	case "windows":
		return "dll"
	default:
		return ""
	}
}

func sqliteVecArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return ""
	}
}
