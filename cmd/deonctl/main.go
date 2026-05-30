package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	artifactspkg "github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/config"
	configenvpkg "github.com/deon7769/deonclaw/internal/configenv"
	"github.com/deon7769/deonclaw/internal/contextpack"
	doctorpkg "github.com/deon7769/deonclaw/internal/doctor"
	"github.com/deon7769/deonclaw/internal/domains"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/runner"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/codex"
	"github.com/deon7769/deonclaw/internal/workers/opencode"
)

const usage = `deonctl - DeonClaw control CLI

Usage:
  deonctl version
  deonctl doctor [--output-format text|json] [--store <path>] [--artifacts-dir <path>] [--workers-config <path>]
  deonctl workers doctor [--worker codex|opencode|kimi] [--output-format text|json] [--store <path>] [--artifacts-dir <path>] [--workers-config <path>]
  deonctl config env [--output-format text|json]
  deonctl task validate <path>
  deonctl domains validate --config <path>
  deonctl domains list --config <path>
  deonctl context build --task <task.yaml> --domains <domains.yaml> --output <path>
  deonctl memory proposal new --run <run-id> --task <task-id> --domain <domain> --target <path> --operation <operation> --reason <text> --output <path>
  deonctl memory proposal lint --proposal <path> --policy <path>
  deonctl memory proposal apply --proposal <path> --policy <path> --dry-run [--output <path>]
  deonctl memory proposal approve --proposal <path> --policy <path> --reviewer <name> --decision approved|rejected --reason <text> --output <path>
  deonctl memory proposal apply-preflight --proposal <path> --approval <path> --policy <path> [--output <path>]
  deonctl memory proposal backup-plan --proposal <path> --approval <path> --policy <path> --output <path>
  deonctl memory proposal backup-materialize --backup-plan <path> --output <path>
  deonctl memory proposal restore --backup-plan <path> --backup-result <path> --dry-run --output <path>
  deonctl memory proposal restore-execute --backup-plan <path> --backup-result <path> --restore-preview <path> --output <path> --confirm-restore
  deonctl memory proposal apply-execute --proposal <path> --approval <path> --policy <path> --backup-plan <path> --backup-result <path> --output <path> --confirm-apply
  deonctl worker codex dry-run <task-path> [--workers-config <path>]
  deonctl worker codex run <task-path> --store <path> --artifacts-dir <path> [--domains <domains.yaml>] [--memory-policy <policy.yaml>] [--workers-config <path>]
  deonctl worker opencode dry-run <task-path> [--workers-config <path>]
  deonctl worker opencode run <task-path> --store <path> --artifacts-dir <path> [--domains <domains.yaml>] [--memory-policy <policy.yaml>] [--workers-config <path>]
  deonctl artifacts list --store <path> [--run <run-id>] [--status <status>]
  deonctl artifacts prune --store <path> --artifacts-dir <path> --older-than <duration> [--dry-run]
`

var codexWorkerFactory = func() workers.Worker {
	return codex.New()
}

var opencodeWorkerFactory = func() workers.Worker {
	return opencode.New()
}

var memoryExecuteApply = memory.ExecuteApply

var runIDFactory = func() string {
	return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
}

var gitDiffRunner = captureGitDiff

var gitSnapshotRunner = git.TakeSnapshot

type workspacePreparer = runner.WorkspacePreparer

type changedFile = runner.ChangedFile

