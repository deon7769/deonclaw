package configenv

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type OutputFormat string

const (
	OutputText OutputFormat = "text"
	OutputJSON OutputFormat = "json"
)

var SensitiveEnvNames = []string{
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ZAI_API_KEY",
	"KIMI_API_KEY",
}

var SensitiveMarkers = []string{
	"TOKEN",
	"KEY",
	"SECRET",
	"PASSWORD",
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ZAI_API_KEY",
	"KIMI_API_KEY",
}

type Entry struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Value string `json:"value"`
}

type Report struct {
	Entries []Entry `json:"entries"`
}

func Build() Report {
	names := map[string]struct{}{}
	for _, name := range SensitiveEnvNames {
		names[name] = struct{}{}
	}
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		if IsSensitiveName(name) {
			names[name] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	report := Report{Entries: make([]Entry, 0, len(ordered))}
	for _, name := range ordered {
		value, ok := os.LookupEnv(name)
		entry := Entry{Name: name}
		if !ok {
			entry.State = "unset"
			entry.Value = "unset"
		} else if IsSensitiveName(name) {
			entry.State = "set"
			entry.Value = "set(masked)"
		} else {
			entry.State = "set"
			entry.Value = value
		}
		report.Entries = append(report.Entries, entry)
	}
	return report
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
		for _, entry := range report.Entries {
			if _, err := fmt.Fprintf(out, "%s=%s\n", entry.Name, entry.Value); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func IsSensitiveName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, marker := range SensitiveMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}
