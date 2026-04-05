package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func getVectorExtCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-vector-ext",
		Short: "Download the sqlite-vec extension for native vector search",
		Long: `Downloads the latest prebuilt sqlite-vec shared library for your platform.
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

	// Resolve the latest release from GitHub API.
	osName := sqliteVecOS()
	assetPattern := fmt.Sprintf("sqlite-vec-*-loadable-%s-%s.tar.gz", osName, arch)

	fmt.Printf("Fetching latest sqlite-vec release for %s/%s...\n", runtime.GOOS, runtime.GOARCH)
	downloadURL, version, err := resolveGitHubAsset("asg017/sqlite-vec", assetPattern)
	if err != nil {
		return fmt.Errorf("cannot resolve sqlite-vec release: %w", err)
	}

	fmt.Printf("  Downloading %s...\n", version)

	// Download the tarball to a temp file.
	tmpTar := destPath + ".tar.gz"
	if dlErr := fetchFile(downloadURL, tmpTar, 5*time.Minute, true); dlErr != nil {
		_ = os.Remove(tmpTar)
		return fmt.Errorf("download failed: %w", dlErr)
	}
	defer func() { _ = os.Remove(tmpTar) }()

	// Extract the library from the tarball (vec0.so, vec0.dylib, or vec0.dll).
	libName := "vec0." + ext
	if extractErr := extractFromTarGz(tmpTar, libName, destPath); extractErr != nil {
		return fmt.Errorf("extract failed: %w", extractErr)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(destPath, 0o755)
	}

	info, _ := os.Stat(destPath)
	fmt.Printf("  ✓ sqlite-vec %s (%s)\n", version, formatBytes(info.Size()))
	fmt.Println("  sqlite-vec will be loaded automatically on next startup.")
	return nil
}

// sqliteVecOS maps runtime.GOOS to sqlite-vec release naming.
func sqliteVecOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS
	}
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

// resolveGitHubAsset queries the GitHub releases API for the latest release
// and returns the download URL for the asset matching the glob pattern.
func resolveGitHubAsset(repo, pattern string) (downloadURL, version string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	client := http.Client{Timeout: 15 * time.Second}
	resp, httpErr := client.Get(apiURL)
	if httpErr != nil {
		return "", "", fmt.Errorf("GitHub API request failed: %w", httpErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("repository %s has no releases (may be private or unavailable)", repo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if decErr := json.NewDecoder(resp.Body).Decode(&release); decErr != nil {
		return "", "", fmt.Errorf("parse GitHub response: %w", decErr)
	}

	for _, asset := range release.Assets {
		matched, _ := filepath.Match(pattern, asset.Name)
		if matched {
			return asset.BrowserDownloadURL, release.TagName, nil
		}
	}

	return "", "", fmt.Errorf("no asset matching %q in release %s", pattern, release.TagName)
}

// extractFromTarGz extracts a single file from a .tar.gz archive.
func extractFromTarGz(archivePath, targetName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, tarErr := tr.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return fmt.Errorf("tar read: %w", tarErr)
		}

		if strings.HasSuffix(hdr.Name, targetName) || hdr.Name == targetName {
			out, createErr := os.Create(destPath)
			if createErr != nil {
				return createErr
			}
			if _, cpErr := io.Copy(out, tr); cpErr != nil {
				_ = out.Close()
				return cpErr
			}
			return out.Close()
		}
	}

	return fmt.Errorf("file %q not found in archive", targetName)
}
