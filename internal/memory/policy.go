package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type MemoryPolicy struct {
	DefaultWriteMode string                          `yaml:"default_write_mode" json:"default_write_mode"`
	Protected        []string                        `yaml:"protected" json:"protected"`
	Controlled       []string                        `yaml:"controlled" json:"controlled"`
	AppendOnly       []string                        `yaml:"append_only" json:"append_only"`
	IsolatedDomains  map[string]IsolatedDomainPolicy `yaml:"isolated_domains" json:"isolated_domains"`
}

type IsolatedDomainPolicy struct {
	Root                 string   `yaml:"root" json:"root"`
	ForbiddenGlobalWrite []string `yaml:"forbidden_global_write" json:"forbidden_global_write"`
	AllowedGlobalBridge  []string `yaml:"allowed_global_bridge" json:"allowed_global_bridge"`
}

type policyFile struct {
	MemoryPolicy MemoryPolicy `yaml:"memory_policy"`
}

type LintStatus string

const (
	LintStatusOK     LintStatus = "ok"
	LintStatusFailed LintStatus = "failed"
)

type LintResult struct {
	ProposalID string
	Status     LintStatus
	Violations []string
	Warnings   []string
}

func LoadPolicyFromFile(policyPath string) (*MemoryPolicy, error) {
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("read memory policy %q: %w", policyPath, err)
	}

	var parsed policyFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse memory policy yaml: %w", err)
	}
	normalizePolicy(&parsed.MemoryPolicy)
	return &parsed.MemoryPolicy, nil
}

func LoadProposalFromFile(proposalPath string) (MemoryProposal, error) {
	data, err := os.ReadFile(proposalPath)
	if err != nil {
		return MemoryProposal{}, fmt.Errorf("read memory proposal %q: %w", proposalPath, err)
	}

	var proposal MemoryProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		return MemoryProposal{}, fmt.Errorf("parse memory proposal json: %w", err)
	}
	return proposal, nil
}

func LintProposal(proposal MemoryProposal, policy *MemoryPolicy) LintResult {
	result := LintResult{
		ProposalID: proposal.ProposalID,
		Status:     LintStatusOK,
	}
	if err := proposal.Validate(); err != nil {
		result.Violations = append(result.Violations, err.Error())
		result.Status = LintStatusFailed
		return result
	}
	if policy == nil {
		result.Violations = append(result.Violations, "memory policy is required")
		result.Status = LintStatusFailed
		return result
	}

	targetPath := normalizeMemoryPath(proposal.TargetPath)
	isolatedPolicy, isIsolatedDomain := policy.IsolatedDomains[proposal.Domain]
	allowedBridge, bridgePattern := firstMatchingPattern(targetPath, isolatedPolicy.AllowedGlobalBridge)
	if allowedBridge {
		result.Warnings = append(result.Warnings, fmt.Sprintf("target_path %q matches allowed_global_bridge %q; controlled review required", proposal.TargetPath, bridgePattern))
		return result
	}

	if protected, pattern := firstMatchingPattern(targetPath, policy.Protected); protected {
		result.Violations = append(result.Violations, fmt.Sprintf("target_path %q matches protected path %q", proposal.TargetPath, pattern))
	}

	if isIsolatedDomain {
		lintIsolatedDomainProposal(proposal, targetPath, isolatedPolicy, &result)
	}

	if len(result.Violations) > 0 {
		result.Status = LintStatusFailed
	}
	return result
}

func lintIsolatedDomainProposal(proposal MemoryProposal, targetPath string, policy IsolatedDomainPolicy, result *LintResult) {
	if forbidden, pattern := firstMatchingPattern(targetPath, policy.ForbiddenGlobalWrite); forbidden {
		result.Violations = append(result.Violations, fmt.Sprintf("target_path %q matches forbidden_global_write %q", proposal.TargetPath, pattern))
		return
	}

	if withinRoot(targetPath, policy.Root) || isMemoryInboxPath(targetPath) {
		return
	}

	result.Violations = append(result.Violations, fmt.Sprintf("isolated domain %q cannot write target_path %q", proposal.Domain, proposal.TargetPath))
}

func normalizePolicy(policy *MemoryPolicy) {
	if policy == nil {
		return
	}
	policy.DefaultWriteMode = strings.TrimSpace(policy.DefaultWriteMode)
	trimStrings(policy.Protected)
	trimStrings(policy.Controlled)
	trimStrings(policy.AppendOnly)
	if policy.IsolatedDomains == nil {
		policy.IsolatedDomains = map[string]IsolatedDomainPolicy{}
	}
	for domain, domainPolicy := range policy.IsolatedDomains {
		normalizedDomain := strings.TrimSpace(domain)
		domainPolicy.Root = strings.TrimSpace(domainPolicy.Root)
		trimStrings(domainPolicy.ForbiddenGlobalWrite)
		trimStrings(domainPolicy.AllowedGlobalBridge)
		if normalizedDomain != domain {
			delete(policy.IsolatedDomains, domain)
		}
		policy.IsolatedDomains[normalizedDomain] = domainPolicy
	}
}

func trimStrings(values []string) {
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
}

func firstMatchingPattern(targetPath string, patterns []string) (bool, string) {
	for _, pattern := range patterns {
		if matchMemoryPath(pattern, targetPath) {
			return true, normalizeMemoryPath(pattern)
		}
	}
	return false, ""
}

func matchMemoryPath(pattern string, targetPath string) bool {
	normalizedPattern := normalizeMemoryPath(pattern)
	normalizedTarget := normalizeMemoryPath(targetPath)
	if normalizedPattern == "" || normalizedTarget == "" {
		return false
	}
	if normalizedPattern == "**" {
		return true
	}
	if strings.HasSuffix(normalizedPattern, "/**") {
		prefix := strings.TrimSuffix(normalizedPattern, "/**")
		return normalizedTarget == prefix || strings.HasPrefix(normalizedTarget, prefix+"/")
	}
	matched, err := path.Match(normalizedPattern, normalizedTarget)
	return err == nil && matched
}

func normalizeMemoryPath(value string) string {
	value = strings.TrimSpace(filepath.ToSlash(value))
	if value == "" {
		return ""
	}
	value = path.Clean(value)
	if value == "." {
		return ""
	}
	return strings.TrimPrefix(value, "./")
}

func withinRoot(targetPath string, root string) bool {
	targetPath = normalizeMemoryPath(targetPath)
	root = normalizeMemoryPath(root)
	if targetPath == "" || root == "" {
		return false
	}
	if targetPath == root || strings.HasPrefix(targetPath, root+"/") {
		return true
	}
	relativeRoot := strings.TrimPrefix(root, "/")
	if targetPath == relativeRoot || strings.HasPrefix(targetPath, relativeRoot+"/") {
		return true
	}
	rootBase := path.Base(root)
	return targetPath == rootBase || strings.HasPrefix(targetPath, rootBase+"/")
}

func isMemoryInboxPath(targetPath string) bool {
	return matchMemoryPath("/vault/mysecondbrain/memory/inbox/**", targetPath) ||
		matchMemoryPath("memory/inbox/**", targetPath)
}