var workspaceManagerFactory = func() workspacePreparer {
	return runtime.NewWorkspaceManager()
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "deonclaw %s\n", config.Version)
		return 0
	case "doctor":
		opts, err := parseDoctorOptions(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runDoctor(opts, stdout, stderr)
	case "workers":
		if len(args) < 2 || args[1] != "doctor" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseDoctorOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runDoctor(opts, stdout, stderr)
	case "config":
		if len(args) < 2 || args[1] != "env" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseConfigEnvOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		return runConfigEnv(opts, stdout, stderr)
	case "task":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			if len(args) != 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runTaskValidate(args[2], stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "domains":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseDomainsOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			return runDomainsValidate(opts, stdout, stderr)
		case "list":
			return runDomainsList(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "context":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "build":
			opts, err := parseContextBuildOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runContextBuild(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "memory":
		if len(args) < 3 || args[1] != "proposal" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[2] {
		case "new":
			opts, err := parseMemoryProposalNewOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalNew(opts, stdout, stderr)
		case "lint":
			opts, err := parseMemoryProposalLintOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalLint(opts, stdout, stderr)
		case "apply":
			opts, err := parseMemoryProposalApplyOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalApply(opts, stdout, stderr)
		case "approve":
			opts, err := parseMemoryProposalApproveOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalApprove(opts, stdout, stderr)
		case "apply-preflight":
			opts, err := parseMemoryProposalApplyPreflightOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalApplyPreflight(opts, stdout, stderr)
		case "backup-plan":
			opts, err := parseMemoryProposalBackupPlanOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalBackupPlan(opts, stdout, stderr)
		case "backup-materialize":
			opts, err := parseMemoryProposalBackupMaterializeOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalBackupMaterialize(opts, stdout, stderr)
		case "restore":
			opts, err := parseMemoryProposalRestoreOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalRestore(opts, stdout, stderr)
		case "restore-execute":
			opts, err := parseMemoryProposalRestoreExecuteOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalRestoreExecute(opts, stdout, stderr)
		case "apply-execute":
			opts, err := parseMemoryProposalApplyExecuteOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runMemoryProposalApplyExecute(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "worker":
		if len(args) < 4 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "codex":
			switch args[2] {
			case "dry-run":
				opts, err := parseWorkerDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexDryRun(opts, stdout, stderr)
			case "run":
				opts, err := parseCodexRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runCodexRun(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		case "opencode":
			switch args[2] {
			case "dry-run":
				opts, err := parseWorkerDryRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runOpenCodeDryRun(opts, stdout, stderr)
			case "run":
				opts, err := parseOpenCodeRunOptions(args[3:])
				if err != nil {
					fmt.Fprintf(stderr, "error: %v\n", err)
					fmt.Fprint(stderr, usage)
					return 2
				}
				return runOpenCodeRun(opts, stdout, stderr)
			default:
				fmt.Fprint(stderr, usage)
				return 2
			}
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "artifacts":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "list":
			opts, err := parseArtifactListOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactList(opts, stdout, stderr)
		case "prune":
			opts, err := parseArtifactPruneOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactPrune(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

type contextBuildOptions struct {
	taskPath    string
	domainsPath string
	outputPath  string
}

func parseContextBuildOptions(args []string) (contextBuildOptions, error) {
	var opts contextBuildOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return contextBuildOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return contextBuildOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.taskPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --task")
	}
	if opts.domainsPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --domains")
	}
	if opts.outputPath == "" {
		return contextBuildOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runContextBuild(opts contextBuildOptions, stdout io.Writer, stderr io.Writer) int {
	pack, err := (contextpack.Builder{}).Build(context.Background(), contextpack.BuildOptions{
		TaskPath:    opts.taskPath,
		DomainsPath: opts.domainsPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "context build failed: %v\n", err)
		return 1
	}

	outputDir := filepath.Dir(opts.outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "context build failed: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(opts.outputPath, pack.Markdown(), 0o600); err != nil {
		fmt.Fprintf(stderr, "context build failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "context pack written: %s\n", opts.outputPath)
	if len(pack.Warnings) > 0 {
		fmt.Fprintf(stdout, "warnings: %d\n", len(pack.Warnings))
	}
	return 0
}

type memoryProposalNewOptions struct {
	runID      string
	taskID     string
	domain     string
	targetPath string
	operation  string
	reason     string
	outputPath string
}

func parseMemoryProposalNewOptions(args []string) (memoryProposalNewOptions, error) {
	var opts memoryProposalNewOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskID = args[i+1]
			i++
		case "--domain":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --domain")
			}
			opts.domain = args[i+1]
			i++
		case "--target":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --target")
			}
			opts.targetPath = args[i+1]
			i++
		case "--operation":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --operation")
			}
			opts.operation = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.runID == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --run")
	}
	if opts.taskID == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --task")
	}
	if opts.domain == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --domain")
	}
	if opts.targetPath == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --target")
	}
	if opts.operation == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --operation")
	}
	if opts.reason == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return memoryProposalNewOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalNew(opts memoryProposalNewOptions, stdout io.Writer, stderr io.Writer) int {
	proposal := memory.NewProposal(memory.NewProposalOptions{
		RunID:      opts.runID,
		TaskID:     opts.taskID,
		Domain:     opts.domain,
		TargetPath: opts.targetPath,
		Operation:  memory.MemoryOperation(opts.operation),
		Reason:     opts.reason,
		Evidence: []memory.MemoryEvidence{
			{Type: "run", RunID: opts.runID},
		},
	})

	var data []byte
	var err error
	if strings.EqualFold(filepath.Ext(opts.outputPath), ".md") {
		data, err = proposal.Markdown()
	} else {
		data, err = proposal.JSON()
	}
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
		return 1
	}

	outputDir := filepath.Dir(opts.outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(opts.outputPath, data, 0o600); err != nil {
		fmt.Fprintf(stderr, "memory proposal failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "memory proposal written: %s\n", opts.outputPath)
	return 0
}

type memoryProposalLintOptions struct {
	proposalPath string
	policyPath   string
}

func parseMemoryProposalLintOptions(args []string) (memoryProposalLintOptions, error) {
	var opts memoryProposalLintOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalLintOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalLintOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		default:
			return memoryProposalLintOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalLintOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalLintOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func runMemoryProposalLint(opts memoryProposalLintOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal lint failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal lint failed: %v\n", err)
		return 1
	}

	result := memory.LintProposal(proposal, policy)
	printMemoryProposalLintResult(stdout, result)
	if result.Status == memory.LintStatusFailed {
		return 1
	}
	return 0
}

func printMemoryProposalLintResult(stdout io.Writer, result memory.LintResult) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "status: %s\n", result.Status)
	fmt.Fprintf(stdout, "violations: %d\n", len(result.Violations))
	for _, violation := range result.Violations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}
	fmt.Fprintf(stdout, "warnings: %d\n", len(result.Warnings))
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
}

type memoryProposalApproveOptions struct {
	proposalPath string
	policyPath   string
	reviewer     string
	decision     string
	reason       string
	outputPath   string
}

func parseMemoryProposalApproveOptions(args []string) (memoryProposalApproveOptions, error) {
	var opts memoryProposalApproveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--reviewer":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --reviewer")
			}
			opts.reviewer = args[i+1]
			i++
		case "--decision":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --decision")
			}
			opts.decision = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.reviewer == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --reviewer")
	}
	if opts.decision == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --decision")
	}
	if opts.reason == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return memoryProposalApproveOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalApprove(opts memoryProposalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
		fmt.Fprintf(stderr, "memory proposal approve failed: refusing to write approval artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal approve failed: refusing to write approval artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	approval, err := memory.BuildApproval(proposal, policy, memory.NewApprovalOptions{
		Reviewer: opts.reviewer,
		Decision: memory.ApprovalDecision(opts.decision),
		Reason:   opts.reason,
	})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}
	if err := writeMemoryApproval(opts.outputPath, approval); err != nil {
		fmt.Fprintf(stderr, "memory proposal approve failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "memory approval written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", approval.ProposalID)
	fmt.Fprintf(stdout, "decision: %s\n", approval.Decision)
	fmt.Fprintf(stdout, "lint_status: %s\n", approval.LintStatus)
	fmt.Fprintf(stdout, "apply_status: %s\n", approval.ApplyStatus)
	return 0
}

func outputConflictsWithApprovalTarget(outputPath string, proposal memory.MemoryProposal) bool {
	if sameCleanPath(outputPath, proposal.TargetPath) {
		return true
	}
	for _, patch := range proposal.Patches {
		targetPath := strings.TrimSpace(patch.TargetPath)
		if targetPath == "" {
			targetPath = proposal.TargetPath
		}
		if sameCleanPath(outputPath, targetPath) {
			return true
		}
	}
	return false
}

type memoryProposalApplyPreflightOptions struct {
	proposalPath string
	approvalPath string
	policyPath   string
	outputPath   string
}

func parseMemoryProposalApplyPreflightOptions(args []string) (memoryProposalApplyPreflightOptions, error) {
	var opts memoryProposalApplyPreflightOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApplyPreflightOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyPreflightOptions{}, fmt.Errorf("missing --policy")
	}
	return opts, nil
}

