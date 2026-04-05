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

// defaultFetchTimeout is the default timeout for large file downloads.
const defaultFetchTimeout = 30 * time.Minute

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

// onnxRuntimeRepo is the GitHub repository for ONNX Runtime releases.
const onnxRuntimeRepo = "microsoft/onnxruntime"

func runGetClassifier(_ *cobra.Command, _ []string) error {
	variant, ok := modelVariants[classifierVariant]
	if !ok {
		return fmt.Errorf("unknown variant %q (available: base, small)", classifierVariant)
	}

	opHome, err := openparallaxHome()
	if err != nil {
		return err
	}

	modelDir := filepath.Join(opHome, "models", "prompt-injection")
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
		if dlErr := fetchFile(url, destPath, defaultFetchTimeout, classifierForce); dlErr != nil {
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

	fmt.Print("  Resolving latest ONNX Runtime...\n")
	if dlErr := downloadONNXRuntime(modelDir); dlErr != nil {
		return fmt.Errorf("download ONNX Runtime: %w", dlErr)
	}
	fmt.Printf("  ✓ %s\n", libName)

	fmt.Println("\n  Setup complete. Shield will use the classifier on next start.")
	return nil
}

// downloadONNXRuntime resolves the latest ONNX Runtime release from GitHub
// and extracts the shared library for the current platform.
func downloadONNXRuntime(destDir string) error {
	// Resolve latest release and find the matching archive asset.
	archivePattern := onnxRuntimeAssetPattern()
	downloadURL, version, err := resolveGitHubAsset(onnxRuntimeRepo, archivePattern)
	if err != nil {
		return fmt.Errorf("resolve ONNX Runtime release: %w", err)
	}
	// Strip the leading "v" from tag (e.g. "v1.23.0" → "1.23.0").
	semver := strings.TrimPrefix(version, "v")
	fmt.Printf("  Downloading ONNX Runtime %s...\n", version)

	client := http.Client{Timeout: 10 * time.Minute}
	resp, httpErr := client.Get(downloadURL)
	if httpErr != nil {
		return httpErr
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	// ONNX Runtime releases are .tgz archives.
	gz, gzErr := gzip.NewReader(resp.Body)
	if gzErr != nil {
		return fmt.Errorf("gzip reader: %w", gzErr)
	}
	defer func() { _ = gz.Close() }()

	// The library filename inside the archive embeds the version
	// (e.g. libonnxruntime.so.1.23.0, libonnxruntime.1.23.0.dylib).
	// Match by looking for the library name containing the semver.
	libName := onnxRuntimeLibName()
	tr := tar.NewReader(gz)
	for {
		header, tarErr := tr.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return fmt.Errorf("tar reader: %w", tarErr)
		}

		base := filepath.Base(header.Name)
		isLib := strings.Contains(base, semver) && (strings.HasPrefix(base, "libonnxruntime") || strings.HasPrefix(base, "onnxruntime"))
		if isLib {
			destPath := filepath.Join(destDir, libName)
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

	return fmt.Errorf("library not found in ONNX Runtime %s archive", version)
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

// onnxRuntimeAssetPattern returns a glob pattern matching the ONNX Runtime
// release asset for the current platform. Used with resolveGitHubAsset.
func onnxRuntimeAssetPattern() string {
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "onnxruntime-linux-x64-*.tgz"
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		return "onnxruntime-linux-aarch64-*.tgz"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "onnxruntime-osx-arm64-*.tgz"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return "onnxruntime-osx-x86_64-*.tgz"
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return "onnxruntime-win-x64-*.zip"
	default:
		return "onnxruntime-linux-x64-*.tgz"
	}
}
