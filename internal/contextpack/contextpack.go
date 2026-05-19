package contextpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/deon7769/deonclaw/internal/domains"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const defaultMaxSourceBytes = 262144

type ContextPack struct {
	Task     tasks.Task
	Domain   domains.DomainConfig
	Sources  []ContextSource
	Warnings []string
}

type ContextSource struct {
	Kind      string
	Path      string
	Domain    string
	Exists    bool
	SizeBytes int64
	SHA256    string
	Truncated bool
	Content   string
}

type Builder struct{}

type BuildOptions struct {
	TaskPath       string
	DomainsPath    string
	MaxSourceBytes int64
}

func (b Builder) Build(ctx context.Context, opts BuildOptions) (*ContextPack, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.TaskPath) == "" {
		return nil, fmt.Errorf("task path is required")
	}
	if strings.TrimSpace(opts.DomainsPath) == "" {
		return nil, fmt.Errorf("domains path is required")
	}
	maxSourceBytes := opts.MaxSourceBytes
	if maxSourceBytes <= 0 {
		maxSourceBytes = defaultMaxSourceBytes
	}

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		return nil, err
	}
	if err := tasks.Validate(task); err != nil {
		return nil, fmt.Errorf("validate task: %w", err)
	}

	domainsConfig, err := domains.LoadFromFile(opts.DomainsPath)
	if err != nil {
		return nil, err
	}
	if err := domains.Validate(domainsConfig); err != nil {
		return nil, fmt.Errorf("validate domains config: %w", err)
	}

	domain, ok := domainsConfig.Domains[task.Domain]
	if !ok {
		return nil, fmt.Errorf("task domain %q not found in domains config", task.Domain)
	}

	pack := &ContextPack{
		Task:   *task,
		Domain: domain,
	}

	taskSource, err := readContextSource(opts.TaskPath, "task", task.Domain, maxSourceBytes, false)
	if err != nil {
		return nil, err
	}
	domainsSource, err := readContextSource(opts.DomainsPath, "domains_config", task.Domain, maxSourceBytes, false)
	if err != nil {
		return nil, err
	}
	pack.Sources = []ContextSource{taskSource, domainsSource}

	if domain.IsIsolated() {
		for _, bridgePath := range domain.BridgeFiles {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			source, err := readContextSource(bridgePath, "bridge_file", domain.Name, maxSourceBytes, true)
			if err != nil {
				return nil, err
			}
			if !source.Exists {
				pack.Warnings = append(pack.Warnings, fmt.Sprintf("bridge file not found: %s", bridgePath))
			}
			if source.Truncated {
				pack.Warnings = append(pack.Warnings, fmt.Sprintf("bridge file %s truncated to %d bytes from %d bytes", bridgePath, maxSourceBytes, source.SizeBytes))
			}
			pack.Sources = append(pack.Sources, source)
		}
	}

	return pack, nil
}

