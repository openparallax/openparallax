package executors

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/xuri/excelize/v2"

	"github.com/openparallax/openparallax/internal/types"
)

const (
	maxArchiveSize    = 1 << 30 // 1 GB
	maxPDFPages       = 100
	maxPDFTextBytes   = 500 * 1024 // 500 KB
	maxSpreadsheetRow = 1000
)

// FileFormatExecutor handles archive, PDF, and spreadsheet operations.
type FileFormatExecutor struct {
	workspacePath string
}

// NewFileFormatExecutor creates a file format executor.
func NewFileFormatExecutor(workspacePath string) *FileFormatExecutor {
	return &FileFormatExecutor{workspacePath: workspacePath}
}

// WorkspaceScope reports that file-format operations are confined to the workspace.
func (f *FileFormatExecutor) WorkspaceScope() WorkspaceScope { return ScopeScoped }

// SupportedActions returns file format action types.
func (f *FileFormatExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionArchiveCreate, types.ActionArchiveExtract,
		types.ActionPDFRead,
		types.ActionSpreadsheetRead, types.ActionSpreadsheetWrite,
	}
}

// ToolSchemas returns tool definitions for file format tools.
func (f *FileFormatExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionArchiveCreate, Name: "archive_create", Description: "Create a zip or tar.gz archive from files or directories.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"output": map[string]any{"type": "string", "description": "Output archive path (.zip or .tar.gz)."}, "paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Files and directories to include."}}, "required": []string{"output", "paths"}}},
		{ActionType: types.ActionArchiveExtract, Name: "archive_extract", Description: "Extract a zip or tar.gz archive to a directory.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"archive": map[string]any{"type": "string", "description": "Path to archive file (.zip, .tar.gz, .tgz)."}, "destination": map[string]any{"type": "string", "description": "Directory to extract to. Default: current directory."}}, "required": []string{"archive"}}},
		{ActionType: types.ActionPDFRead, Name: "pdf_read", Description: "Extract text content from a PDF file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Path to the PDF file."}}, "required": []string{"path"}}},
		{ActionType: types.ActionSpreadsheetRead, Name: "spreadsheet_read", Description: "Read data from a CSV or Excel (.xlsx) file as a table.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Path to .csv, .tsv, or .xlsx file."}, "sheet": map[string]any{"type": "string", "description": "Sheet name for .xlsx files. Default: first sheet."}, "max_rows": map[string]any{"type": "integer", "description": "Max rows to return. Default: 100, max: 1000."}}, "required": []string{"path"}}},
		{ActionType: types.ActionSpreadsheetWrite, Name: "spreadsheet_write", Description: "Write data to a CSV or Excel (.xlsx) file.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "Output file path (.csv, .tsv, .xlsx)."}, "headers": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Column headers."}, "rows": map[string]any{"type": "array", "items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "description": "Data rows."}}, "required": []string{"path", "rows"}}},
	}
}

// Execute dispatches to the appropriate file format operation.
func (f *FileFormatExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionArchiveCreate:
		return f.archiveCreate(action)
	case types.ActionArchiveExtract:
		return f.archiveExtract(action)
	case types.ActionPDFRead:
		return f.pdfRead(action)
	case types.ActionSpreadsheetRead:
		return f.spreadsheetRead(action)
	case types.ActionSpreadsheetWrite:
		return f.spreadsheetWrite(action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown file format action"}
	}
}

// --- Archive Create ---

func (f *FileFormatExecutor) archiveCreate(action *types.ActionRequest) *types.ActionResult {
	output := ResolvePath(action.Payload["output"], f.workspacePath)
	pathsRaw, _ := action.Payload["paths"].([]any)
	if len(pathsRaw) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "paths is required"}
	}

	var paths []string
	for _, p := range pathsRaw {
		if s, ok := p.(string); ok {
			paths = append(paths, ResolvePath(s, f.workspacePath))
		}
	}

	switch {
	case strings.HasSuffix(output, ".zip"):
		if err := createZip(output, paths); err != nil {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
		}
	case strings.HasSuffix(output, ".tar.gz") || strings.HasSuffix(output, ".tgz"):
		if err := createTarGz(output, paths); err != nil {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
		}
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "output must end in .zip, .tar.gz, or .tgz"}
	}

	info, _ := os.Stat(output)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Archive created: %s (%s)", filepath.Base(output), formatFileSize(size)),
		Summary: fmt.Sprintf("created %s", filepath.Base(output)),
	}
}

