package memoryindex

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type textChunk struct {
	Index     int
	CharStart int
	CharEnd   int
	Text      string
}

func Build(cfg Config, artifactsDir string, configBytes []byte) (BuildResult, error) {
	if err := Validate(cfg); err != nil {
		return BuildResult{}, err
	}
	if strings.TrimSpace(artifactsDir) == "" {
		return BuildResult{}, fmt.Errorf("artifacts dir is required")
	}

	scan, err := scanConfig(cfg)
	if err != nil {
		return BuildResult{}, err
	}

	manifestPath, chunksPath, err := resolveOutputPaths(cfg, artifactsDir)
	if err != nil {
		return BuildResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return BuildResult{}, fmt.Errorf("create artifacts dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(chunksPath), 0o755); err != nil {
		return BuildResult{}, fmt.Errorf("create artifacts dir: %w", err)
	}

	chunks := make([]Chunk, 0)
	for _, candidate := range scan.Candidates {
		content, err := os.ReadFile(candidate.Path)
		if err != nil {
			return BuildResult{}, fmt.Errorf("read source %q: %w", candidate.Path, err)
		}
		sourceSHA := sha256Hex(content)
		text := string(content)
		for _, piece := range chunkText(text, cfg.MemoryIndex.Chunking.MaxChars, cfg.MemoryIndex.Chunking.OverlapChars) {
			chunks = append(chunks, Chunk{
				ID:           chunkID(candidate.Domain, sourceSHA, piece.Index),
				Domain:       candidate.Domain,
				SourcePath:   candidate.Path,
				SourceSHA256: sourceSHA,
				ChunkIndex:   piece.Index,
				Text:         piece.Text,
				TextSHA256:   sha256Hex([]byte(piece.Text)),
				CharStart:    piece.CharStart,
				CharEnd:      piece.CharEnd,
			})
		}
	}

	if err := writeChunksJSONL(chunksPath, chunks); err != nil {
		return BuildResult{}, err
	}

	manifest := Manifest{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		ConfigSHA256: ConfigSHA256(configBytes),
		Domains:      append([]string(nil), cfg.MemoryIndex.Domains...),
		SourceCount:  len(scan.Candidates),
		ChunkCount:   len(chunks),
		SkippedCount: scan.SkippedCount,
		Skipped:      scan.Skipped,
		ManifestPath: manifestPath,
		ChunksPath:   chunksPath,
	}
	sort.Strings(manifest.Domains)

	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BuildResult{}, err
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := os.WriteFile(manifestPath, manifestJSON, 0o644); err != nil {
		return BuildResult{}, fmt.Errorf("write manifest %q: %w", manifestPath, err)
	}

	return BuildResult{
		Manifest: manifest,
		Chunks:   chunks,
	}, nil
}

func chunkText(text string, maxChars int, overlap int) []textChunk {
	if maxChars <= 0 || text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	var chunks []textChunk
	start := 0
	index := 0
	for start < len(runes) {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		piece := string(runes[start:end])
		chunks = append(chunks, textChunk{
			Index:     index,
			CharStart: start,
			CharEnd:   end,
			Text:      piece,
		})
		if end == len(runes) {
			break
		}
		nextStart := end - overlap
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
		index++
	}
	return chunks
}

func chunkID(domain string, sourceSHA string, chunkIndex int) string {
	prefix := sourceSHA
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	return fmt.Sprintf("%s:%s:%d", domain, prefix, chunkIndex)
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func writeChunksJSONL(path string, chunks []Chunk) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open chunks jsonl %q: %w", path, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, chunk := range chunks {
		line, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		if _, err := writer.Write(line); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	return writer.Flush()
}
