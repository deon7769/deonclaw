package git

import (
	"context"
	"testing"
)

func TestParseStatusEmpty(t *testing.T) {
	snap := ParseStatus([]byte(""))
	if !snap.IsClean() {
		t.Fatal("empty output should be clean")
	}
	if len(snap.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(snap.Entries))
	}
}

func TestParseStatusSingleModified(t *testing.T) {
	snap := ParseStatus([]byte(" M internal/tasks/task.go\n"))
	if snap.IsClean() {
		t.Fatal("modified file should not be clean")
	}
	if len(snap.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snap.Entries))
	}
	entry := snap.Entries[0]
	if entry.Path != "internal/tasks/task.go" {
		t.Fatalf("path = %q, want internal/tasks/task.go", entry.Path)
	}
	if entry.Staged != StatusUnmodified {
		t.Fatalf("staged = %q, want space (unmodified)", entry.Staged)
	}
	if entry.Unstaged != StatusModified {
		t.Fatalf("unstaged = %q, want M", entry.Unstaged)
	}
}

func TestParseStatusMultipleEntries(t *testing.T) {
	input := "M  staged.go\n" +
		" M modified.go\n" +
		"?? untracked.go\n" +
		"D  deleted.go\n" +
		" R renamed.go\n"
	snap := ParseStatus([]byte(input))
	if len(snap.Entries) != 5 {
		t.Fatalf("entries = %d, want 5", len(snap.Entries))
	}

	want := []struct {
		path     string
		staged   StatusCode
		unstaged StatusCode
	}{
		{"staged.go", StatusModified, StatusUnmodified},
		{"modified.go", StatusUnmodified, StatusModified},
		{"untracked.go", StatusUntracked, StatusUntracked},
		{"deleted.go", StatusDeleted, StatusUnmodified},
		{"renamed.go", StatusUnmodified, StatusRenamed},
	}
	for i, w := range want {
		entry := snap.Entries[i]
		if entry.Path != w.path {
			t.Errorf("entry[%d].Path = %q, want %q", i, entry.Path, w.path)
		}
		if entry.Staged != w.staged {
			t.Errorf("entry[%d].Staged = %q, want %q", i, entry.Staged, w.staged)
		}
		if entry.Unstaged != w.unstaged {
			t.Errorf("entry[%d].Unstaged = %q, want %q", i, entry.Unstaged, w.unstaged)
		}
	}
}

func TestParseStatusSkipsShortLines(t *testing.T) {
	input := "\nab\n?? valid.go\n"
	snap := ParseStatus([]byte(input))
	if len(snap.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snap.Entries))
	}
	if snap.Entries[0].Path != "valid.go" {
		t.Fatalf("path = %q, want valid.go", snap.Entries[0].Path)
	}
}

func TestFileEntryIsUntracked(t *testing.T) {
	tests := []struct {
		entry  FileEntry
		result bool
	}{
		{FileEntry{Path: "a.go", Staged: StatusUntracked, Unstaged: StatusUntracked}, true},
		{FileEntry{Path: "b.go", Staged: StatusModified, Unstaged: StatusUntracked}, false},
		{FileEntry{Path: "c.go", Staged: StatusUntracked, Unstaged: StatusModified}, false},
		{FileEntry{Path: "d.go", Staged: StatusAdded, Unstaged: StatusUnmodified}, false},
	}
	for _, tt := range tests {
		if got := tt.entry.IsUntracked(); got != tt.result {
			t.Errorf("%+v.IsUntracked() = %v, want %v", tt.entry, got, tt.result)
		}
	}
}

func TestFileEntryIsDeleted(t *testing.T) {
	tests := []struct {
		entry  FileEntry
		result bool
	}{
		{FileEntry{Path: "a.go", Staged: StatusDeleted, Unstaged: StatusUnmodified}, true},
		{FileEntry{Path: "b.go", Staged: StatusUnmodified, Unstaged: StatusDeleted}, true},
		{FileEntry{Path: "c.go", Staged: StatusModified, Unstaged: StatusUnmodified}, false},
	}
	for _, tt := range tests {
		if got := tt.entry.IsDeleted(); got != tt.result {
			t.Errorf("%+v.IsDeleted() = %v, want %v", tt.entry, got, tt.result)
		}
	}
}