func runMemoryProposalApplyPreflight(opts memoryProposalApplyPreflightOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
		return 1
	}

	preflight := memory.BuildApplyPreflight(proposal, approval, policy, memory.NewApplyPreflightOptions{})
	printMemoryProposalApplyPreflight(stdout, preflight)
	if opts.outputPath != "" {
		if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: refusing to write preflight artifact to target_path %q\n", opts.outputPath)
			return 1
		}
		if outputTouchesMemoryDomain(opts.outputPath, policy) {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: refusing to write preflight artifact inside memory domain %q\n", opts.outputPath)
			return 1
		}
		if err := writeApplyPreflight(opts.outputPath, preflight); err != nil {
			fmt.Fprintf(stderr, "memory proposal apply-preflight failed: %v\n", err)
			return 1
		}
	}
	if preflight.Status == memory.ApplyPreflightStatusFailed {
		return 1
	}
	return 0
}

func printMemoryProposalApplyPreflight(stdout io.Writer, preflight memory.ApplyPreflight) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", preflight.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", preflight.ApprovalID)
	fmt.Fprintf(stdout, "status: %s\n", preflight.Status)
	fmt.Fprintf(stdout, "failures: %d\n", len(preflight.Failures))
	for _, failure := range preflight.Failures {
		fmt.Fprintf(stdout, "- %s\n", failure)
	}
	fmt.Fprintf(stdout, "lint_status: %s\n", preflight.LintStatus)
	fmt.Fprintf(stdout, "apply_status: %s\n", preflight.ApplyStatus)
	fmt.Fprintf(stdout, "patch_count: %d\n", preflight.PatchCount)
	fmt.Fprintf(stdout, "patch violations: %d\n", len(preflight.PatchViolations))
	for _, violation := range preflight.PatchViolations {
		fmt.Fprintf(stdout, "- patch[%d] %s %s: %s\n", violation.PatchIndex, violation.Operation, violation.TargetPath, violation.Violation)
	}
}

