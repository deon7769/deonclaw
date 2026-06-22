package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const sidecarFilename = "deonclaw.skill.yaml"

func ParseSKILLFile(path string) (ParsedSkill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParsedSkill{}, fmt.Errorf("read SKILL.md %q: %w", path, err)
	}
	return ParseSKILLContent(string(data), path)
}

func ParseSKILLContent(content string, sourcePath string) (ParsedSkill, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return ParsedSkill{}, err
	}
	var meta SkillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return ParsedSkill{}, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return ParsedSkill{}, fmt.Errorf("SKILL.md frontmatter name is required")
	}
	if err := ValidateSkillName(name); err != nil {
		return ParsedSkill{}, err
	}
	description := strings.TrimSpace(meta.Description)
	if description == "" {
		return ParsedSkill{}, fmt.Errorf("SKILL.md frontmatter description is required")
	}
	return ParsedSkill{
		Name:          name,
		Description:   description,
		License:       strings.TrimSpace(meta.License),
		Compatibility: strings.TrimSpace(meta.Compatibility),
		Body:          strings.TrimSpace(body),
		SourcePath:    filepath.Clean(sourcePath),
		SKILLPath:     filepath.Clean(sourcePath),
	}, nil
}

func splitFrontmatter(content string) (string, string, error) {
	trimmed := strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(trimmed, "---") {
		return "", "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	rest := trimmed[3:]
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else {
		return "", "", fmt.Errorf("SKILL.md frontmatter must be followed by a newline")
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", fmt.Errorf("SKILL.md frontmatter is not closed")
	}
	frontmatter := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimLeft(body, "\r\n")
	return frontmatter, body, nil
}

func LoadSidecar(skillDir string) (*DeonClawSidecar, error) {
	path := filepath.Join(skillDir, sidecarFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sidecar %q: %w", path, err)
	}
	var sidecar DeonClawSidecar
	if err := yaml.Unmarshal(data, &sidecar); err != nil {
		return nil, fmt.Errorf("parse sidecar %q: %w", path, err)
	}
	return &sidecar, nil
}

func DiscoverSkillDirectory(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("skill path is required")
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("stat skill path %q: %w", root, err)
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Base(root), MaterializedSKILLFilename) {
			return filepath.Dir(root), nil
		}
		return "", fmt.Errorf("skill path %q is not a directory", root)
	}
	skillPath := filepath.Join(root, MaterializedSKILLFilename)
	if _, err := os.Stat(skillPath); err == nil {
		return root, nil
	}
	return "", fmt.Errorf("SKILL.md not found under %q", root)
}
