package lancedbpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	writeSmokeManifestName    = "lancedb-write-smoke-manifest.json"
	writeSmokeRowsSummaryName = "lancedb-write-smoke-rows-summary.json"
	writeSmokeLogName         = "lancedb-write-smoke.log"
)

// PreflightResult reports LanceDB write-smoke dependency readiness.
type PreflightResult struct {
	Status     string `json:"status"`
	PythonPath string `json:"python_path,omitempty"`
	Message    string `json:"message,omitempty"`
}

// PythonLanceDBWriter invokes the local Python LanceDB helper script.
type PythonLanceDBWriter struct {
	PythonPath string
	ScriptPath string
}

func (w PythonLanceDBWriter) python() string {
	if strings.TrimSpace(w.PythonPath) != "" {
		return w.PythonPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_PYTHON")); env != "" {
		return env
	}
	return "python3"
}

func (w PythonLanceDBWriter) script() string {
	if strings.TrimSpace(w.ScriptPath) != "" {
		return w.ScriptPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_WRITE_SCRIPT")); env != "" {
		return env
	}
	return "scripts/lancedb_write_smoke.py"
}

func (w PythonLanceDBWriter) Write(req WriteRequest) (WriteResponse, error) {
	scriptPath := w.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return WriteResponse{}, fmt.Errorf("lancedb write script %q not found: install repo scripts or set DEONCLAW_LANCEDB_WRITE_SCRIPT", scriptPath)
	}

	payload, err := json.Marshal(map[string]any{
		"database_path":    req.DatabasePath,
		"table":            req.Table,
		"vector_column":    req.VectorColumn,
		"text_ref_column":  req.TextRefColumn,
		"metadata_columns": req.MetadataColumns,
		"rows":             req.Rows,
	})
	if err != nil {
		return WriteResponse{}, err
	}

	cmd := exec.Command(w.python(), scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return WriteResponse{}, fmt.Errorf("lancedb python write failed: %s", msg)
	}

	var response struct {
		Status   string `json:"status"`
		RowCount int    `json:"row_count"`
		Message  string `json:"message,omitempty"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return WriteResponse{}, fmt.Errorf("parse lancedb write response: %w", err)
	}
	if response.Status != StatusOK {
		if strings.TrimSpace(response.Message) == "" {
			response.Message = "lancedb write returned non-ok status"
		}
		return WriteResponse{}, fmt.Errorf("%s", response.Message)
	}
	return WriteResponse{RowCount: response.RowCount}, nil
}

// PreflightWriteSmoke checks python3 and the lancedb Python package without writing data.
func PreflightWriteSmoke() PreflightResult {
	writer := PythonLanceDBWriter{}
	python := writer.python()
	if _, err := exec.LookPath(python); err != nil {
		return PreflightResult{
			Status:  StatusFailed,
			Message: fmt.Sprintf("%s not found in PATH; install Python 3 and set DEONCLAW_LANCEDB_PYTHON if needed", python),
		}
	}

	cmd := exec.Command(python, "-c", "import lancedb")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "python lancedb import failed"
		}
		return PreflightResult{
			Status:     StatusFailed,
			PythonPath: python,
			Message:    fmt.Sprintf("%s; install with: pip install lancedb pyarrow", msg),
		}
	}

	scriptPath := writer.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return PreflightResult{
			Status:     StatusFailed,
			PythonPath: python,
			Message:    fmt.Sprintf("lancedb write script %q not found", scriptPath),
		}
	}

	return PreflightResult{
		Status:     StatusOK,
		PythonPath: python,
	}
}
