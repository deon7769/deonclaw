package runner

import (
	"bytes"

	"github.com/deon7769/deonclaw/internal/git"
)

type ChangedFile struct {
	Path     string `json:"path"`
	Staged   string `json:"staged"`
	Unstaged string `json:"unstaged"`
	Source   string `json:"source"`
}

func changedFilesFromSnapshot(before *git.Snapshot, after *git.Snapshot) []ChangedFile {
	if after == nil {
		return nil
	}

	beforeSet := make(map[string]struct{})
	if before != nil {
		for _, entry := range before.Entries {
			beforeSet[entry.Path] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	changedFiles := make([]ChangedFile, 0, len(after.Entries))
	for _, entry := range after.Entries {
		if _, inBefore := beforeSet[entry.Path]; inBefore {
			continue
		}
		if _, already := seen[entry.Path]; already {
			continue
		}
		seen[entry.Path] = struct{}{}
		changedFiles = append(changedFiles, ChangedFile{
			Path:     entry.Path,
			Staged:   string(entry.Staged),
			Unstaged: string(entry.Unstaged),
			Source:   "snapshot",
		})
	}
	return changedFiles
}

func appendUntrackedMetadata(diffPatch []byte, changedFiles []ChangedFile) []byte {
	var untracked []ChangedFile
	for _, changed := range changedFiles {
		if changed.Staged == string(git.StatusUntracked) && changed.Unstaged == string(git.StatusUntracked) {
			untracked = append(untracked, changed)
		}
	}
	if len(untracked) == 0 {
		return diffPatch
	}

	var output bytes.Buffer
	output.Write(diffPatch)
	if len(diffPatch) > 0 && !bytes.HasSuffix(diffPatch, []byte("\n")) {
		output.WriteByte('\n')
	}
	if len(diffPatch) > 0 {
		output.WriteByte('\n')
	}
	output.WriteString("# Untracked files from snapshot\n")
	for _, changed := range untracked {
		output.WriteString("# path: ")
		output.WriteString(changed.Path)
		output.WriteString(" staged: ")
		output.WriteString(changed.Staged)
		output.WriteString(" unstaged: ")
		output.WriteString(changed.Unstaged)
		output.WriteString(" source: ")
		output.WriteString(changed.Source)
		output.WriteByte('\n')
	}
	return output.Bytes()
}
