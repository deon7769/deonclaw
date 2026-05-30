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
	Name              string    `json:"name"`
	ConfiguredCommand string    `json:"configured_command"`
	Available         bool      `json:"available"`
	Path              string    `json:"path,omitempty"`
	Error             string    `json:"error,omitempty"`
	CommandCheck      ToolCheck `json:"command_check"`
}

type PathCheck struct {
	Path     string `json:"path"`
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
		Workers:    workerChecks(workersConfig, opts.Worker),
	}
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
			if _, err := fmt.Fprintf(out, "worker %s: command=%s available=%t", worker.Name, worker.ConfiguredCommand, worker.Available); err != nil {
				return err
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
		}
		if report.StorePath != nil {
			if _, err := fmt.Fprintf(out, "store_path: %s exists=%t writable=%t", report.StorePath.Path, report.StorePath.Exists, report.StorePath.Writable); err != nil {
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
			if _, err := fmt.Fprintf(out, "artifacts_dir: %s exists=%t writable=%t", report.ArtifactsDir.Path, report.ArtifactsDir.Exists, report.ArtifactsDir.Writable); err != nil {
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

func workerChecks(cfg workerconfig.Config, workerFilter string) []WorkerCheck {
	workers := []string{"codex", "opencode"}
	workerFilter = strings.TrimSpace(workerFilter)
	if workerFilter != "" {
		workers = []string{workerFilter}
	}
	sort.Strings(workers)
	checks := make([]WorkerCheck, 0, len(workers))
	for _, worker := range workers {
		command := cfg.Command(worker)
		tool := checkTool(command)
		checks = append(checks, WorkerCheck{
			Name:              worker,
			ConfiguredCommand: command,
			Available:         tool.Available,
			Path:              tool.Path,
			Error:             tool.Error,
			CommandCheck:      tool,
		})
	}
	return checks
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
		check.Error = err.Error()
		return check
	}
	check.Exists = true
	if info.IsDir() {
		check.Error = "path is a directory"
		return check
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	_ = file.Close()
	check.Writable = true
	return check
}

func checkWritableDir(path string) PathCheck {
	check := PathCheck{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	check.Exists = true
	if !info.IsDir() {
		check.Error = "path is not a directory"
		return check
	}
	temp, err := os.CreateTemp(path, ".deonclaw-doctor-*")
	if err != nil {
		check.Error = err.Error()
		return check
	}
	tempPath := temp.Name()
	_ = temp.Close()
	_ = os.Remove(tempPath)
	check.Writable = true
	return check
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