func createZip(output string, paths []string) error {
	_ = os.MkdirAll(filepath.Dir(output), 0o755)
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	defer func() { _ = w.Close() }()

	for _, p := range paths {
		if addErr := addToZip(w, p, filepath.Dir(p)); addErr != nil {
			return addErr
		}
	}
	return nil
}

func addToZip(w *zip.Writer, path, baseDir string) error {
	return filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(baseDir, fp)
		zw, createErr := w.Create(rel)
		if createErr != nil {
			return createErr
		}
		data, readErr := os.ReadFile(fp)
		if readErr != nil {
			return readErr
		}
		_, writeErr := zw.Write(data)
		return writeErr
	})
}

func createTarGz(output string, paths []string) error {
	_ = os.MkdirAll(filepath.Dir(output), 0o755)
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gw := gzip.NewWriter(f)
	defer func() { _ = gw.Close() }()
	tw := tar.NewWriter(gw)
	defer func() { _ = tw.Close() }()

	for _, p := range paths {
		if addErr := addToTar(tw, p, filepath.Dir(p)); addErr != nil {
			return addErr
		}
	}
	return nil
}

func addToTar(tw *tar.Writer, path, baseDir string) error {
	return filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		header, headerErr := tar.FileInfoHeader(info, "")
		if headerErr != nil {
			return headerErr
		}
		rel, _ := filepath.Rel(baseDir, fp)
		header.Name = rel
		if writeErr := tw.WriteHeader(header); writeErr != nil {
			return writeErr
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(fp)
		if readErr != nil {
			return readErr
		}
		_, writeErr := tw.Write(data)
		return writeErr
	})
}

// --- Archive Extract ---

func (f *FileFormatExecutor) archiveExtract(action *types.ActionRequest) *types.ActionResult {
	archive := ResolvePath(action.Payload["archive"], f.workspacePath)
	dest := f.workspacePath
	if d, ok := action.Payload["destination"].(string); ok && d != "" {
		dest = ResolvePath(d, f.workspacePath)
	}

	var count int
	var err error
	switch {
	case strings.HasSuffix(archive, ".zip"):
		count, err = extractZip(archive, dest)
	case strings.HasSuffix(archive, ".tar.gz") || strings.HasSuffix(archive, ".tgz"):
		count, err = extractTarGz(archive, dest)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unsupported archive format (use .zip, .tar.gz, or .tgz)"}
	}

	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error()}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: fmt.Sprintf("Extracted %d files to %s", count, dest), Summary: fmt.Sprintf("extracted %d files", count),
	}
}

func extractZip(archive, dest string) (int, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return 0, err
	}
	defer func() { _ = r.Close() }()

	count := 0
	for _, zf := range r.File {
		target := filepath.Join(dest, zf.Name)
		// Zip slip protection.
		rel, relErr := filepath.Rel(dest, target)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return count, fmt.Errorf("zip slip detected: %s", zf.Name)
		}

		if zf.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		rc, openErr := zf.Open()
		if openErr != nil {
			return count, openErr
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxArchiveSize))
		_ = rc.Close()
		if readErr != nil {
			return count, readErr
		}
		if writeErr := os.WriteFile(target, data, 0o644); writeErr != nil {
			return count, writeErr
		}
		count++
	}
	return count, nil
}

func extractTarGz(archive, dest string) (int, error) {
	f, err := os.Open(archive)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	count := 0
	for {
		header, headerErr := tr.Next()
		if headerErr == io.EOF {
			break
		}
		if headerErr != nil {
			return count, headerErr
		}

		target := filepath.Join(dest, header.Name)
		rel, relErr := filepath.Rel(dest, target)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return count, fmt.Errorf("tar slip detected: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			data, readErr := io.ReadAll(io.LimitReader(tr, maxArchiveSize))
			if readErr != nil {
				return count, readErr
			}
			if writeErr := os.WriteFile(target, data, 0o644); writeErr != nil {
				return count, writeErr
			}
			count++
		}
	}
	return count, nil
}

// --- PDF Read ---

