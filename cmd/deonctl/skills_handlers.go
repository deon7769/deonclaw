package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/skills"
)

type skillsPolicyValidateOptions struct {
	configPath string
}

type skillsInspectOptions struct {
	sourceRef    string
	outputFormat string
}

type skillsImportOptions struct {
	sourceRef  string
	policyPath string
	outputPath string
}

type skillsInstallOptions struct {
	sourcePath   string
	registryRoot string
	asName       string
	policyPath   string
	outputPath   string
}

type skillsVerifyOptions struct {
	skillName    string
	registryRoot string
	outputFormat string
}

type skillsListOptions struct {
	registryRoot string
	outputFormat string
}

type skillsShowOptions struct {
	skillName    string
	registryRoot string
	outputFormat string
}

type skillsEnableOptions struct {
	skillName    string
	agentID      string
	registryRoot string
}

type skillsSnapshotOptions struct {
	agentID      string
	sessionID    string
	registryRoot string
	policyPath   string
	outputPath   string
}

type skillsMaterializeOptions struct {
	snapshotPath string
	workspace    string
	registryRoot string
	outputPath   string
}

func parseSkillsPolicyValidateOptions(args []string) (skillsPolicyValidateOptions, error) {
	var opts skillsPolicyValidateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return skillsPolicyValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return skillsPolicyValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return skillsPolicyValidateOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func parseSkillsInspectOptions(args []string) (skillsInspectOptions, error) {
	opts := skillsInspectOptions{outputFormat: "text"}
	if len(args) == 0 {
		return skillsInspectOptions{}, fmt.Errorf("missing skill source path or reference")
	}
	opts.sourceRef = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--output-format":
			if i+1 >= len(args) {
				return skillsInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return skillsInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return skillsInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseSkillsImportOptions(args []string) (skillsImportOptions, error) {
	var opts skillsImportOptions
	if len(args) == 0 {
		return skillsImportOptions{}, fmt.Errorf("missing skill source path or reference")
	}
	opts.sourceRef = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--policy":
			if i+1 >= len(args) {
				return skillsImportOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return skillsImportOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return skillsImportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputPath == "" {
		return skillsImportOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseSkillsInstallOptions(args []string) (skillsInstallOptions, error) {
	var opts skillsInstallOptions
	if len(args) == 0 {
		return skillsInstallOptions{}, fmt.Errorf("missing skill source path")
	}
	opts.sourcePath = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsInstallOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--as":
			if i+1 >= len(args) {
				return skillsInstallOptions{}, fmt.Errorf("missing value for --as")
			}
			opts.asName = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return skillsInstallOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return skillsInstallOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return skillsInstallOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.registryRoot == "" {
		return skillsInstallOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func loadSkillPolicy(path string) (skills.Policy, error) {
	if strings.TrimSpace(path) == "" {
		return skills.Policy{
			DefaultVisibility: "deny",
			AllowSources:      []string{"local", "managed", "workspace"},
		}, nil
	}
	cfg, err := skills.LoadPolicy(path)
	if err != nil {
		return skills.Policy{}, err
	}
	if err := skills.ValidatePolicy(cfg); err != nil {
		return skills.Policy{}, err
	}
	return cfg.SkillPolicy, nil
}

func runSkillsPolicyValidate(opts skillsPolicyValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := skills.LoadPolicy(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "skills policy validate failed: %v\n", err)
		return 1
	}
	if err := skills.ValidatePolicy(cfg); err != nil {
		fmt.Fprintf(stderr, "skills policy validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "skills policy validate: ok")
	return 0
}

func runSkillsInspect(opts skillsInspectOptions, stdout io.Writer, stderr io.Writer) int {
	report, err := skills.InspectSourceRef(opts.sourceRef)
	if err != nil {
		fmt.Fprintf(stderr, "skills inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		if err := skills.WriteInspectJSON(report, stdout); err != nil {
			fmt.Fprintf(stderr, "skills inspect failed: %v\n", err)
			return 1
		}
	default:
		if err := skills.WriteInspectText(report, stdout); err != nil {
			fmt.Fprintf(stderr, "skills inspect failed: %v\n", err)
			return 1
		}
	}
	return 0
}

func runSkillsImport(opts skillsImportOptions, stdout io.Writer, stderr io.Writer) int {
	policy, err := loadSkillPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "skills import failed: %v\n", err)
		return 1
	}
	plan, err := skills.BuildImportPlan(opts.sourceRef, policy)
	if err != nil {
		fmt.Fprintf(stderr, "skills import failed: %v\n", err)
		return 1
	}
	if err := skills.WriteImportPlanJSON(plan, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "skills import failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "skills import: ok would_install=%t\n", plan.WouldInstall)
	return 0
}

func runSkillsInstall(opts skillsInstallOptions, stdout io.Writer, stderr io.Writer) int {
	policy, err := loadSkillPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "skills install failed: %v\n", err)
		return 1
	}
	result, _, err := skills.InstallLocal(skills.InstallOptions{
		SourcePath:   opts.sourcePath,
		RegistryRoot: opts.registryRoot,
		AsName:       opts.asName,
		Policy:       policy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "skills install failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(opts.outputPath) != "" {
		if err := skills.WriteInstallResultJSON(result, opts.outputPath); err != nil {
			fmt.Fprintf(stderr, "skills install failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "skills install: ok skill=%s revision=%s state=%s\n", result.SkillName, result.RevisionID, result.State)
	return 0
}

func parseSkillsVerifyOptions(args []string) (skillsVerifyOptions, error) {
	opts := skillsVerifyOptions{outputFormat: "text"}
	if len(args) == 0 {
		return skillsVerifyOptions{}, fmt.Errorf("missing skill name")
	}
	opts.skillName = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsVerifyOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return skillsVerifyOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return skillsVerifyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.registryRoot == "" {
		return skillsVerifyOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func runSkillsVerify(opts skillsVerifyOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := skills.VerifySkill(opts.registryRoot, opts.skillName)
	if err != nil {
		fmt.Fprintf(stderr, "skills verify failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "skills verify failed: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stdout, "skills verify: skill=%s revision=%s status=%s\n", result.SkillName, result.Revision, result.Status)
	}
	if result.Status == skills.ScanStatusFailed {
		return 1
	}
	return 0
}

func parseSkillsListOptions(args []string) (skillsListOptions, error) {
	opts := skillsListOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsListOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return skillsListOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return skillsListOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.registryRoot == "" {
		return skillsListOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func runSkillsList(opts skillsListOptions, stdout io.Writer, stderr io.Writer) int {
	registry, err := skills.LoadRegistry(opts.registryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "skills list failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(registry.Skills); err != nil {
			fmt.Fprintf(stderr, "skills list failed: %v\n", err)
			return 1
		}
	default:
		if err := skills.WriteRegistryListText(registry, stdout); err != nil {
			fmt.Fprintf(stderr, "skills list failed: %v\n", err)
			return 1
		}
	}
	return 0
}

func parseSkillsShowOptions(args []string) (skillsShowOptions, error) {
	opts := skillsShowOptions{outputFormat: "text"}
	if len(args) == 0 {
		return skillsShowOptions{}, fmt.Errorf("missing skill name")
	}
	opts.skillName = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsShowOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return skillsShowOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return skillsShowOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.registryRoot == "" {
		return skillsShowOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func runSkillsShow(opts skillsShowOptions, stdout io.Writer, stderr io.Writer) int {
	registry, err := skills.LoadRegistry(opts.registryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "skills show failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		entry, ok := registry.Skills[opts.skillName]
		if !ok {
			fmt.Fprintf(stderr, "skills show failed: skill %q not found\n", opts.skillName)
			return 1
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(entry); err != nil {
			fmt.Fprintf(stderr, "skills show failed: %v\n", err)
			return 1
		}
	default:
		if err := skills.WriteSkillShowText(registry, opts.skillName, stdout); err != nil {
			fmt.Fprintf(stderr, "skills show failed: %v\n", err)
			return 1
		}
	}
	return 0
}

func parseSkillsEnableOptions(args []string) (skillsEnableOptions, error) {
	var opts skillsEnableOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return skillsEnableOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsEnableOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		default:
			if opts.skillName == "" && !strings.HasPrefix(args[i], "--") {
				opts.skillName = args[i]
				continue
			}
			return skillsEnableOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.skillName == "" {
		return skillsEnableOptions{}, fmt.Errorf("missing skill name")
	}
	if opts.agentID == "" {
		return skillsEnableOptions{}, fmt.Errorf("missing --agent")
	}
	if opts.registryRoot == "" {
		return skillsEnableOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func runSkillsApprove(opts skillsShowOptions, stdout io.Writer, stderr io.Writer) int {
	registry, err := skills.LoadRegistry(opts.registryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "skills approve failed: %v\n", err)
		return 1
	}
	registry, err = skills.ApproveSkillInstallation(registry, opts.skillName)
	if err != nil {
		fmt.Fprintf(stderr, "skills approve failed: %v\n", err)
		return 1
	}
	if err := skills.SaveRegistry(registry); err != nil {
		fmt.Fprintf(stderr, "skills approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "skills approve: ok skill=%s state=%s\n", opts.skillName, skills.LifecycleVerified)
	return 0
}

func runSkillsEnable(opts skillsEnableOptions, stdout io.Writer, stderr io.Writer) int {
	registry, err := skills.LoadRegistry(opts.registryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "skills enable failed: %v\n", err)
		return 1
	}
	registry, err = skills.EnableSkillForAgent(registry, opts.agentID, opts.skillName)
	if err != nil {
		fmt.Fprintf(stderr, "skills enable failed: %v\n", err)
		return 1
	}
	if err := skills.SaveRegistry(registry); err != nil {
		fmt.Fprintf(stderr, "skills enable failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "skills enable: ok agent=%s skill=%s\n", opts.agentID, opts.skillName)
	return 0
}

func runSkillsDisable(opts skillsEnableOptions, stdout io.Writer, stderr io.Writer) int {
	registry, err := skills.LoadRegistry(opts.registryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "skills disable failed: %v\n", err)
		return 1
	}
	registry, err = skills.DisableSkillForAgent(registry, opts.agentID, opts.skillName)
	if err != nil {
		fmt.Fprintf(stderr, "skills disable failed: %v\n", err)
		return 1
	}
	if err := skills.SaveRegistry(registry); err != nil {
		fmt.Fprintf(stderr, "skills disable failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "skills disable: ok agent=%s skill=%s\n", opts.agentID, opts.skillName)
	return 0
}

func parseSkillsSnapshotOptions(args []string) (skillsSnapshotOptions, error) {
	var opts skillsSnapshotOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return skillsSnapshotOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--session":
			if i+1 >= len(args) {
				return skillsSnapshotOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.sessionID = args[i+1]
			i++
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsSnapshotOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return skillsSnapshotOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return skillsSnapshotOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return skillsSnapshotOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.agentID == "" {
		return skillsSnapshotOptions{}, fmt.Errorf("missing --agent")
	}
	if opts.sessionID == "" {
		return skillsSnapshotOptions{}, fmt.Errorf("missing --session")
	}
	if opts.registryRoot == "" {
		return skillsSnapshotOptions{}, fmt.Errorf("missing --registry-root")
	}
	if opts.outputPath == "" {
		return skillsSnapshotOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runSkillsSnapshot(opts skillsSnapshotOptions, stdout io.Writer, stderr io.Writer) int {
	policy, err := loadSkillPolicy(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "skills snapshot failed: %v\n", err)
		return 1
	}
	snapshot, err := skills.BuildSnapshot(skills.SnapshotOptions{
		AgentID:      opts.agentID,
		SessionID:    opts.sessionID,
		RegistryRoot: opts.registryRoot,
		Policy:       policy,
	})
	if err != nil {
		fmt.Fprintf(stderr, "skills snapshot failed: %v\n", err)
		return 1
	}
	if err := skills.WriteSnapshotJSON(snapshot, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "skills snapshot failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "skills snapshot: ok skills=%d sha256=%s\n", len(snapshot.Skills), snapshot.SHA256)
	return 0
}

func parseSkillsMaterializeOptions(args []string) (skillsMaterializeOptions, error) {
	var opts skillsMaterializeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--snapshot":
			if i+1 >= len(args) {
				return skillsMaterializeOptions{}, fmt.Errorf("missing value for --snapshot")
			}
			opts.snapshotPath = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return skillsMaterializeOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspace = args[i+1]
			i++
		case "--registry-root":
			if i+1 >= len(args) {
				return skillsMaterializeOptions{}, fmt.Errorf("missing value for --registry-root")
			}
			opts.registryRoot = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return skillsMaterializeOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return skillsMaterializeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.snapshotPath == "" {
		return skillsMaterializeOptions{}, fmt.Errorf("missing --snapshot")
	}
	if opts.workspace == "" {
		return skillsMaterializeOptions{}, fmt.Errorf("missing --workspace")
	}
	if opts.registryRoot == "" {
		return skillsMaterializeOptions{}, fmt.Errorf("missing --registry-root")
	}
	return opts, nil
}

func runSkillsMaterialize(opts skillsMaterializeOptions, stdout io.Writer, stderr io.Writer) int {
	snapshot, err := skills.ReadSnapshotJSON(opts.snapshotPath)
	if err != nil {
		fmt.Fprintf(stderr, "skills materialize failed: %v\n", err)
		return 1
	}
	result, err := skills.MaterializeSnapshot(skills.MaterializeOptions{
		Snapshot:     snapshot,
		Workspace:    opts.workspace,
		RegistryRoot: opts.registryRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "skills materialize failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(opts.outputPath) != "" {
		if err := skills.WriteMaterializeResultJSON(result, opts.outputPath); err != nil {
			fmt.Fprintf(stderr, "skills materialize failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "skills materialize: ok count=%d\n", len(result.Materialized))
	return 0
}
