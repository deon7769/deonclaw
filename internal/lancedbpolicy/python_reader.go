package lancedbpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PythonLanceDBReader invokes the local Python LanceDB readback helper script.
type PythonLanceDBReader struct {
	PythonPath string
	ScriptPath string
}

func (r PythonLanceDBReader) python() string {
	if strings.TrimSpace(r.PythonPath) != "" {
		return r.PythonPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_PYTHON")); env != "" {
		return env
	}
	return "python3"
}

func (r PythonLanceDBReader) script() string {
	if strings.TrimSpace(r.ScriptPath) != "" {
		return r.ScriptPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_READBACK_SCRIPT")); env != "" {
		return env
	}
	return "scripts/lancedb_readback.py"
}

func (r PythonLanceDBReader) Readback(req ReadbackRequest) (ReadbackResponse, error) {
	scriptPath := r.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return ReadbackResponse{}, fmt.Errorf("lancedb readback script %q not found: install repo scripts or set DEONCLAW_LANCEDB_READBACK_SCRIPT", scriptPath)
	}

	payload, err := json.Marshal(map[string]any{
		"database_path":       req.DatabasePath,
		"table":               req.Table,
		"vector_column":       req.VectorColumn,
		"text_ref_column":     req.TextRefColumn,
		"metadata_columns":    req.MetadataColumns,
		"expected_dimensions": req.ExpectedDimensions,
	})
	if err != nil {
		return ReadbackResponse{}, err
	}

	cmd := exec.Command(r.python(), scriptPath)
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
		return ReadbackResponse{}, fmt.Errorf("lancedb python readback failed: %s", msg)
	}

	var response ReadbackResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return ReadbackResponse{}, fmt.Errorf("parse lancedb readback response: %w", err)
	}
	if response.Status != StatusOK {
		if strings.TrimSpace(response.Message) == "" {
			response.Message = "lancedb readback returned non-ok status"
		}
		return response, fmt.Errorf("%s", response.Message)
	}
	return response, nil
}

// PreflightReadback checks python3, lancedb, and the readback helper script.
func PreflightReadback() PreflightResult {
	reader := PythonLanceDBReader{}
	python := reader.python()
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

	scriptPath := reader.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return PreflightResult{
			Status:     StatusFailed,
			PythonPath: python,
			Message:    fmt.Sprintf("lancedb readback script %q not found", scriptPath),
		}
	}

	return PreflightResult{
		Status:     StatusOK,
		PythonPath: python,
	}
}
