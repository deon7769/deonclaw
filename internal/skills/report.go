package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
)

func WriteImportPlanJSON(plan ImportPlan, path string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal import plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write import plan %q: %w", path, err)
	}
	return nil
}

func WriteInstallResultJSON(result InstallResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal install result: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write install result %q: %w", path, err)
	}
	return nil
}

func WriteInspectJSON(report InspectReport, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteInspectText(report InspectReport, w io.Writer) error {
	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "source_type:\t%s\n", report.SourceType)
	fmt.Fprintf(writer, "source_ref:\t%s\n", report.SourceRef)
	fmt.Fprintf(writer, "skill_name:\t%s\n", report.Skill.Name)
	fmt.Fprintf(writer, "description:\t%s\n", report.Skill.Description)
	fmt.Fprintf(writer, "scan_status:\t%s\n", report.Scan.Status)
	fmt.Fprintf(writer, "content_sha256:\t%s\n", report.ContentSHA256)
	fmt.Fprintf(writer, "ready_to_stage:\t%t\n", report.ReadyToStage)
	fmt.Fprintln(writer, "finding\tseverity\tcode\tmessage")
	for _, finding := range report.Scan.Findings {
		fmt.Fprintf(writer, "finding\t%s\t%s\t%s\n", finding.Severity, finding.Code, finding.Message)
	}
	return writer.Flush()
}

func WriteRegistryListText(registry Registry, w io.Writer) error {
	names := make([]string, 0, len(registry.Skills))
	for name := range registry.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "name\tstate\trevision\tsha256")
	for _, name := range names {
		entry := registry.Skills[name]
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", entry.Name, entry.State, entry.CurrentRevision, entry.ContentSHA256)
	}
	return writer.Flush()
}

func WriteSkillShowText(registry Registry, skillName string, w io.Writer) error {
	entry, ok := registry.Skills[skillName]
	if !ok {
		return fmt.Errorf("skill %q not found", skillName)
	}
	writer := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "name:\t%s\n", entry.Name)
	fmt.Fprintf(writer, "state:\t%s\n", entry.State)
	fmt.Fprintf(writer, "revision:\t%s\n", entry.CurrentRevision)
	fmt.Fprintf(writer, "sha256:\t%s\n", entry.ContentSHA256)
	fmt.Fprintf(writer, "installed_at:\t%s\n", entry.InstalledAt)
	return writer.Flush()
}
