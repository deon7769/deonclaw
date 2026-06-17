package lancedbpolicy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const MaxSearchTopK = 50

// PythonLanceDBSearcher invokes the local Python LanceDB search helper script.
type PythonLanceDBSearcher struct {
	PythonPath string
	ScriptPath string
}

func (s PythonLanceDBSearcher) python() string {
	if strings.TrimSpace(s.PythonPath) != "" {
		return s.PythonPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_PYTHON")); env != "" {
		return env
	}
	return "python3"
}

func (s PythonLanceDBSearcher) script() string {
	if strings.TrimSpace(s.ScriptPath) != "" {
		return s.ScriptPath
	}
	if env := strings.TrimSpace(os.Getenv("DEONCLAW_LANCEDB_SEARCH_SCRIPT")); env != "" {
		return env
	}
	return "scripts/lancedb_search_smoke.py"
}

func (s PythonLanceDBSearcher) Search(req SearchRequest) (SearchResponse, error) {
	scriptPath := s.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return SearchResponse{}, fmt.Errorf("lancedb search script %q not found: install repo scripts or set DEONCLAW_LANCEDB_SEARCH_SCRIPT", scriptPath)
	}

	payload := map[string]any{
		"database_path":       req.DatabasePath,
		"table":               req.Table,
		"vector_column":       req.VectorColumn,
		"text_ref_column":     req.TextRefColumn,
		"top_k":               req.TopK,
		"expected_dimensions": req.ExpectedDimensions,
	}
	if len(req.QueryVector) > 0 {
		payload["query_vector"] = req.QueryVector
	}
	if strings.TrimSpace(req.QueryChunkID) != "" {
		payload["query_chunk_id"] = req.QueryChunkID
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return SearchResponse{}, err
	}

	cmd := exec.Command(s.python(), scriptPath)
	cmd.Stdin = bytes.NewReader(payloadBytes)
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
		return SearchResponse{}, fmt.Errorf("lancedb python search failed: %s", msg)
	}

	var response SearchResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return SearchResponse{}, fmt.Errorf("parse lancedb search response: %w", err)
	}
	if response.Status != StatusOK {
		if strings.TrimSpace(response.Message) == "" {
			response.Message = "lancedb search returned non-ok status"
		}
		return response, fmt.Errorf("%s", response.Message)
	}
	return response, nil
}

// PreflightSearchSmoke checks python3, lancedb, and the search helper script.
func PreflightSearchSmoke() PreflightResult {
	searcher := PythonLanceDBSearcher{}
	python := searcher.python()
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

	scriptPath := searcher.script()
	if _, err := os.Stat(scriptPath); err != nil {
		return PreflightResult{
			Status:     StatusFailed,
			PythonPath: python,
			Message:    fmt.Sprintf("lancedb search script %q not found", scriptPath),
		}
	}

	return PreflightResult{
		Status:     StatusOK,
		PythonPath: python,
	}
}
