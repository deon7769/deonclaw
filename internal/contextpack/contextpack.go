package contextpack

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/deon7769/deonclaw/internal/domains"
	"github.com/deon7769/deonclaw/internal/tasks"
)

type ContextPack struct {
	Task     tasks.Task
	Domain   domains.DomainConfig
	Sources  []ContextSource
	Warnings []string
}

type ContextSource struct {
	Kind    string
	Path    string
	Domain  string
	Content string
}

type Builder struct{}

type BuildOptions struct {
	TaskPath    string
	DomainsPath string
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
		Sources: []ContextSource{
			{Kind: "task", Path: opts.TaskPath, Domain: task.Domain},
			{Kind: "domains_config", Path: opts.DomainsPath, Domain: task.Domain},
		},
	}

	if domain.IsIsolated() {
		for _, bridgePath := range domain.BridgeFiles {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			source := ContextSource{
				Kind:   "bridge_file",
				Path:   bridgePath,
				Domain: domain.Name,
			}
			data, err := os.ReadFile(bridgePath)
			if err != nil {
				if os.IsNotExist(err) {
					pack.Warnings = append(pack.Warnings, fmt.Sprintf("bridge file not found: %s", bridgePath))
					pack.Sources = append(pack.Sources, source)
					continue
				}
				return nil, fmt.Errorf("read bridge file %q: %w", bridgePath, err)
			}
			source.Content = string(data)
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