func writeApplyPreflight(outputPath string, preflight memory.ApplyPreflight) error {
	data, err := preflight.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalBackupPlanOptions struct {
	proposalPath string
	approvalPath string
	policyPath   string
	outputPath   string
}

func parseMemoryProposalBackupPlanOptions(args []string) (memoryProposalBackupPlanOptions, error) {
	var opts memoryProposalBackupPlanOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalBackupPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.outputPath == "" {
		return memoryProposalBackupPlanOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalBackupPlan(opts memoryProposalBackupPlanOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: refusing to write backup plan artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: refusing to write backup plan artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	plan, err := memory.BuildBackupPlan(proposal, approval, policy, memory.NewBackupPlanOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}
	if err := writeBackupPlan(opts.outputPath, plan); err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-plan failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup plan written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", plan.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", plan.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(plan.Items))
	return 0
}

func writeBackupPlan(outputPath string, plan memory.BackupPlan) error {
	data, err := plan.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalBackupMaterializeOptions struct {
	backupPlanPath string
	outputPath     string
}

func parseMemoryProposalBackupMaterializeOptions(args []string) (memoryProposalBackupMaterializeOptions, error) {
	var opts memoryProposalBackupMaterializeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.outputPath == "" {
		return memoryProposalBackupMaterializeOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalBackupMaterialize(opts memoryProposalBackupMaterializeOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}
	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: refusing to write backup result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memory.MaterializeBackup(plan, memory.NewBackupMaterializeOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}
	if err := writeBackupResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal backup-materialize failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "backup result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

type memoryProposalRestoreOptions struct {
	backupPlanPath   string
	backupResultPath string
	dryRun           bool
	outputPath       string
}

func parseMemoryProposalRestoreOptions(args []string) (memoryProposalRestoreOptions, error) {
	var opts memoryProposalRestoreOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalRestoreOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalRestoreOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --backup-result")
	}
	if !opts.dryRun {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --dry-run")
	}
	if opts.outputPath == "" {
		return memoryProposalRestoreOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func runMemoryProposalRestore(opts memoryProposalRestoreOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	result, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal restore failed: refusing to write restore preview artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	preview, err := memory.BuildRestorePreview(plan, result, memory.NewRestorePreviewOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}
	if err := writeRestorePreview(opts.outputPath, preview); err != nil {
		fmt.Fprintf(stderr, "memory proposal restore failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "restore preview written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", preview.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", preview.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(preview.Items))
	return 0
}

type memoryProposalRestoreExecuteOptions struct {
	backupPlanPath     string
	backupResultPath   string
	restorePreviewPath string
	outputPath         string
	confirmRestore     bool
}

func parseMemoryProposalRestoreExecuteOptions(args []string) (memoryProposalRestoreExecuteOptions, error) {
	var opts memoryProposalRestoreExecuteOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--restore-preview":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --restore-preview")
			}
			opts.restorePreviewPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-restore":
			opts.confirmRestore = true
		default:
			return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.backupPlanPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --backup-result")
	}
	if opts.restorePreviewPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --restore-preview")
	}
	if opts.outputPath == "" {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmRestore {
		return memoryProposalRestoreExecuteOptions{}, fmt.Errorf("missing --confirm-restore")
	}
	return opts, nil
}

func runMemoryProposalRestoreExecute(opts memoryProposalRestoreExecuteOptions, stdout io.Writer, stderr io.Writer) int {
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	backupResult, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	restorePreview, err := memory.LoadRestorePreviewFromFile(opts.restorePreviewPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}

	if outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, nil) {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: refusing to write restore result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memory.ExecuteRestore(plan, backupResult, restorePreview, memory.NewRestoreExecuteOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}
	if err := writeRestoreResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal restore-execute failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "restore result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

type memoryProposalApplyExecuteOptions struct {
	proposalPath     string
	approvalPath     string
	policyPath       string
	backupPlanPath   string
	backupResultPath string
	outputPath       string
	confirmApply     bool
}

func parseMemoryProposalApplyExecuteOptions(args []string) (memoryProposalApplyExecuteOptions, error) {
	var opts memoryProposalApplyExecuteOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--backup-plan":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --backup-plan")
			}
			opts.backupPlanPath = args[i+1]
			i++
		case "--backup-result":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --backup-result")
			}
			opts.backupResultPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-apply":
			opts.confirmApply = true
		default:
			return memoryProposalApplyExecuteOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --policy")
	}
	if opts.backupPlanPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --backup-plan")
	}
	if opts.backupResultPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --backup-result")
	}
	if opts.outputPath == "" {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmApply {
		return memoryProposalApplyExecuteOptions{}, fmt.Errorf("missing --confirm-apply")
	}
	return opts, nil
}

