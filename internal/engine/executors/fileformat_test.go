package executors

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileFormatSupportedActions(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	assert.Len(t, exec.SupportedActions(), 5)
}

func TestFileFormatToolSchemas(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	schemas := exec.ToolSchemas()
	assert.Len(t, schemas, 5)
	names := make(map[string]bool)
	for _, s := range schemas {
		names[s.Name] = true
	}
	assert.True(t, names["archive_create"])
	assert.True(t, names["archive_extract"])
	assert.True(t, names["pdf_read"])
	assert.True(t, names["spreadsheet_read"])
	assert.True(t, names["spreadsheet_write"])
}

func TestArchiveCreateExtractZip(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("world"), 0o644))

	exec := NewFileFormatExecutor(dir)

	// Create zip.
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveCreate, Payload: map[string]any{
			"output": filepath.Join(dir, "test.zip"),
			"paths":  []any{srcDir},
		},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "test.zip")

	// Extract zip.
	extractDir := filepath.Join(dir, "extracted")
	result = exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveExtract, Payload: map[string]any{
			"archive":     filepath.Join(dir, "test.zip"),
			"destination": extractDir,
		},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Extracted")

	// Verify extracted content.
	data, err := os.ReadFile(filepath.Join(extractDir, "src", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestArchiveCreateExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("tar content"), 0o644))

	exec := NewFileFormatExecutor(dir)

	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveCreate, Payload: map[string]any{
			"output": filepath.Join(dir, "test.tar.gz"),
			"paths":  []any{srcDir},
		},
	})
	require.True(t, result.Success)

	extractDir := filepath.Join(dir, "tar-out")
	result = exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveExtract, Payload: map[string]any{
			"archive":     filepath.Join(dir, "test.tar.gz"),
			"destination": extractDir,
		},
	})
	require.True(t, result.Success)
}

func TestArchiveUnsupportedFormat(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveCreate, Payload: map[string]any{
			"output": "test.rar", "paths": []any{"src"},
		},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, ".zip")
}

func TestArchiveExtractUnsupportedFormat(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionArchiveExtract, Payload: map[string]any{"archive": "test.rar"},
	})
	assert.False(t, result.Success)
}

func TestZipSlipProtection(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	zf, err := os.Create(zipPath)
	require.NoError(t, err)
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("../../etc/passwd")
	_, _ = w.Write([]byte("evil"))
	_ = zw.Close()
	_ = zf.Close()

	extractDir := filepath.Join(dir, "safe")
	require.NoError(t, os.MkdirAll(extractDir, 0o755))
	_, err = extractZip(zipPath, extractDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zip slip")
}

func TestCSVReadWrite(t *testing.T) {
	dir := t.TempDir()
	exec := NewFileFormatExecutor(dir)
	csvPath := filepath.Join(dir, "data.csv")

	// Write CSV.
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetWrite, Payload: map[string]any{
			"path":    csvPath,
			"headers": []any{"Name", "Age"},
			"rows":    []any{[]any{"Alice", "30"}, []any{"Bob", "25"}},
		},
	})
	require.True(t, result.Success)

	// Read CSV.
	result = exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetRead, Payload: map[string]any{"path": csvPath},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Alice")
	assert.Contains(t, result.Output, "Bob")
	assert.Contains(t, result.Output, "2 rows")
}

func TestTSVReadWrite(t *testing.T) {
	dir := t.TempDir()
	tsvPath := filepath.Join(dir, "data.tsv")

	// Write TSV manually.
	f, err := os.Create(tsvPath)
	require.NoError(t, err)
	w := csv.NewWriter(f)
	w.Comma = '\t'
	_ = w.Write([]string{"Col1", "Col2"})
	_ = w.Write([]string{"val1", "val2"})
	w.Flush()
	_ = f.Close()

	exec := NewFileFormatExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetRead, Payload: map[string]any{"path": tsvPath},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Col1")
	assert.Contains(t, result.Output, "val1")
}

func TestXLSXReadWrite(t *testing.T) {
	dir := t.TempDir()
	exec := NewFileFormatExecutor(dir)
	xlsxPath := filepath.Join(dir, "data.xlsx")

	// Write XLSX.
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetWrite, Payload: map[string]any{
			"path":    xlsxPath,
			"headers": []any{"Name", "Score"},
			"rows":    []any{[]any{"Charlie", "95"}, []any{"Diana", "88"}},
		},
	})
	require.True(t, result.Success)

	// Read XLSX.
	result = exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetRead, Payload: map[string]any{"path": xlsxPath},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "Charlie")
	assert.Contains(t, result.Output, "Diana")
}

func TestSpreadsheetMaxRows(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "big.csv")

	f, err := os.Create(csvPath)
	require.NoError(t, err)
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Value"})
	for i := range 200 {
		_ = w.Write([]string{fmt.Sprintf("%d", i), "data"})
	}
	w.Flush()
	_ = f.Close()

	exec := NewFileFormatExecutor(dir)
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetRead, Payload: map[string]any{"path": csvPath, "max_rows": float64(10)},
	})
	require.True(t, result.Success)
	assert.Contains(t, result.Output, "10 rows shown of 200 total")
}

func TestSpreadsheetUnsupportedFormat(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetRead, Payload: map[string]any{"path": "data.ods"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unsupported format")
}

func TestSpreadsheetWriteEmptyRows(t *testing.T) {
	exec := NewFileFormatExecutor(t.TempDir())
	result := exec.Execute(context.Background(), &types.ActionRequest{
		Type: types.ActionSpreadsheetWrite, Payload: map[string]any{"path": "out.csv"},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "rows is required")
}
