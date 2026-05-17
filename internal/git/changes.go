package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type StatusCode string

const (
	StatusUnmodified StatusCode = " "
	StatusModified   StatusCode = "M"
	StatusAdded      StatusCode = "A"
	StatusDeleted    StatusCode = "D"
	StatusRenamed    StatusCode = "R"
	StatusCopied     StatusCode = "C"
	StatusUnmerged   StatusCode = "U"
	StatusUntracked  StatusCode = "?"
	StatusIgnored    StatusCode = "!"
	StatusTypeChange StatusCode = "T"
)

type FileEntry struct {
	Path     string
	Staged   StatusCode
	Unstaged StatusCode
}

func (f FileEntry) IsUntracked() bool {
	return f.Staged == StatusUntracked && f.Unstaged == StatusUntracked
}

func (f FileEntry) IsDeleted() bool {
	return f.Staged == StatusDeleted || f.Unstaged == StatusDeleted
}

func (f FileEntry) IsModified() bool {
	return f.Staged == StatusModified || f.Unstaged == StatusModified
}

func (f FileEntry) IsStaged() bool {
	return f.Staged != StatusUnmodified && f.Staged != StatusUntracked && f.Staged != StatusIgnored
}

type Snapshot struct {
	Entries []FileEntry
}

func (s *Snapshot) IsClean() bool {
	return len(s.Entries) == 0
}

func (s *Snapshot) Paths() []string {
	paths := make([]string, 0, len(s.Entries))
	for _, entry := range s.Entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

type Diff struct {
	Before *Snapshot
	After  *Snapshot
}

func (d Diff) ChangedPaths() []string {
	if d.After == nil {
		return nil
	}

	beforeSet := make(map[string]struct{})
	if d.Before != nil {
		for _, entry := range d.Before.Entries {
			beforeSet[entry.Path] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	var paths []string
	for _, entry := range d.After.Entries {
		if _, inBefore := beforeSet[entry.Path]; inBefore {
			continue
		}
		if _, already := seen[entry.Path]; already {
			continue
		}
		seen[entry.Path] = struct{}{}
		paths = append(paths, entry.Path)
	}
	return paths
}

type GitError struct {
	Command string
	Stderr  string
	Err     error
}

func (e *GitError) Error() string {
	msg := fmt.Sprintf("git %s: %v", e.Command, e.Err)
	if e.Stderr != "" {
		msg += ": " + strings.TrimSpace(e.Stderr)
	}
	return msg
}

func (e *GitError) Unwrap() error {
	return e.Err
}

type SnapshotRunner func(ctx context.Context, workspace string) ([]byte, error)

func TakeSnapshot(ctx context.Context, workspace string) (*Snapshot, error) {
	return SnapshotWith(ctx, workspace, runGitStatus)
}

func SnapshotWith(ctx context.Context, workspace string, runner SnapshotRunner) (*Snapshot, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = "."
	}

	output, err := runner(ctx, workspace)
	if err != nil {
		return nil, err
	}
	return ParseStatus(output), nil
}

func runGitStatus(ctx context.Context, workspace string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "status", "--porcelain")
	var stderr strings.Builder
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, &GitError{
			Command: "status",
			Stderr:  stderr.String(),
			Err:     err,
		}
	}
	return output, nil
}

func ParseStatus(output []byte) *Snapshot {
	var entries []FileEntry
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		staged := StatusCode(line[0:1])
		unstaged := StatusCode(line[1:2])
		filePath := line[3:]
		if filePath == "" {
			continue
		}
		entries = append(entries, FileEntry{
			Path:     filePath,
			Staged:   staged,
			Unstaged: unstaged,
		})
	}
	return &Snapshot{Entries: entries}
}
