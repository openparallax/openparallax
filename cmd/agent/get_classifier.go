package main

import (
	"archive/tar"
	"compress/gzip"
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

var (
	classifierVariant string
	classifierForce   bool
)

var getClassifierCmd = &cobra.Command{
	Use:   "get-classifier",
	Short: "Download the Shield prompt-injection classifier",
	Long: `Downloads the DeBERTa ONNX model and ONNX Runtime library for in-process
Shield classification. This enables Tier 1 ML-based injection detection
alongside the heuristic rules.

Without the classifier, Shield runs in heuristic-only mode.`,
	SilenceUsage: true,
	RunE:         runGetClassifier,
}

func init() {
	getClassifierCmd.Flags().StringVar(&classifierVariant, "variant", "base", "model variant: base (~700MB) or small (~250MB)")
	getClassifierCmd.Flags().BoolVar(&classifierForce, "force", false, "re-download even if files exist")
	rootCmd.AddCommand(getClassifierCmd)
}

// modelVariant holds download URLs for a model variant.
type modelVariant struct {
	repo        string // HuggingFace repo
	description string
	files       []string // files to download from onnx/ subdirectory
}

var modelVariants = map[string]modelVariant{
	"base": {
		repo:        "openparallax/shield-classifier-v1",
		description: "Fine-tuned DeBERTa-v3-base (~700MB, 98.8% accuracy)",
		files:       []string{"model.onnx", "tokenizer.json"},
	},
	"small": {
		repo:        "ProtectAI/deberta-v3-small-prompt-injection-v2",
		description: "DeBERTa-v3-small (~250MB, faster inference)",
		files:       []string{"model.onnx", "tokenizer.json"},
	},
}

// onnxRuntimeVersion is the ONNX Runtime version to download.
const onnxRuntimeVersion = "1.23.0"

func runGetClassifier(_ *cobra.Command, _ []string) error {
	variant, ok := modelVariants[classifierVariant]
	if !ok {
		return fmt.Errorf("unknown variant %q (available: base, small)", classifierVariant)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	modelDir := filepath.Join(home, ".openparallax", "models", "prompt-injection")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}

	fmt.Printf("Shield Classifier Setup\n")
	fmt.Printf("  Variant:  %s\n", variant.description)
	fmt.Printf("  Location: %s\n\n", modelDir)

	// Download model files from HuggingFace.
	for _, file := range variant.files {
		destPath := filepath.Join(modelDir, file)
		if !classifierForce {
			if _, statErr := os.Stat(destPath); statErr == nil {
				fmt.Printf("  ✓ %s (already exists)\n", file)
				continue
			}
		}
		url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/onnx/%s", variant.repo, file)
		fmt.Printf("  Downloading %s...\n", file)
		if dlErr := downloadFile(url, destPath); dlErr != nil {
			return fmt.Errorf("download %s: %w", file, dlErr)
		}
		info, _ := os.Stat(destPath)
		fmt.Printf("  ✓ %s (%s)\n", file, formatBytes(info.Size()))
	}

	// Download ONNX Runtime shared library.
	libName := onnxRuntimeLibName()
	libPath := filepath.Join(modelDir, libName)
	if !classifierForce {
		if _, statErr := os.Stat(libPath); statErr == nil {
			fmt.Printf("  ✓ %s (already exists)\n", libName)
			fmt.Println("\n  Setup complete. Shield will use the classifier on next start.")
			return nil
		}
	}

	fmt.Printf("  Downloading ONNX Runtime %s...\n", onnxRuntimeVersion)
	if dlErr := downloadONNXRuntime(modelDir); dlErr != nil {
		return fmt.Errorf("download ONNX Runtime: %w", dlErr)
	}
	fmt.Printf("  ✓ %s\n", libName)

	fmt.Println("\n  Setup complete. Shield will use the classifier on next start.")
	return nil
}

// downloadFile downloads a URL to a local file with progress.
func downloadFile(url, destPath string) error {
	client := http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(url)
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

// downloadONNXRuntime downloads and extracts the ONNX Runtime shared library.
func downloadONNXRuntime(destDir string) error {
	archiveName, libFile := onnxRuntimeArchive()
	url := fmt.Sprintf("https://github.com/microsoft/onnxruntime/releases/download/v%s/%s", onnxRuntimeVersion, archiveName)

	client := http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// ONNX Runtime releases are .tgz archives.
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, tarErr := tr.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return fmt.Errorf("tar reader: %w", tarErr)
		}

		// Look for the shared library file inside the archive.
		if strings.HasSuffix(header.Name, libFile) {
			destPath := filepath.Join(destDir, onnxRuntimeLibName())
			f, createErr := os.Create(destPath)
			if createErr != nil {
				return createErr
			}
			if _, copyErr := io.Copy(f, tr); copyErr != nil {
				_ = f.Close()
				return copyErr
			}
			_ = f.Close()
			return os.Chmod(destPath, 0o755)
		}
	}

	return fmt.Errorf("library %s not found in archive %s", libFile, archiveName)
}

// onnxRuntimeLibName returns the platform-specific library filename.
func onnxRuntimeLibName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}

// onnxRuntimeArchive returns the archive name and library path within it.
func onnxRuntimeArchive() (archive, libPath string) {
	version := onnxRuntimeVersion
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("onnxruntime-linux-x64-%s.tgz", version),
			"lib/libonnxruntime.so." + version
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return fmt.Sprintf("onnxruntime-linux-aarch64-%s.tgz", version),
			"lib/libonnxruntime.so." + version
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return fmt.Sprintf("onnxruntime-osx-arm64-%s.tgz", version),
			"lib/libonnxruntime." + version + ".dylib"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("onnxruntime-osx-x86_64-%s.tgz", version),
			"lib/libonnxruntime." + version + ".dylib"
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("onnxruntime-win-x64-%s.zip", version),
			"lib/onnxruntime.dll"
	default:
		return fmt.Sprintf("onnxruntime-linux-x64-%s.tgz", version),
			"lib/libonnxruntime.so." + version
	}
}
