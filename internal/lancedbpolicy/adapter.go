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

// LanceDBReader performs structural readback from a local LanceDB database.
type LanceDBReader interface {
	Readback(req ReadbackRequest) (ReadbackResponse, error)
}

// ReadbackRequest is the adapter-neutral readback contract.
type ReadbackRequest struct {
	DatabasePath       string
	Table              string
	VectorColumn       string
	TextRefColumn      string
	MetadataColumns    []string
	ExpectedDimensions int
}

// ReadbackResponse summarizes structural LanceDB table metadata.
type ReadbackResponse struct {
	Status                 string   `json:"status"`
	TableExists            bool     `json:"table_exists"`
	RowCount               int      `json:"row_count"`
	Columns                []string `json:"columns"`
	VectorColumnExists     bool     `json:"vector_column_exists"`
	TextRefColumnExists    bool     `json:"text_ref_column_exists"`
	MetadataColumnsPresent []string `json:"metadata_columns_present"`
	InferredDimensions     int      `json:"inferred_dimensions"`
	SampleChunkIDs         []string `json:"sample_chunk_ids"`
	Message                string   `json:"message,omitempty"`
}

// DefaultReader is the production LanceDB readback adapter.
var DefaultReader LanceDBReader = PythonLanceDBReader{}

// LanceDBSearcher performs controlled vector search against a local LanceDB table.
type LanceDBSearcher interface {
	Search(req SearchRequest) (SearchResponse, error)
}

// SearchRequest is the adapter-neutral search contract.
type SearchRequest struct {
	DatabasePath       string
	Table              string
	VectorColumn       string
	TextRefColumn      string
	TopK               int
	QueryVector        []float64
	QueryChunkID       string
	ExpectedDimensions int
}

// SearchHit is one ranked search result without vector or chunk text payloads.
type SearchHit struct {
	Rank           int     `json:"rank"`
	ChunkID        string  `json:"chunk_id"`
	VectorID       string  `json:"vector_id"`
	Distance       float64 `json:"distance"`
	Domain         string  `json:"domain"`
	SourcePath     string  `json:"source_path"`
	SourceSHA256   string  `json:"source_sha256"`
	TextSHA256     string  `json:"text_sha256"`
	EmbeddingModel string  `json:"embedding_model"`
	Provider       string  `json:"provider"`
}

// SearchResponse summarizes a controlled vector search run.
type SearchResponse struct {
	Status    string      `json:"status"`
	QueryMode string      `json:"query_mode"`
	TopK      int         `json:"top_k"`
	Results   []SearchHit `json:"results"`
	Message   string      `json:"message,omitempty"`
}

// DefaultSearcher is the production LanceDB search adapter.
var DefaultSearcher LanceDBSearcher = PythonLanceDBSearcher{}
