package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ScanStatusOK      = "ok"
	ScanStatusFailed  = "failed"
	ScanStatusWarning = "warning"

	findingError   = "error"
	findingWarning = "warning"
)

var suspiciousPatterns = []struct {
	code    string
	pattern string
}{
	{code: "secret_exfiltration", pattern: "curl.*api[_-]?key"},
	{code: "secret_exfiltration", pattern: "wget.*token"},
	{code: "hidden_unicode", pattern: "\u200b"},
}

func ScanSkillDirectory(skillDir string) (ScanReport, ParsedSkill, error) {
	skillDir, err := DiscoverSkillDirectory(skillDir)
	if err != nil {
		return ScanReport{Status: ScanStatusFailed, Findings: []ScanFinding{{
			Severity: findingError,
			Code:     "missing_skill_md",
			Message:  err.Error(),
		}}}, ParsedSkill{}, nil
	}

	skillPath := filepath.Join(skillDir, MaterializedSKILLFilename)
	parsed, err := ParseSKILLFile(skillPath)
	if err != nil {
		return ScanReport{Status: ScanStatusFailed, Findings: []ScanFinding{{
			Severity: findingError,
			Code:     "invalid_skill_md",
			Message:  err.Error(),
			Path:     skillPath,
		}}}, ParsedSkill{}, nil
	}
	parsed.SourcePath = skillDir

	sidecar, err := LoadSidecar(skillDir)
	if err != nil {
		return ScanReport{Status: ScanStatusFailed, Findings: []ScanFinding{{
			Severity: findingError,
			Code:     "invalid_sidecar",
			Message:  err.Error(),
		}}}, parsed, nil
	}
	parsed.Sidecar = sidecar

	findings := make([]ScanFinding, 0)
	files := make([]string, 0)
	scripts := make([]string, 0)

	rootResolved, err := filepath.EvalSymlinks(skillDir)
	if err != nil {
		findings = append(findings, ScanFinding{
			Severity: findingError,
			Code:     "symlink_resolution_failed",
			Message:  err.Error(),
			Path:     skillDir,
		})
	} else {
		rootResolved = filepath.Clean(rootResolved)
	}

	walkErr := filepath.WalkDir(skillDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			findings = append(findings, ScanFinding{
				Severity: findingError,
				Code:     "walk_failed",
				Message:  walkErr.Error(),
				Path:     path,
			})
			return nil
		}
		rel, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.Contains(rel, "..") {
			findings = append(findings, ScanFinding{
				Severity: findingError,
				Code:     "path_traversal",
				Message:  "relative path escapes skill root",
				Path:     rel,
			})
			return nil
		}

		resolved := path
		if linkTarget, err := filepath.EvalSymlinks(path); err == nil {
			resolved = linkTarget
		} else if entry.Type()&os.ModeSymlink != 0 {
			findings = append(findings, ScanFinding{
				Severity: findingError,
				Code:     "symlink_resolution_failed",
				Message:  err.Error(),
				Path:     rel,
			})
			return nil
		}
		if rootResolved != "" {
			cleanResolved := filepath.Clean(resolved)
			if cleanResolved != rootResolved && !strings.HasPrefix(cleanResolved, rootResolved+string(os.PathSeparator)) {
				findings = append(findings, ScanFinding{
					Severity: findingError,
					Code:     "symlink_escape",
					Message:  "symlink points outside skill root",
					Path:     rel,
				})
				return fs.SkipDir
			}
		}

		if entry.IsDir() {
			return nil
		}

		files = append(files, rel)
		if strings.HasPrefix(rel, "scripts"+string(os.PathSeparator)) || strings.HasPrefix(rel, "scripts/") {
			scripts = append(scripts, rel)
		}

		info, err := entry.Info()
		if err == nil && info.Size() > 5*1024*1024 {
			findings = append(findings, ScanFinding{
				Severity: findingWarning,
				Code:     "large_binary",
				Message:  fmt.Sprintf("file exceeds 5MB (%d bytes)", info.Size()),
				Path:     rel,
			})
		}

		if strings.HasSuffix(strings.ToLower(rel), ".md") || rel == MaterializedSKILLFilename {
			data, err := os.ReadFile(path)
			if err != nil {
				findings = append(findings, ScanFinding{
					Severity: findingError,
					Code:     "read_failed",
					Message:  err.Error(),
					Path:     rel,
				})
				return nil
			}
			content := strings.ToLower(string(data))
			for _, pattern := range suspiciousPatterns {
				if strings.Contains(content, strings.ToLower(pattern.pattern)) {
					findings = append(findings, ScanFinding{
						Severity: findingWarning,
						Code:     pattern.code,
						Message:  "suspicious pattern detected in markdown",
						Path:     rel,
					})
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		findings = append(findings, ScanFinding{
			Severity: findingError,
			Code:     "walk_failed",
			Message:  walkErr.Error(),
		})
	}

	parsed.Files = files
	parsed.Scripts = scripts

	status := ScanStatusOK
	for _, finding := range findings {
		if finding.Severity == findingError {
			status = ScanStatusFailed
			break
		}
	}
	if status == ScanStatusOK {
		for _, finding := range findings {
			if finding.Severity == findingWarning {
				status = ScanStatusWarning
				break
			}
		}
	}

	return ScanReport{Status: status, Findings: findings}, parsed, nil
}

func HashSkillDirectory(skillDir string) (string, error) {
	skillDir, err := DiscoverSkillDirectory(skillDir)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	entries, err := collectSortedFiles(skillDir)
	if err != nil {
		return "", err
	}
	for _, rel := range entries {
		path := filepath.Join(skillDir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		_, _ = hasher.Write([]byte(rel))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write(data)
		_, _ = hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func collectSortedFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