func (f *FileFormatExecutor) pdfRead(action *types.ActionRequest) *types.ActionResult {
	path := ResolvePath(action.Payload["path"], f.workspacePath)

	pdfCtx, err := pdfapi.ReadContextFile(path)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("read PDF: %s", err)}
	}

	pageCount := pdfCtx.PageCount
	if pageCount == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "PDF has no pages.", Summary: "empty PDF"}
	}

	// Extract content to a temp directory.
	tmpDir, err := os.MkdirTemp("", "openparallax-pdf-*")
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("create temp dir: %s", err)}
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	conf := pdfmodel.NewDefaultConfiguration()
	if extractErr := pdfapi.ExtractContentFile(path, tmpDir, nil, conf); extractErr != nil {
		// Strip the temp dir path from any error so callers don't learn
		// where on disk OpenParallax stages PDF extraction.
		msg := strings.ReplaceAll(extractErr.Error(), tmpDir, "<tmp>")
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("extract PDF content: %s", msg)}
	}

	// Read extracted text files from temp dir.
	var sb strings.Builder
	truncated := false
	totalBytes := 0
	pagesRead := 0

	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if entry.IsDir() || totalBytes > maxPDFTextBytes || pagesRead >= maxPDFPages {
			truncated = true
			break
		}
		data, readErr := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
		if readErr != nil {
			continue
		}
		pagesRead++
		if pagesRead > 1 {
			fmt.Fprintf(&sb, "\n--- Page %d ---\n", pagesRead)
		}
		sb.Write(data)
		totalBytes += len(data)
	}

	output := sb.String()
	if output == "" {
		output = "This PDF contains no extractable text (may be scanned images)."
	}
	if truncated {
		output += fmt.Sprintf("\n\n[Truncated — PDF has %d pages, showing first %d]", pageCount, pagesRead)
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output: output, Summary: fmt.Sprintf("PDF: %d pages, %d chars extracted", pageCount, len(output)),
	}
}

// --- Spreadsheet Read ---

func (f *FileFormatExecutor) spreadsheetRead(action *types.ActionRequest) *types.ActionResult {
	path := ResolvePath(action.Payload["path"], f.workspacePath)
	maxRows := 100
	if m, ok := action.Payload["max_rows"].(float64); ok && m > 0 {
		maxRows = int(m)
		if maxRows > maxSpreadsheetRow {
			maxRows = maxSpreadsheetRow
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return f.readCSV(action.RequestID, path, maxRows, ',')
	case ".tsv":
		return f.readCSV(action.RequestID, path, maxRows, '\t')
	case ".xlsx":
		sheet, _ := action.Payload["sheet"].(string)
		return f.readXLSX(action.RequestID, path, sheet, maxRows)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("unsupported format: %s", ext)}
	}
}

func (f *FileFormatExecutor) readCSV(requestID, path string, maxRows int, delimiter rune) *types.ActionResult {
	file, err := os.Open(path)
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: err.Error()}
	}
	defer func() { _ = file.Close() }()

	r := csv.NewReader(file)
	r.Comma = delimiter
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: fmt.Sprintf("parse CSV: %s", err)}
	}

	return formatTable(requestID, path, records, maxRows)
}

func (f *FileFormatExecutor) readXLSX(requestID, path, sheet string, maxRows int) *types.ActionResult {
	ef, err := excelize.OpenFile(path)
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: fmt.Sprintf("open xlsx: %s", err)}
	}
	defer func() { _ = ef.Close() }()

	if sheet == "" {
		sheet = ef.GetSheetName(0)
	}
	rows, err := ef.GetRows(sheet)
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: fmt.Sprintf("read sheet %q: %s", sheet, err)}
	}

	return formatTable(requestID, path, rows, maxRows)
}

