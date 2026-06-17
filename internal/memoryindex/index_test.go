package memoryindex_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/memoryindex"
)

func TestValidateAcceptsValidConfig(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := memoryindex.Validate(loaded); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsMissingRoot(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.MemoryIndex.Sources[0].Root = filepath.Join(t.TempDir(), "missing-root")
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want missing root failure")
	}
}

func TestValidateRejectsOverlapGTEMaxChars(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Chunking.OverlapChars = cfg.MemoryIndex.Chunking.MaxChars
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want overlap failure")
	}
}

func TestValidateRejectsSecretsRoot(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Sources[0].Root = filepath.Join(root, "memory", "secrets")
	if err := os.MkdirAll(cfg.MemoryIndex.Sources[0].Root, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want secrets root failure")
	}
}

func TestPlanCountsFilesPerDomain(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "notes", "a.md"), "# A\n")
	writeFile(t, filepath.Join(root, "memory", "escalasoft_brain", "cases", "b.md"), "# B\n")

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.SourceCount != 2 {
		t.Fatalf("source_count = %d, want 2", plan.SourceCount)
	}
	counts := map[string]int{}
	for _, domain := range plan.Domains {
		counts[domain.Domain] = domain.FileCount
	}
	if counts["mysecondbrain"] != 1 || counts["escalasoft_brain"] != 1 {
		t.Fatalf("domain counts = %#v, want 1 each", counts)
	}
}

func TestBuildGeneratesManifestAndChunks(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	content := strings.Repeat("memory ", 500)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), content)

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	artifactsDir := filepath.Join(root, "artifacts")
	result, err := memoryindex.Build(loaded, artifactsDir, configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Manifest.SourceCount != 1 || result.Manifest.ChunkCount == 0 {
		t.Fatalf("manifest = %#v, want indexed source and chunks", result.Manifest)
	}
	if _, err := os.Stat(result.Manifest.ManifestPath); err != nil {
		t.Fatalf("manifest path missing: %v", err)
	}
	if _, err := os.Stat(result.Manifest.ChunksPath); err != nil {
		t.Fatalf("chunks path missing: %v", err)
	}
	if result.Manifest.ConfigSHA256 == "" {
		t.Fatal("config_sha256 is empty")
	}

	chunks := readChunksJSONL(t, result.Manifest.ChunksPath)
	if len(chunks) == 0 {
		t.Fatal("chunks jsonl is empty")
	}
	for _, chunk := range chunks {
		if chunk.SourceSHA256 == "" || chunk.TextSHA256 == "" {
			t.Fatalf("chunk = %#v, want sha256 fields", chunk)
		}
	}
}

func TestExcludeSkipsArchiveFiles(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "live.md"), "# live\n")
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", ".archive", "old.md"), "# old\n")

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1", plan.SourceCount)
	}
}

func TestSecretsPathBlocked(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "live.md"), "# live\n")
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "secrets", "token.md"), "# secret\n")

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1 without secrets", plan.SourceCount)
	}
	if plan.Skipped.SecretPaths == 0 {
		t.Fatalf("skipped_secret_paths = %d, want > 0", plan.Skipped.SecretPaths)
	}
}

func TestPlanSkipsSymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	realPath := filepath.Join(root, "memory", "mysecondbrain", "real.md")
	writeFile(t, realPath, "# real\n")
	linkPath := filepath.Join(root, "memory", "mysecondbrain", "link.md")
	symlinkOrSkip(t, realPath, linkPath)

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1 without symlink", plan.SourceCount)
	}
	if plan.Skipped.Symlinks != 1 {
		t.Fatalf("skipped_symlinks = %d, want 1", plan.Skipped.Symlinks)
	}
	for _, domain := range plan.Domains {
		for _, path := range domain.Files {
			if path == linkPath {
				t.Fatalf("indexed symlink %q", linkPath)
			}
		}
	}
}

func TestPlanSkipsSymlinkPointingOutsideRoot(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	outside := filepath.Join(root, "outside.md")
	writeFile(t, outside, "# outside\n")
	linkPath := filepath.Join(root, "memory", "mysecondbrain", "escape.md")
	symlinkOrSkip(t, outside, linkPath)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "inside.md"), "# inside\n")

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1", plan.SourceCount)
	}
	if plan.Skipped.Symlinks != 1 {
		t.Fatalf("skipped_symlinks = %d, want 1", plan.Skipped.Symlinks)
	}
	for _, domain := range plan.Domains {
		for _, path := range domain.Files {
			if path == linkPath || path == outside {
				t.Fatalf("indexed symlink or outside target: %q", path)
			}
		}
	}
}

func TestValidateRejectsAbsoluteOutputPath(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Output.Manifest = "/tmp/memory-index-manifest.json"
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want absolute output failure")
	}
}

func TestValidateRejectsOutputWithParentTraversal(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Output.Chunks = "../escape-chunks.jsonl"
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want parent traversal failure")
	}
}

func TestValidateRejectsSecretsOutputPath(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Output.Manifest = "secrets/memory-index-manifest.json"
	if err := memoryindex.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want secrets output failure")
	}
}