func (p *ContextPack) Markdown() []byte {
	if p == nil {
		return nil
	}

	var output strings.Builder
	output.WriteString("# DeonClaw Context Pack\n\n")

	output.WriteString("## task\n")
	writeField(&output, "id", p.Task.ID)
	writeField(&output, "title", p.Task.Title)
	writeField(&output, "domain", p.Task.Domain)
	writeField(&output, "goal", p.Task.Goal)
	writeField(&output, "mode", p.Task.Mode)
	output.WriteByte('\n')

	output.WriteString("## domain\n")
	writeField(&output, "name", p.Domain.Name)
	writeField(&output, "type", p.Domain.Type)
	writeField(&output, "root", p.Domain.Root)
	writeField(&output, "default", strconv.FormatBool(p.Domain.Default))
	writeField(&output, "isolated", strconv.FormatBool(p.Domain.IsIsolated()))
	if p.Domain.DefaultAgent != "" {
		writeField(&output, "default_agent", p.Domain.DefaultAgent)
	}
	if len(p.Domain.StructuredData) > 0 {
		output.WriteString("structured_data:\n")
		for _, key := range sortedMapKeys(p.Domain.StructuredData) {
			output.WriteString("- ")
			output.WriteString(key)
			output.WriteString(": ")
			output.WriteString(p.Domain.StructuredData[key])
			output.WriteByte('\n')
		}
	}
	if len(p.Domain.Staging) > 0 {
		writeList(&output, "staging", p.Domain.Staging)
	}
	output.WriteByte('\n')

	writeList(&output, "allowed_paths", p.Task.AllowedPaths)
	writeList(&output, "forbidden_paths", p.Task.ForbiddenPaths)

	output.WriteString("## validation commands\n")
	if len(p.Task.Validation.Commands) == 0 {
		output.WriteString("- none\n")
	} else {
		for _, command := range p.Task.Validation.Commands {
			output.WriteString("- name: ")
			output.WriteString(command.Name)
			output.WriteByte('\n')
			output.WriteString("  command: ")
			output.WriteString(command.Command)
			output.WriteByte('\n')
			if len(command.Args) > 0 {
				output.WriteString("  args: ")
				output.WriteString(strings.Join(command.Args, " "))
				output.WriteByte('\n')
			}
			if command.TimeoutSeconds > 0 {
				output.WriteString("  timeout_seconds: ")
				output.WriteString(strconv.Itoa(command.TimeoutSeconds))
				output.WriteByte('\n')
			}
		}
	}
	output.WriteByte('\n')

	output.WriteString("## context sources\n")
	for _, source := range p.Sources {
		output.WriteString("- kind: ")
		output.WriteString(source.Kind)
		output.WriteByte('\n')
		output.WriteString("  path: ")
		output.WriteString(source.Path)
		output.WriteByte('\n')
		output.WriteString("  exists: ")
		output.WriteString(strconv.FormatBool(source.Exists))
		output.WriteByte('\n')
		output.WriteString("  size_bytes: ")
		output.WriteString(strconv.FormatInt(source.SizeBytes, 10))
		output.WriteByte('\n')
		output.WriteString("  sha256: ")
		output.WriteString(source.SHA256)
		output.WriteByte('\n')
		output.WriteString("  truncated: ")
		output.WriteString(strconv.FormatBool(source.Truncated))
		output.WriteByte('\n')
		if source.Domain != "" {
			output.WriteString("  domain: ")
			output.WriteString(source.Domain)
			output.WriteByte('\n')
		}
		if source.Content != "" {
			output.WriteString("\n```markdown\n")
			output.WriteString(source.Content)
			if !strings.HasSuffix(source.Content, "\n") {
				output.WriteByte('\n')
			}
			output.WriteString("```\n")
		}
	}
	output.WriteByte('\n')

	output.WriteString("## Warnings\n")
	if len(p.Warnings) == 0 {
		output.WriteString("- none\n")
	} else {
		for _, warning := range p.Warnings {
			output.WriteString("- ")
			output.WriteString(warning)
			output.WriteByte('\n')
		}
	}

	return []byte(output.String())
}

func readContextSource(path string, kind string, domain string, maxBytes int64, includeContent bool) (ContextSource, error) {
	source := ContextSource{
		Kind:   kind,
		Path:   path,
		Domain: domain,
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return source, nil
		}
		return source, fmt.Errorf("read context source %q: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return source, fmt.Errorf("inspect context source %q: %w", path, err)
	}
	source.Exists = true
	source.SizeBytes = info.Size()

	hasher := sha256.New()
	content := make([]byte, 0, int(minInt64(source.SizeBytes, maxBytes)))
	remaining := maxBytes
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			if _, err := hasher.Write(chunk); err != nil {
				return source, fmt.Errorf("hash context source %q: %w", path, err)
			}
			if includeContent && remaining > 0 {
				take := n
				if int64(take) > remaining {
					take = int(remaining)
				}
				content = append(content, chunk[:take]...)
				remaining -= int64(take)
			}
		}
		if readErr == nil {
			continue
		}
		if readErr == io.EOF {
			break
		}
		return source, fmt.Errorf("read context source %q: %w", path, readErr)
	}

	source.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	if includeContent {
		source.Content = string(content)
		source.Truncated = source.SizeBytes > int64(len(content))
	}
	return source, nil
}

func writeField(output *strings.Builder, key string, value string) {
	output.WriteString(key)
	output.WriteString(": ")
	output.WriteString(value)
	output.WriteByte('\n')
}

func writeList(output *strings.Builder, title string, values []string) {
	output.WriteString("## ")
	output.WriteString(title)
	output.WriteByte('\n')
	if len(values) == 0 {
		output.WriteString("- none\n\n")
		return
	}
	for _, value := range values {
		output.WriteString("- ")
		output.WriteString(value)
		output.WriteByte('\n')
	}
	output.WriteByte('\n')
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
