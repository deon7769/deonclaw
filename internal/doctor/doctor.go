package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/config"
	"github.com/deon7769/deonclaw/internal/workerconfig"
)

type OutputFormat string

const (
	OutputText OutputFormat = "text"
	OutputJSON OutputFormat = "json"
)

type Options struct {
	OutputFormat      OutputFormat
	Worker            string
	WorkersConfigPath string
	StorePath         string
	ArtifactsDir      string
}

type Report struct {
	Version      string        `json:"version"`
	OS           string        `json:"os"`
	Arch         string        `json:"arch"`
	WorkingDir   string        `json:"working_dir"`
	Git          ToolCheck     `json:"git"`
	Workers      []WorkerCheck `json:"workers"`
	StorePath    *PathCheck    `json:"store_path,omitempty"`
	ArtifactsDir *PathCheck    `json:"artifacts_dir,omitempty"`
}

type ToolCheck struct {
	Command   string `json:"command"`
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Error     string `json:"error,omitempty"`
}

type WorkerCheck struct {
	Name                 string                             `json:"name"`
	ConfiguredCommand    string                             `json:"configured_command"`
	Provider             string                             `json:"provider,omitempty"`
	Model                string                             `json:"model,omitempty"`
	EnvRequiredOK        bool                               `json:"env_required_ok"`
	EnvRequirements      []workerconfig.EnvRequirementCheck `json:"env_requirements,omitempty"`
	ImplementationStatus string                             `json:"implementation_status"`
	Available            bool                               `json:"available"`
	Path                 string                             `json:"path,omitempty"`
	Error                string                             `json:"error,omitempty"`
	CommandCheck         ToolCheck                          `json:"command_check"`
}

type PathCheck struct {
	Path     string `json:"path"`
	State    string `json:"state"`
	Exists   bool   `json:"exists"`
	Writable bool   `json:"writable"`
	Error    string `json:"error,omitempty"`
}

func Build(opts Options) (Report, error) {
	format := opts.OutputFormat
	if format == "" {
		format = OutputText
	}
	if format != OutputText && format != OutputJSON {
		return Report{}, fmt.Errorf("unsupported output format %q", format)
	}

	workersConfig := workerconfig.Default()
	if strings.TrimSpace(opts.WorkersConfigPath) != "" {
		loaded, err := workerconfig.Load(opts.WorkersConfigPath)
		if err != nil {
			return Report{}, err
		}
		workersConfig = loaded
	}

	wd, err := os.Getwd()
	if err != nil {
		return Report{}, fmt.Errorf("get working directory: %w", err)
	}
	report := Report{
		Version:    config.Version,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		WorkingDir: wd,
		Git:        checkTool("git"),
	}
	workerChecks, err := workerChecks(workersConfig, opts.Worker)
	if err != nil {
		return Report{}, err
	}
	report.Workers = workerChecks
	if strings.TrimSpace(opts.StorePath) != "" {
		check := checkWritableFile(opts.StorePath)
		report.StorePath = &check
	}
	if strings.TrimSpace(opts.ArtifactsDir) != "" {
		check := checkWritableDir(opts.ArtifactsDir)
		report.ArtifactsDir = &check
	}
	return report, nil
}