func TestFileEntryIsStaged(t *testing.T) {
	tests := []struct {
		entry  FileEntry
		result bool
	}{
		{FileEntry{Path: "a.go", Staged: StatusAdded, Unstaged: StatusUnmodified}, true},
		{FileEntry{Path: "b.go", Staged: StatusModified, Unstaged: StatusUnmodified}, true},
		{FileEntry{Path: "c.go", Staged: StatusUntracked, Unstaged: StatusUntracked}, false},
		{FileEntry{Path: "d.go", Staged: StatusUnmodified, Unstaged: StatusModified}, false},
		{FileEntry{Path: "e.go", Staged: StatusIgnored, Unstaged: StatusUnmodified}, false},
	}
	for _, tt := range tests {
		if got := tt.entry.IsStaged(); got != tt.result {
			t.Errorf("%+v.IsStaged() = %v, want %v", tt.entry, got, tt.result)
		}
	}
}

func TestSnapshotPaths(t *testing.T) {
	snap := &Snapshot{
		Entries: []FileEntry{
			{Path: "a.go"},
			{Path: "b.go"},
		},
	}
	paths := snap.Paths()
	if len(paths) != 2 || paths[0] != "a.go" || paths[1] != "b.go" {
		t.Fatalf("Paths() = %v, want [a.go b.go]", paths)
	}
}

func TestDiffChangedPathsBothNil(t *testing.T) {
	d := Diff{}
	if paths := d.ChangedPaths(); paths != nil {
		t.Fatalf("ChangedPaths() = %v, want nil", paths)
	}
}

func TestDiffChangedPathsBeforeNil(t *testing.T) {
	d := Diff{
		After: &Snapshot{
			Entries: []FileEntry{
				{Path: "new.go"},
			},
		},
	}
	paths := d.ChangedPaths()
	if len(paths) != 1 || paths[0] != "new.go" {
		t.Fatalf("ChangedPaths() = %v, want [new.go]", paths)
	}
}

func TestDiffChangedPathsAfterNil(t *testing.T) {
	d := Diff{
		Before: &Snapshot{
			Entries: []FileEntry{
				{Path: "old.go"},
			},
		},
	}
	if paths := d.ChangedPaths(); paths != nil {
		t.Fatalf("ChangedPaths() = %v, want nil", paths)
	}
}

func TestDiffChangedPathsNewFilesOnly(t *testing.T) {
	d := Diff{
		Before: &Snapshot{
			Entries: []FileEntry{
				{Path: "existing.go"},
			},
		},
		After: &Snapshot{
			Entries: []FileEntry{
				{Path: "existing.go"},
				{Path: "new.go"},
				{Path: "also-new.go"},
			},
		},
	}
	paths := d.ChangedPaths()
	if len(paths) != 2 {
		t.Fatalf("ChangedPaths() = %v, want 2 paths", paths)
	}
	if paths[0] != "new.go" || paths[1] != "also-new.go" {
		t.Fatalf("ChangedPaths() = %v, want [new.go also-new.go]", paths)
	}
}

func TestDiffChangedPathsNoChanges(t *testing.T) {
	d := Diff{
		Before: &Snapshot{
			Entries: []FileEntry{
				{Path: "a.go"},
			},
		},
		After: &Snapshot{
			Entries: []FileEntry{
				{Path: "a.go"},
			},
		},
	}
	if paths := d.ChangedPaths(); len(paths) != 0 {
		t.Fatalf("ChangedPaths() = %v, want empty", paths)
	}
}

func TestDiffChangedPathsDeduplicates(t *testing.T) {
	d := Diff{
		After: &Snapshot{
			Entries: []FileEntry{
				{Path: "dup.go", Staged: StatusAdded},
				{Path: "dup.go", Unstaged: StatusModified},
			},
		},
	}
	paths := d.ChangedPaths()
	if len(paths) != 1 {
		t.Fatalf("ChangedPaths() = %v, want 1 path", paths)
	}
	if paths[0] != "dup.go" {
		t.Fatalf("ChangedPaths()[0] = %q, want dup.go", paths[0])
	}
}

func TestSnapshotWithCallsRunner(t *testing.T) {
	called := false
	runner := func(ctx context.Context, workspace string) ([]byte, error) {
		called = true
		if workspace != "." {
			t.Fatalf("workspace = %q, want .", workspace)
		}
		return []byte(" M test.go\n"), nil
	}
	snap, err := SnapshotWith(nil, "", runner)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("runner was not called")
	}
	if len(snap.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(snap.Entries))
	}
}

func TestSnapshotWithTrimsWorkspace(t *testing.T) {
	runner := func(ctx context.Context, workspace string) ([]byte, error) {
		if workspace != "/some/path" {
			t.Fatalf("workspace = %q, want /some/path", workspace)
		}
		return []byte(""), nil
	}
	_, err := SnapshotWith(nil, "  /some/path  ", runner)
	if err != nil {
		t.Fatal(err)
	}
}
