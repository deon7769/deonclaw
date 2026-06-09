package tasks

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func Parse(data []byte) (*Task, error) {
	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("parse task yaml: %w", err)
	}
	normalize(&task)
	return &task, nil
}

func LoadFromFile(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file %q: %w", path, err)
	}
	return Parse(data)
}

func normalize(task *Task) {
	task.ID = strings.TrimSpace(task.ID)
	task.Title = strings.TrimSpace(task.Title)
	task.Domain = strings.TrimSpace(task.Domain)
	task.Worker = strings.TrimSpace(task.Worker)
	task.ModelProfile = strings.TrimSpace(task.ModelProfile)
	if task.ModelStrategy != nil {
		task.ModelStrategy.Preferred = normalizeStringList(task.ModelStrategy.Preferred)
		task.ModelStrategy.Fallback = normalizeStringList(task.ModelStrategy.Fallback)
		task.ModelStrategy.RequireTags = normalizeStringList(task.ModelStrategy.RequireTags)
		if task.ModelStrategy.FallbackPolicy != nil {
			task.ModelStrategy.FallbackPolicy.RetryOn = normalizeStringList(task.ModelStrategy.FallbackPolicy.RetryOn)
			task.ModelStrategy.FallbackPolicy.NeverRetryOn = normalizeStringList(task.ModelStrategy.FallbackPolicy.NeverRetryOn)
		}
	}
	task.Goal = strings.TrimSpace(task.Goal)
	task.Mode = strings.TrimSpace(task.Mode)
	task.Workspace.Strategy = strings.TrimSpace(task.Workspace.Strategy)
	task.Workspace.Path = strings.TrimSpace(task.Workspace.Path)
	task.Memory.Scope = strings.TrimSpace(task.Memory.Scope)
	for i := range task.Validation.Commands {
		task.Validation.Commands[i].Name = strings.TrimSpace(task.Validation.Commands[i].Name)
		task.Validation.Commands[i].Command = strings.TrimSpace(task.Validation.Commands[i].Command)
	}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