func Write(report Report, format OutputFormat, out io.Writer) error {
	if format == "" {
		format = OutputText
	}
	switch format {
	case OutputJSON:
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	case OutputText:
		_, err := fmt.Fprintf(out, "deonctl: %s\nos: %s/%s\nworking_dir: %s\ngit: %s\n", report.Version, report.OS, report.Arch, report.WorkingDir, formatTool(report.Git))
		if err != nil {
			return err
		}
		for _, worker := range report.Workers {
			if _, err := fmt.Fprintf(out, "worker %s: command=%s status=%s available=%t env_required_ok=%t", worker.Name, worker.ConfiguredCommand, worker.ImplementationStatus, worker.Available, worker.EnvRequiredOK); err != nil {
				return err
			}
			if worker.Provider != "" {
				if _, err := fmt.Fprintf(out, " provider=%s", worker.Provider); err != nil {
					return err
				}
			}
			if worker.Model != "" {
				if _, err := fmt.Fprintf(out, " model=%s", worker.Model); err != nil {
					return err
				}
			}
			if worker.Path != "" {
				if _, err := fmt.Fprintf(out, " path=%s", worker.Path); err != nil {
					return err
				}
			}
			if worker.Error != "" {
				if _, err := fmt.Fprintf(out, " error=%s", worker.Error); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
			for _, envRequirement := range worker.EnvRequirements {
				if _, err := fmt.Fprintf(out, "worker %s env %s: requirement=%s state=%s\n", worker.Name, envRequirement.Name, envRequirement.Requirement, envRequirement.State); err != nil {
					return err
				}
			}
		}
		if report.StorePath != nil {
			if _, err := fmt.Fprintf(out, "store_path: %s state=%s exists=%t writable=%t", report.StorePath.Path, report.StorePath.State, report.StorePath.Exists, report.StorePath.Writable); err != nil {
				return err
			}
			if report.StorePath.Error != "" {
				if _, err := fmt.Fprintf(out, " error=%s", report.StorePath.Error); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if report.ArtifactsDir != nil {
			if _, err := fmt.Fprintf(out, "artifacts_dir: %s state=%s exists=%t writable=%t", report.ArtifactsDir.Path, report.ArtifactsDir.State, report.ArtifactsDir.Exists, report.ArtifactsDir.Writable); err != nil {
				return err
			}
			if report.ArtifactsDir.Error != "" {
				if _, err := fmt.Fprintf(out, " error=%s", report.ArtifactsDir.Error); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func workerChecks(cfg workerconfig.Config, workerFilter string) ([]WorkerCheck, error) {
	workers := []string{"codex", "opencode"}
	workerFilter = strings.TrimSpace(workerFilter)
	if workerFilter != "" {
		if !isKnownWorker(workerFilter) {
			return nil, fmt.Errorf("unknown worker %q", workerFilter)
		}
		workers = []string{workerFilter}
	}
	sort.Strings(workers)
	checks := make([]WorkerCheck, 0, len(workers))
	for _, worker := range workers {
		workerConfig := cfg.Worker(worker)
		command := workerConfig.Command
		tool := checkTool(command)
		envRequirements := workerConfig.EnvRequirementChecks()
		checks = append(checks, WorkerCheck{
			Name:                 worker,
			ConfiguredCommand:    command,
			Provider:             workerConfig.Provider,
			Model:                workerConfig.Model,
			EnvRequiredOK:        len(workerconfig.MissingRequiredEnv(envRequirements)) == 0,
			EnvRequirements:      envRequirements,
			ImplementationStatus: implementationStatus(worker),
			Available:            tool.Available,
			Path:                 tool.Path,
			Error:                tool.Error,
			CommandCheck:         tool,
		})
	}
	return checks, nil
}

func implementationStatus(worker string) string {
	if worker == "kimi" {
		return "future_worker"
	}
	return "implemented"
}

func isKnownWorker(worker string) bool {
	for _, known := range workerconfig.KnownWorkers() {
		if worker == known {
			return true
		}
	}
	return false
}

func checkTool(command string) ToolCheck {
	command = strings.TrimSpace(command)
	check := ToolCheck{Command: command}
	if command == "" {
		check.Error = "command is empty"
		return check
	}
	path, err := exec.LookPath(command)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	check.Available = true
	check.Path = path
	return check
}

func checkWritableFile(path string) PathCheck {
	check := PathCheck{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkMissingFile(path)
		}
		check.State = "missing_parent_not_writable"
		check.Error = err.Error()
		return check
	}
	check.Exists = true
	if info.IsDir() {
		check.State = "invalid_directory"
		check.Error = "path is a directory"
		return check
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		check.State = "exists_not_writable"
		check.Error = err.Error()
		return check
	}
	_ = file.Close()
	check.State = "exists_writable"
	check.Writable = true
	return check
}

func checkWritableDir(path string) PathCheck {
	check := PathCheck{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkCreatableDir(path)
		}
		check.State = "missing_parent_not_writable"
		check.Error = err.Error()
		return check
	}
	check.Exists = true
	if !info.IsDir() {
		check.State = "invalid_file"
		check.Error = "path is not a directory"
		return check
	}
	temp, err := os.CreateTemp(path, ".deonclaw-doctor-*")
	if err != nil {
		check.State = "exists_not_writable"
		check.Error = err.Error()
		return check
	}
	tempPath := temp.Name()
	_ = temp.Close()
	_ = os.Remove(tempPath)
	check.State = "exists_writable"
	check.Writable = true
	return check
}

func checkMissingFile(path string) PathCheck {
	check := PathCheck{Path: path}
	if err := checkParentWritable(parentDir(path)); err != nil {
		check.State = "missing_parent_not_writable"
		check.Error = err.Error()
		return check
	}
	check.State = "missing_parent_writable"
	check.Writable = true
	return check
}

func checkCreatableDir(path string) PathCheck {
	check := PathCheck{Path: path}
	tempDir, err := os.MkdirTemp(parentDir(path), ".deonclaw-doctor-*")
	if err != nil {
		check.State = "missing_parent_not_writable"
		check.Error = err.Error()
		return check
	}
	_ = os.Remove(tempDir)
	check.State = "missing_parent_writable"
	check.Writable = true
	return check
}

func checkParentWritable(parent string) error {
	info, err := os.Stat(parent)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("parent path is not a directory")
	}
	temp, err := os.CreateTemp(parent, ".deonclaw-doctor-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return os.Remove(tempPath)
}

func parentDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	path = strings.TrimRight(path, string(os.PathSeparator))
	if path == "" {
		return string(os.PathSeparator)
	}
	index := strings.LastIndex(path, string(os.PathSeparator))
	if index < 0 {
		return "."
	}
	if index == 0 {
		return string(os.PathSeparator)
	}
	return path[:index]
}

func formatTool(check ToolCheck) string {
	if check.Available {
		return fmt.Sprintf("available path=%s", check.Path)
	}
	if check.Error != "" {
		return fmt.Sprintf("missing error=%s", check.Error)
	}
	return "missing"
}
