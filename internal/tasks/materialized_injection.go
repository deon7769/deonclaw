package tasks

import (
	"fmt"
	"path/filepath"
	"strings"
)

const MaterializedInjectionNotSupportedYet = "materialized injection not supported yet"

func HasMaterializedInjection(task *Task) bool {
	return task != nil && task.RetrievalContext.MaterializedInjection != nil
}

func validateMaterializedInjectionSchema(spec *MaterializedInjectionSpec) []error {
	if spec == nil {
		return nil
	}
	var errs []error
	if spec.Enabled {
		errs = append(errs, fmt.Errorf("retrieval_context.materialized_injection.enabled: %s", MaterializedInjectionNotSupportedYet))
	}
	if err := validateTaskRelativeSafePath("retrieval_context.materialized_injection.governance_bundle", spec.GovernanceBundle); err != nil {
		errs = append(errs, err)
	}
	if err := validateTaskRelativeSafePath("retrieval_context.materialized_injection.prompt_preview", spec.PromptPreview); err != nil {
		errs = append(errs, err)
	}
	if !spec.RequireConfirmFlag {
		errs = append(errs, fmt.Errorf("retrieval_context.materialized_injection.require_confirm_flag must be true"))
	}
	if spec.MaxTotalChars <= 0 {
		errs = append(errs, fmt.Errorf("retrieval_context.materialized_injection.max_total_chars must be > 0"))
	}
	return errs
}

func validateTaskRelativeSafePath(field, configured string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fmt.Errorf("%s is required", field)
	}
	if filepath.IsAbs(configured) {
		return fmt.Errorf("%s must be a relative path", field)
	}
	slash := filepath.ToSlash(configured)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("%s must not contain ..", field)
	}
	for _, segment := range strings.Split(slash, "/") {
		if segment == "secrets" {
			return fmt.Errorf("%s uses a blocked path", field)
		}
		if segment == ".env" || strings.HasSuffix(segment, ".env") {
			return fmt.Errorf("%s uses a blocked path", field)
		}
	}
	return nil
}