func runMemoryProposalApplyExecute(opts memoryProposalApplyExecuteOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	approval, err := memory.LoadApprovalFromFile(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	plan, err := memory.LoadBackupPlanFromFile(opts.backupPlanPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	backupResult, err := memory.LoadBackupResultFromFile(opts.backupResultPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}

	if outputConflictsWithApprovalTarget(opts.outputPath, proposal) || outputConflictsWithBackupPlanTarget(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact to target_path %q\n", opts.outputPath)
		return 1
	}
	if outputConflictsWithBackupPlanBackupPath(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact to backup_path %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesBackupRoot(opts.outputPath, plan) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact inside backup_root %q\n", opts.outputPath)
		return 1
	}
	if outputTouchesMemoryDomain(opts.outputPath, policy) {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: refusing to write apply result artifact inside memory domain %q\n", opts.outputPath)
		return 1
	}

	result, err := memoryExecuteApply(proposal, approval, policy, plan, backupResult, memory.NewApplyExecuteOptions{})
	if err != nil {
		var applyErr *memory.ApplyExecutionError
		if errors.As(err, &applyErr) && applyErr.Result.Status == memory.ApplyResultStatusPartialFailed {
			if writeErr := writeApplyResult(opts.outputPath, applyErr.Result); writeErr != nil {
				fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", writeErr)
				return 1
			}
		}
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}
	if err := writeApplyResult(opts.outputPath, result); err != nil {
		fmt.Fprintf(stderr, "memory proposal apply-execute failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "apply result written: %s\n", opts.outputPath)
	fmt.Fprintf(stdout, "proposal_id: %s\n", result.ProposalID)
	fmt.Fprintf(stdout, "approval_id: %s\n", result.ApprovalID)
	fmt.Fprintf(stdout, "items: %d\n", len(result.Items))
	return 0
}

func outputConflictsWithBackupPlanTarget(outputPath string, plan memory.BackupPlan) bool {
	for _, item := range plan.Items {
		if sameCleanPath(outputPath, item.TargetPath) {
			return true
		}
	}
	return false
}

func outputConflictsWithBackupPlanBackupPath(outputPath string, plan memory.BackupPlan) bool {
	for _, item := range plan.Items {
		if sameCleanPath(outputPath, item.BackupPath) {
			return true
		}
	}
	return false
}

func outputTouchesBackupRoot(outputPath string, plan memory.BackupPlan) bool {
	if strings.TrimSpace(plan.BackupRoot) == "" {
		return false
	}
	return cleanPathWithinRoot(outputPath, plan.BackupRoot)
}

func writeBackupResult(outputPath string, result memory.BackupResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeRestorePreview(outputPath string, preview memory.RestorePreview) error {
	data, err := preview.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeRestoreResult(outputPath string, result memory.RestoreResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeApplyResult(outputPath string, result memory.ApplyResult) error {
	data, err := result.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

type memoryProposalApplyOptions struct {
	proposalPath string
	policyPath   string
	dryRun       bool
	outputPath   string
}

func parseMemoryProposalApplyOptions(args []string) (memoryProposalApplyOptions, error) {
	var opts memoryProposalApplyOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --policy")
			}
			opts.policyPath = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		case "--output":
			if i+1 >= len(args) {
				return memoryProposalApplyOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return memoryProposalApplyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.policyPath == "" {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --policy")
	}
	if !opts.dryRun {
		return memoryProposalApplyOptions{}, fmt.Errorf("missing --dry-run")
	}
	return opts, nil
}

func runMemoryProposalApply(opts memoryProposalApplyOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := memory.LoadProposalFromFile(opts.proposalPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
		return 1
	}
	policy, err := memory.LoadPolicyFromFile(opts.policyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
		return 1
	}

	preview, previewErr := memory.BuildApplyDryRunPreview(proposal, policy)
	printMemoryProposalApplyPreview(stdout, preview)
	if opts.outputPath != "" {
		if outputConflictsWithApplyTarget(opts.outputPath, preview) {
			fmt.Fprintf(stderr, "memory proposal apply failed: refusing to write apply preview to target_path %q\n", opts.outputPath)
			return 1
		}
		if err := writeApplyPreview(opts.outputPath, preview); err != nil {
			fmt.Fprintf(stderr, "memory proposal apply failed: %v\n", err)
			return 1
		}
	}
	if previewErr != nil {
		return 1
	}
	return 0
}

func outputConflictsWithApplyTarget(outputPath string, preview memory.ApplyPreview) bool {
	if sameCleanPath(outputPath, preview.TargetPath) {
		return true
	}
	for _, action := range preview.Actions {
		if sameCleanPath(outputPath, action.TargetPath) {
			return true
		}
	}
	return false
}

func outputTouchesMemoryDomain(outputPath string, policy *memory.MemoryPolicy) bool {
	roots := []string{
		"/vault/mysecondbrain",
		"vault/mysecondbrain",
		"mysecondbrain",
		"/domains/escalasoft_brain",
		"domains/escalasoft_brain",
		"escalasoft_brain",
	}
	if policy != nil {
		for _, domainPolicy := range policy.IsolatedDomains {
			if strings.TrimSpace(domainPolicy.Root) == "" {
				continue
			}
			roots = append(roots, domainPolicy.Root)
			roots = append(roots, strings.TrimPrefix(domainPolicy.Root, "/"))
			roots = append(roots, filepath.Base(domainPolicy.Root))
		}
	}
	for _, root := range roots {
		if cleanPathWithinRoot(outputPath, root) {
			return true
		}
	}
	return false
}

func cleanPathWithinRoot(value string, root string) bool {
	value = cleanSlashPath(value)
	root = cleanSlashPath(root)
	if value == "" || root == "" {
		return false
	}
	value = strings.Trim(value, "/")
	root = strings.Trim(root, "/")
	if value == root || strings.HasPrefix(value, root+"/") {
		return true
	}
	return strings.Contains(value, "/"+root+"/")
}

func cleanSlashPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(value))
	if value == "." {
		return ""
	}
	return value
}

func sameCleanPath(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return filepath.ToSlash(filepath.Clean(left)) == filepath.ToSlash(filepath.Clean(right))
}

func printMemoryProposalApplyPreview(stdout io.Writer, preview memory.ApplyPreview) {
	fmt.Fprintf(stdout, "proposal_id: %s\n", preview.ProposalID)
	fmt.Fprintf(stdout, "domain: %s\n", preview.Domain)
	fmt.Fprintf(stdout, "target_path: %s\n", preview.TargetPath)
	fmt.Fprintf(stdout, "operation: %s\n", preview.Operation)
	fmt.Fprintf(stdout, "status: %s\n", preview.Status)

	fmt.Fprintf(stdout, "lint warnings: %d\n", len(preview.LintWarnings))
	for _, warning := range preview.LintWarnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
	fmt.Fprintf(stdout, "lint violations: %d\n", len(preview.LintViolations))
	for _, violation := range preview.LintViolations {
		fmt.Fprintf(stdout, "- %s\n", violation)
	}

	fmt.Fprintf(stdout, "patch_count: %d\n", preview.PatchCount)
	fmt.Fprintf(stdout, "patch warnings: %d\n", len(preview.PatchWarnings))
	for _, warning := range preview.PatchWarnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
	fmt.Fprintf(stdout, "patch violations: %d\n", len(preview.PatchViolations))
	for _, violation := range preview.PatchViolations {
		fmt.Fprintf(stdout, "- patch[%d] %s %s: %s\n", violation.PatchIndex, violation.Operation, violation.TargetPath, violation.Violation)
	}
	if len(preview.Actions) == 0 {
		return
	}

	fmt.Fprintln(stdout, "actions:")
	for _, action := range preview.Actions {
		fmt.Fprintf(stdout, "- %s: %s\n", action.Description, action.TargetPath)
		if action.Warning != "" {
			fmt.Fprintf(stdout, "  warning: %s\n", action.Warning)
		}
		for _, violation := range action.Violations {
			fmt.Fprintf(stdout, "  violation: %s\n", violation)
		}
		if action.Content != "" {
			fmt.Fprintln(stdout, "  content:")
			writeIndentedText(stdout, action.Content, "    ")
		}
	}
}

func writeApplyPreview(outputPath string, preview memory.ApplyPreview) error {
	data, err := preview.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, data, 0o600)
}

func writeMemoryApproval(outputPath string, approval memory.MemoryApproval) error {
	data, err := approval.JSON()
	if err != nil {
		return err
	}
	outputDir := filepath.Dir(outputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return err
	}
	return nil
}

func writeIndentedText(output io.Writer, value string, prefix string) {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		fmt.Fprintf(output, "%s%s\n", prefix, line)
	}
}

func runTaskValidate(path string, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "task %s: valid\n", task.ID)
	return 0
}

type domainsOptions struct {
	configPath string
}

func parseDomainsOptions(args []string) (domainsOptions, error) {
	var opts domainsOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return domainsOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return domainsOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return domainsOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func loadAndValidateDomains(path string) (*domains.DomainsConfig, error) {
	config, err := domains.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := domains.Validate(config); err != nil {
		return nil, err
	}
	return config, nil
}

func runDomainsValidate(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "domains config valid: %d domains\n", len(config.Domains))
	return 0
}

func runDomainsList(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "name\ttype\troot\tdefault\tisolated\tdefault_agent")
	for _, domain := range config.List() {
		fmt.Fprintf(table, "%s\t%s\t%s\t%t\t%t\t%s\n",
			domain.Name,
			domain.Type,
			domain.Root,
			domain.Default,
			domain.IsIsolated(),
			domain.DefaultAgent,
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "domains list failed: %v\n", err)
		return 1
	}
	return 0
}

type doctorOptions struct {
	outputFormat      string
	worker            string
	workersConfigPath string
	storePath         string
	artifactsDir      string
}

func parseDoctorOptions(args []string) (doctorOptions, error) {
	opts := doctorOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output-format":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		case "--worker":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --worker")
			}
			opts.worker = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return doctorOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		default:
			return doctorOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return doctorOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runDoctor(opts doctorOptions, stdout io.Writer, stderr io.Writer) int {
	report, err := doctorpkg.Build(doctorpkg.Options{
		OutputFormat:      doctorpkg.OutputFormat(opts.outputFormat),
		Worker:            opts.worker,
		WorkersConfigPath: opts.workersConfigPath,
		StorePath:         opts.storePath,
		ArtifactsDir:      opts.artifactsDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "doctor failed: %v\n", err)
		return 1
	}
	if err := doctorpkg.Write(report, doctorpkg.OutputFormat(opts.outputFormat), stdout); err != nil {
		fmt.Fprintf(stderr, "doctor output failed: %v\n", err)
		return 1
	}
	return 0
}

type configEnvOptions struct {
	outputFormat string
}

func parseConfigEnvOptions(args []string) (configEnvOptions, error) {
	opts := configEnvOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output-format":
			if i+1 >= len(args) {
				return configEnvOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return configEnvOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return configEnvOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runConfigEnv(opts configEnvOptions, stdout io.Writer, stderr io.Writer) int {
	report := configenvpkg.Build()
	if err := configenvpkg.Write(report, configenvpkg.OutputFormat(opts.outputFormat), stdout); err != nil {
		fmt.Fprintf(stderr, "config env output failed: %v\n", err)
		return 1
	}
	return 0
}

type workerDryRunOptions struct {
	taskPath          string
	workersConfigPath string
}

func parseWorkerDryRunOptions(args []string) (workerDryRunOptions, error) {
	if len(args) < 1 {
		return workerDryRunOptions{}, fmt.Errorf("missing task path")
	}
	opts := workerDryRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--workers-config":
			if i+1 >= len(args) {
				return workerDryRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return workerDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return opts, nil
}

func runCodexDryRun(opts workerDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if err := ensureTaskWorker(task, "codex"); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	worker, err := configuredCodexWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(event.Command, " "))
	return 0
}

func runOpenCodeDryRun(opts workerDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if err := ensureTaskWorker(task, "opencode"); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	worker, err := configuredOpenCodeWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "policy: %s\n", event.Sandbox)
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(event.Command, " "))
	return 0
}

type codexRunOptions struct {
	taskPath          string
	storePath         string
	artifactsDir      string
	domainsPath       string
	memoryPolicyPath  string
	workersConfigPath string
}

func parseCodexRunOptions(args []string) (codexRunOptions, error) {
	if len(args) < 1 {
		return codexRunOptions{}, fmt.Errorf("missing task path")
	}

	opts := codexRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--memory-policy":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --memory-policy")
			}
			opts.memoryPolicyPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return codexRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return codexRunOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return codexRunOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	return opts, nil
}

