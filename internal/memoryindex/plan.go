package memoryindex

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

type DomainPlan struct {
	Domain     string   `json:"domain"`
	SourceRoot string   `json:"source_root"`
	FileCount  int      `json:"file_count"`
	Files      []string `json:"files"`
}

type PlanResult struct {
	Domains      []DomainPlan `json:"domains"`
	SourceCount  int          `json:"source_count"`
	SkippedCount int          `json:"skipped_count"`
}

func Plan(cfg Config) (PlanResult, error) {
	if err := Validate(cfg); err != nil {
		return PlanResult{}, err
	}
	scan, err := scanConfig(cfg)
	if err != nil {
		return PlanResult{}, err
	}

	byDomain := map[string]*DomainPlan{}
	for _, source := range cfg.MemoryIndex.Sources {
		if _, ok := byDomain[source.Domain]; !ok {
			byDomain[source.Domain] = &DomainPlan{
				Domain:     source.Domain,
				SourceRoot: source.Root,
				Files:      []string{},
			}
		}
	}

	for _, candidate := range scan.Candidates {
		plan := byDomain[candidate.Domain]
		if plan == nil {
			continue
		}
		plan.Files = append(plan.Files, candidate.Path)
		plan.FileCount++
	}

	domains := make([]DomainPlan, 0, len(byDomain))
	for _, domain := range sortedDomainPlans(byDomain) {
		sort.Strings(domain.Files)
		domains = append(domains, *domain)
	}

	return PlanResult{
		Domains:      domains,
		SourceCount:  len(scan.Candidates),
		SkippedCount: scan.SkippedCount,
	}, nil
}

func sortedDomainPlans(plans map[string]*DomainPlan) []*DomainPlan {
	keys := make([]string, 0, len(plans))
	for key := range plans {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]*DomainPlan, 0, len(keys))
	for _, key := range keys {
		out = append(out, plans[key])
	}
	return out
}

func WritePlanText(plan PlanResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_index_plan:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "source_count: %d\n", plan.SourceCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "skipped_count: %d\n", plan.SkippedCount); err != nil {
		return err
	}
	for _, domain := range plan.Domains {
		if _, err := fmt.Fprintf(out, "\ndomain: %s\n", domain.Domain); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "source_root: %s\n", domain.SourceRoot); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "file_count: %d\n", domain.FileCount); err != nil {
			return err
		}
		if len(domain.Files) == 0 {
			continue
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "path"); err != nil {
			return err
		}
		for _, path := range domain.Files {
			if _, err := fmt.Fprintf(table, "%s\n", path); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func WritePlanJSON(plan PlanResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(plan)
}