func TestBuildWritesOutputsInsideArtifactsDir(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfg.MemoryIndex.Output.Manifest = "indexes/manifest.json"
	cfg.MemoryIndex.Output.Chunks = "indexes/chunks.jsonl"
	cfgPath := writeTestIndexConfigWithOutput(t, root, cfg)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	artifactsDir := filepath.Join(root, "artifacts")
	result, err := memoryindex.Build(loaded, artifactsDir, configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	wantManifest := filepath.Join(artifactsDir, "indexes", "manifest.json")
	wantChunks := filepath.Join(artifactsDir, "indexes", "chunks.jsonl")
	if result.Manifest.ManifestPath != wantManifest {
		t.Fatalf("manifest_path = %q, want %q", result.Manifest.ManifestPath, wantManifest)
	}
	if result.Manifest.ChunksPath != wantChunks {
		t.Fatalf("chunks_path = %q, want %q", result.Manifest.ChunksPath, wantChunks)
	}
}

func TestFileOutsideRootNotIndexed(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	outside := filepath.Join(root, "outside.md")
	writeFile(t, outside, "# outside\n")
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "inside.md"), "# inside\n")

	plan, err := memoryindex.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	for _, domain := range plan.Domains {
		for _, path := range domain.Files {
			if path == outside {
				t.Fatalf("indexed outside file %q", outside)
			}
		}
	}
	if plan.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1", plan.SourceCount)
	}
}

func TestBuildDoesNotModifySourceFiles(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	sourcePath := filepath.Join(root, "memory", "mysecondbrain", "note.md")
	original := "# canonical\n"
	writeFile(t, sourcePath, original)

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if _, err := memoryindex.Build(loaded, filepath.Join(root, "artifacts"), configBytes); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	got := readFileString(t, sourcePath)
	if got != original {
		t.Fatalf("source modified: got %q, want %q", got, original)
	}
}

func writeTestIndexLayout(t *testing.T, root string) memoryindex.Config {
	t.Helper()
	mysecondbrainRoot := filepath.Join(root, "memory", "mysecondbrain")
	escalasoftRoot := filepath.Join(root, "memory", "escalasoft_brain")
	for _, dir := range []string{mysecondbrainRoot, escalasoftRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
	}
	return memoryindex.Config{
		MemoryIndex: memoryindex.MemoryIndexConfig{
			Domains: []string{"mysecondbrain", "escalasoft_brain"},
			Sources: []memoryindex.Source{
				{
					Domain:  "mysecondbrain",
					Root:    mysecondbrainRoot,
					Include: []string{"**/*.md"},
					Exclude: []string{"**/.archive/**", "**/secrets/**"},
				},
				{
					Domain:  "escalasoft_brain",
					Root:    escalasoftRoot,
					Include: []string{"**/*.md"},
					Exclude: []string{"**/.archive/**", "**/secrets/**"},
				},
			},
			Chunking: memoryindex.ChunkingConfig{
				MaxChars:     2000,
				OverlapChars: 200,
			},
			Output: memoryindex.OutputConfig{
				Manifest: "memory-index-manifest.json",
				Chunks:   "memory-index-chunks.jsonl",
			},
		},
	}
}

func testConfig(root string) memoryindex.Config {
	mysecondbrainRoot := filepath.Join(root, "memory", "mysecondbrain")
	_ = os.MkdirAll(mysecondbrainRoot, 0o755)
	return memoryindex.Config{
		MemoryIndex: memoryindex.MemoryIndexConfig{
			Domains: []string{"mysecondbrain"},
			Sources: []memoryindex.Source{
				{
					Domain:  "mysecondbrain",
					Root:    mysecondbrainRoot,
					Include: []string{"**/*.md"},
					Exclude: []string{"**/.archive/**", "**/secrets/**"},
				},
			},
			Chunking: memoryindex.ChunkingConfig{
				MaxChars:     2000,
				OverlapChars: 200,
			},
			Output: memoryindex.OutputConfig{
				Manifest: "memory-index-manifest.json",
				Chunks:   "memory-index-chunks.jsonl",
			},
		},
	}
}

func writeTestIndexConfig(t *testing.T, root string, cfg memoryindex.Config) string {
	t.Helper()
	return writeTestIndexConfigWithOutput(t, root, cfg)
}

func writeTestIndexConfigWithOutput(t *testing.T, root string, cfg memoryindex.Config) string {
	t.Helper()
	path := filepath.Join(root, "memory-index.yaml")
	data := []byte(`memory_index:
  domains:
    - mysecondbrain
    - escalasoft_brain
  sources:
    - domain: mysecondbrain
      root: ` + yamlString(cfg.MemoryIndex.Sources[0].Root) + `
      include:
        - "**/*.md"
      exclude:
        - "**/.archive/**"
        - "**/secrets/**"
    - domain: escalasoft_brain
      root: ` + yamlString(cfg.MemoryIndex.Sources[1].Root) + `
      include:
        - "**/*.md"
      exclude:
        - "**/.archive/**"
        - "**/secrets/**"
  chunking:
    max_chars: 2000
    overlap_chars: 200
  output:
    manifest: ` + yamlString(cfg.MemoryIndex.Output.Manifest) + `
    chunks: ` + yamlString(cfg.MemoryIndex.Output.Chunks) + `
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
}

func yamlString(value string) string {
	return `"` + strings.ReplaceAll(value, `\`, `\\`) + `"`
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}

func readChunksJSONL(t *testing.T, path string) []memoryindex.Chunk {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	var chunks []memoryindex.Chunk
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var chunk memoryindex.Chunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error = %v", err)
	}
	return chunks
}