func runCodexRun(opts codexRunOptions, stdout io.Writer, stderr io.Writer) int {
	worker, err := configuredCodexWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	codexRunner := runner.CodexRunner{
		WorkerFactory: func() workers.Worker {
			return worker
		},
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}
	return codexRunner.Run(context.Background(), runner.CodexRunOptions{
		TaskPath:         opts.taskPath,
		StorePath:        opts.storePath,
		ArtifactsDir:     opts.artifactsDir,
		DomainsPath:      opts.domainsPath,
		MemoryPolicyPath: opts.memoryPolicyPath,
	}, stdout, stderr)
}

type openCodeRunOptions struct {
	taskPath          string
	storePath         string
	artifactsDir      string
	domainsPath       string
	memoryPolicyPath  string
	workersConfigPath string
}

func parseOpenCodeRunOptions(args []string) (openCodeRunOptions, error) {
	if len(args) < 1 {
		return openCodeRunOptions{}, fmt.Errorf("missing task path")
	}

	opts := openCodeRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--domains":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --domains")
			}
			opts.domainsPath = args[i+1]
			i++
		case "--memory-policy":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --memory-policy")
			}
			opts.memoryPolicyPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return openCodeRunOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return openCodeRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return openCodeRunOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return openCodeRunOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	return opts, nil
}