func formatTable(requestID, path string, rows [][]string, maxRows int) *types.ActionResult {
	if len(rows) == 0 {
		return &types.ActionResult{RequestID: requestID, Success: true, Output: "No data.", Summary: "empty"}
	}

	totalRows := len(rows)
	if len(rows) > maxRows+1 { // +1 for header
		rows = rows[:maxRows+1]
	}

	// Calculate column widths.
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	widths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	// Cap column widths.
	for i := range widths {
		if widths[i] > 30 {
			widths[i] = 30
		}
		if widths[i] < 3 {
			widths[i] = 3
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s (%d rows, %d columns)\n\n", filepath.Base(path), totalRows-1, cols)

	// Header row.
	if len(rows) > 0 {
		sb.WriteString("| ")
		for i := range cols {
			cell := ""
			if i < len(rows[0]) {
				cell = rows[0][i]
			}
			if len(cell) > widths[i] {
				cell = cell[:widths[i]-2] + ".."
			}
			fmt.Fprintf(&sb, "%-*s | ", widths[i], cell)
		}
		sb.WriteString("\n|")
		for i := range cols {
			sb.WriteString(strings.Repeat("-", widths[i]+2))
			sb.WriteString("|")
		}
		sb.WriteString("\n")
	}

	// Data rows.
	for _, row := range rows[1:] {
		sb.WriteString("| ")
		for i := range cols {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if len(cell) > widths[i] {
				cell = cell[:widths[i]-2] + ".."
			}
			fmt.Fprintf(&sb, "%-*s | ", widths[i], cell)
		}
		sb.WriteString("\n")
	}

	if totalRows-1 > maxRows {
		fmt.Fprintf(&sb, "\n[%d rows shown of %d total]", maxRows, totalRows-1)
	}

	return &types.ActionResult{
		RequestID: requestID, Success: true,
		Output: sb.String(), Summary: fmt.Sprintf("%d rows, %d cols", totalRows-1, cols),
	}
}

// --- Spreadsheet Write ---

func (f *FileFormatExecutor) spreadsheetWrite(action *types.ActionRequest) *types.ActionResult {
	path := ResolvePath(action.Payload["path"], f.workspacePath)
	rowsRaw, _ := action.Payload["rows"].([]any)
	if len(rowsRaw) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "rows is required"}
	}

	var headers []string
	if h, ok := action.Payload["headers"].([]any); ok {
		for _, v := range h {
			if s, ok := v.(string); ok {
				headers = append(headers, s)
			}
		}
	}

	var rows [][]string
	for _, r := range rowsRaw {
		if rowArr, ok := r.([]any); ok {
			var row []string
			for _, cell := range rowArr {
				row = append(row, fmt.Sprintf("%v", cell))
			}
			rows = append(rows, row)
		}
	}

	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return f.writeCSV(action.RequestID, path, headers, rows, ',')
	case ".tsv":
		return f.writeCSV(action.RequestID, path, headers, rows, '\t')
	case ".xlsx":
		return f.writeXLSX(action.RequestID, path, headers, rows)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("unsupported format: %s", ext)}
	}
}

func (f *FileFormatExecutor) writeCSV(requestID, path string, headers []string, rows [][]string, delimiter rune) *types.ActionResult {
	file, err := os.Create(path)
	if err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: err.Error()}
	}
	defer func() { _ = file.Close() }()

	w := csv.NewWriter(file)
	w.Comma = delimiter
	if len(headers) > 0 {
		_ = w.Write(headers)
	}
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()

	return &types.ActionResult{
		RequestID: requestID, Success: true,
		Output: fmt.Sprintf("Wrote %d rows to %s", len(rows), filepath.Base(path)), Summary: fmt.Sprintf("wrote %s", filepath.Base(path)),
	}
}

func (f *FileFormatExecutor) writeXLSX(requestID, path string, headers []string, rows [][]string) *types.ActionResult {
	ef := excelize.NewFile()
	sheet := "Sheet1"

	// Write headers.
	if len(headers) > 0 {
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			_ = ef.SetCellValue(sheet, cell, h)
		}
	}

	// Write data rows.
	startRow := 1
	if len(headers) > 0 {
		startRow = 2
	}
	for ri, row := range rows {
		for ci, cell := range row {
			cellName, _ := excelize.CoordinatesToCellName(ci+1, startRow+ri)
			_ = ef.SetCellValue(sheet, cellName, cell)
		}
	}

	if err := ef.SaveAs(path); err != nil {
		return &types.ActionResult{RequestID: requestID, Success: false, Error: err.Error()}
	}

	return &types.ActionResult{
		RequestID: requestID, Success: true,
		Output: fmt.Sprintf("Wrote %d rows to %s", len(rows), filepath.Base(path)), Summary: fmt.Sprintf("wrote %s", filepath.Base(path)),
	}
}
