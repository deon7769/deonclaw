package lancedbpolicy

// LanceDBWriter persists validated vector rows into a local LanceDB database.
type LanceDBWriter interface {
	Write(req WriteRequest) (WriteResponse, error)
}

// WriteRequest is the adapter-neutral write contract.
type WriteRequest struct {
	DatabasePath    string
	Table           string
	VectorColumn    string
	TextRefColumn   string
	MetadataColumns []string
	Rows            []WriteRow
}

// WriteRow is one LanceDB row with full vector payload for database write only.
type WriteRow struct {
	ChunkID        string    `json:"chunk_id"`
	VectorID       string    `json:"vector_id"`
	Vector         []float64 `json:"vector"`
	Domain         string    `json:"domain"`
	SourcePath     string    `json:"source_path"`
	SourceSHA256   string    `json:"source_sha256"`
	TextSHA256     string    `json:"text_sha256"`
	EmbeddingModel string    `json:"embedding_model"`
	Provider       string    `json:"provider"`
}

// WriteResponse summarizes a successful adapter write.
type WriteResponse struct {
	RowCount int
}

// DefaultWriter is the production LanceDB writer adapter.
var DefaultWriter LanceDBWriter = PythonLanceDBWriter{}