func runOpenCodeRun(opts openCodeRunOptions, stdout io.Writer, stderr io.Writer) int {
	worker, err := configuredOpenCodeWorker(opts.workersConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	openCodeRunner := runner.OpenCodeRunner{
		WorkerFactory: func() workers.Worker {
			return worker
		},
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}
	return openCodeRunner.Run(context.Background(), runner.OpenCodeRunOptions{
		TaskPath:         opts.taskPath,
		StorePath:        opts.storePath,
		ArtifactsDir:     opts.artifactsDir,
		DomainsPath:      opts.domainsPath,
		MemoryPolicyPath: opts.memoryPolicyPath,
	}, stdout, stderr)
}

func configuredCodexWorker(workersConfigPath string) (workers.Worker, error) {
	if strings.TrimSpace(workersConfigPath) == "" {
		return codexWorkerFactory(), nil
	}
	cfg, err := workerconfig.Load(workersConfigPath)
	if err != nil {
		return nil, err
	}
	return codex.NewWithCommand(cfg.Command("codex")), nil
}

func configuredOpenCodeWorker(workersConfigPath string) (workers.Worker, error) {
	if strings.TrimSpace(workersConfigPath) == "" {
		return opencodeWorkerFactory(), nil
	}
	cfg, err := workerconfig.Load(workersConfigPath)
	if err != nil {
		return nil, err
	}
	return opencode.NewWithCommand(cfg.Command("opencode")), nil
}

type artifactListOptions struct {
	storePath string
	runID     string
	status    runs.RunStatus
}

func parseArtifactListOptions(args []string) (artifactListOptions, error) {
	var opts artifactListOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--status":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --status")
			}
			opts.status = runs.RunStatus(args[i+1])
			i++
		default:
			return artifactListOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactListOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

