package retrievalcontext

import (
	"fmt"

	"github.com/deon7769/deonclaw/internal/tasks"
)

type TaskMaterializedInjectionValidationResult struct {
	Declared               bool   `json:"materialized_injection_declared"`
	Enabled                bool   `json:"materialized_injection_enabled"`
	SupportedNow           bool   `json:"materialized_injection_supported_now"`
	GovernanceBundleSHA256 string `json:"governance_bundle_sha256,omitempty"`
}

func ValidateTaskMaterializedInjection(spec *tasks.MaterializedInjectionSpec) (TaskMaterializedInjectionValidationResult, error) {
	if spec == nil {
		return TaskMaterializedInjectionValidationResult{
			Declared:     false,
			Enabled:      false,
			SupportedNow: false,
		}, nil
	}
	result := TaskMaterializedInjectionValidationResult{
		Declared:     true,
		Enabled:      spec.Enabled,
		SupportedNow: false,
	}
	if spec.Enabled {
		return result, fmt.Errorf("retrieval_context.materialized_injection.enabled: %s", tasks.MaterializedInjectionNotSupportedYet)
	}

	bundle, bundleData, err := LoadInjectionGovernanceBundle(spec.GovernanceBundle)
	if err != nil {
		return result, fmt.Errorf("retrieval_context.materialized_injection.governance_bundle: %w", err)
	}
	if err := ValidateInjectionGovernanceBundleForTaskDeclaration(bundle); err != nil {
		return result, fmt.Errorf("retrieval_context.materialized_injection.governance_bundle: %w", err)
	}
	result.GovernanceBundleSHA256 = sha256Hex(bundleData)
	return result, nil
}
