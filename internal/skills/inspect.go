package skills

import (
	"fmt"
	"strings"
)

func InspectLocal(path string) (InspectReport, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return InspectReport{}, fmt.Errorf("skill path is required")
	}
	scan, skill, err := ScanSkillDirectory(path)
	if err != nil {
		return InspectReport{}, err
	}
	contentHash := ""
	if scan.Status != ScanStatusFailed {
		contentHash, err = HashSkillDirectory(path)
		if err != nil {
			return InspectReport{}, err
		}
	}
	ready := scan.Status == ScanStatusOK || scan.Status == ScanStatusWarning
	return InspectReport{
		SourceType:    SourceTypeLocal,
		SourceRef:     path,
		Skill:         skill,
		Scan:          scan,
		ContentSHA256: contentHash,
		ReadyToStage:  ready,
	}, nil
}

func InspectSourceRef(ref string) (InspectReport, error) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "git:") {
		return inspectGitRef(ref)
	}
	return InspectLocal(ref)
}

func inspectGitRef(ref string) (InspectReport, error) {
	remainder := strings.TrimPrefix(ref, "git:")
	if remainder == "" {
		return InspectReport{}, fmt.Errorf("git source reference is required")
	}
	at := strings.LastIndex(remainder, "@")
	if at <= 0 {
		return InspectReport{}, fmt.Errorf("git source %q must use git:owner/repo@ref form", ref)
	}
	repository := strings.TrimSpace(remainder[:at])
	gitRef := strings.TrimSpace(remainder[at+1:])
	if repository == "" || gitRef == "" {
		return InspectReport{}, fmt.Errorf("git source %q must include repository and ref", ref)
	}
	return InspectReport{
		SourceType:   "git",
		SourceRef:    ref,
		ReadyToStage: false,
		Scan: ScanReport{
			Status: ScanStatusWarning,
			Findings: []ScanFinding{{
				Severity: findingWarning,
				Code:     "git_not_cloned",
				Message:  "git inspect is plan-only in this sprint; clone and inspect local path for full scan",
			}},
		},
		Skill: ParsedSkill{
			Name:        suggestNameFromRepository(repository),
			Description: fmt.Sprintf("Planned import from %s@%s", repository, gitRef),
		},
	}, nil
}

func suggestNameFromRepository(repository string) string {
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return "imported-skill"
	}
	parts := strings.Split(repository, "/")
	base := parts[len(parts)-1]
	base = strings.TrimSuffix(base, ".git")
	if err := ValidateSkillName(base); err != nil {
		base = "imported-skill"
	}
	return base
}