type artifactPruneOptions struct {
	storePath    string
	artifactsDir string
	olderThan    time.Duration
	olderThanRaw string
	dryRun       bool
}

func parseArtifactPruneOptions(args []string) (artifactPruneOptions, error) {
	var opts artifactPruneOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--older-than":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --older-than")
			}
			duration, err := parseRetentionDuration(args[i+1])
			if err != nil {
				return artifactPruneOptions{}, err
			}
			opts.olderThan = duration
			opts.olderThanRaw = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		default:
			return artifactPruneOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.olderThan == 0 {
		return artifactPruneOptions{}, fmt.Errorf("missing --older-than")
	}
	return opts, nil
}

func runArtifactList(opts artifactListOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := db.ListArtifacts(context.Background(), storepkg.ArtifactListFilter{
		RunID:  opts.runID,
		Status: opts.status,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "artifact_id\trun_id\tkind\tpath\tsize_bytes\tsha256\tkeep\tcreated_at")
	for _, artifact := range result {
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%d\t%s\t%t\t%s\n",
			artifact.ID,
			artifact.RunID,
			artifact.Kind,
			artifact.Path,
			artifact.SizeBytes,
			artifact.SHA256,
			artifact.Keep,
			artifact.CreatedAt.Format(time.RFC3339Nano),
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}
	return 0
}

func parseRetentionDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("missing --older-than")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid --older-than %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --older-than %q", value)
	}
	return duration, nil
}

func runArtifactPrune(opts artifactPruneOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := artifactspkg.Prune(context.Background(), db, artifactspkg.RetentionConfig{
		ArtifactsDir: opts.artifactsDir,
		OlderThan:    opts.olderThan,
		DryRun:       opts.dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact prune failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "dry_run: %t\n", result.DryRun)
	fmt.Fprintf(stdout, "older_than: %s\n", opts.olderThanRaw)
	fmt.Fprintf(stdout, "cutoff: %s\n", result.Cutoff.Format(time.RFC3339Nano))
	fmt.Fprintf(stdout, "candidates: %d\n", result.Candidates)
	fmt.Fprintf(stdout, "deleted: %d\n", result.Deleted)
	fmt.Fprintf(stdout, "skipped: %d\n", result.Skipped)
	fmt.Fprintf(stdout, "bytes: %d\n", result.SizeBytes)
	return 0
}

func captureGitDiff(ctx context.Context, workspace string) ([]byte, error) {
	return runner.CaptureGitDiff(ctx, workspace)
}

func ensureTaskWorker(task *tasks.Task, requested string) error {
	if task.Worker != requested {
		return fmt.Errorf("task worker %q does not match requested worker %q", task.Worker, requested)
	}
	return nil
}
