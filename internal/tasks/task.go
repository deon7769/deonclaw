package tasks

type Task struct {
	ID                string                `yaml:"id" json:"id"`
	Title             string                `yaml:"title" json:"title"`
	Domain            string                `yaml:"domain" json:"domain"`
	Worker            string                `yaml:"worker" json:"worker"`
	ModelProfile      string                `yaml:"model_profile,omitempty" json:"model_profile,omitempty"`
	ModelStrategy     *ModelStrategy        `yaml:"model_strategy,omitempty" json:"model_strategy,omitempty"`
	Goal              string                `yaml:"goal" json:"goal"`
	Mode              string                `yaml:"mode" json:"mode"`
	Workspace         WorkspaceSpec         `yaml:"workspace" json:"workspace"`
	Memory            MemorySpec            `yaml:"memory" json:"memory"`
	Validation        ValidationSpec        `yaml:"validation" json:"validation"`
	MCPContext        MCPContextSpec        `yaml:"mcp_context,omitempty" json:"mcp_context,omitempty"`
	MCPProposalPolicy MCPProposalPolicySpec `yaml:"mcp_proposal_policy,omitempty" json:"mcp_proposal_policy,omitempty"`
	RetrievalContext  RetrievalContextSpec  `yaml:"retrieval_context,omitempty" json:"retrieval_context,omitempty"`
	AllowedPaths      []string              `yaml:"allowed_paths" json:"allowed_paths"`
	ForbiddenPaths    []string              `yaml:"forbidden_paths" json:"forbidden_paths"`
	ExpectedOutputs   []string              `yaml:"expected_outputs" json:"expected_outputs"`
	DefinitionOfDone  []string              `yaml:"definition_of_done" json:"definition_of_done"`
}

type ModelStrategy struct {
	Preferred      []string        `yaml:"preferred" json:"preferred"`
	Fallback       []string        `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	RequireTags    []string        `yaml:"require_tags,omitempty" json:"require_tags,omitempty"`
	FallbackPolicy *FallbackPolicy `yaml:"fallback_policy,omitempty" json:"fallback_policy,omitempty"`
}

type FallbackPolicy struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	MaxAttempts  int      `yaml:"max_attempts,omitempty" json:"max_attempts,omitempty"`
	RetryOn      []string `yaml:"retry_on,omitempty" json:"retry_on,omitempty"`
	NeverRetryOn []string `yaml:"never_retry_on,omitempty" json:"never_retry_on,omitempty"`
}

type WorkspaceSpec struct {
	Strategy string `yaml:"strategy" json:"strategy"`
	Path     string `yaml:"path" json:"path"`
}

type MemorySpec struct {
	Scope string `yaml:"scope" json:"scope"`
}

type ValidationSpec struct {
	Runtime  string              `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Commands []ValidationCommand `yaml:"commands" json:"commands"`
}

type ValidationCommand struct {
	Name           string   `yaml:"name" json:"name"`
	Command        string   `yaml:"command" json:"command"`
	Args           []string `yaml:"args" json:"args"`
	TimeoutSeconds int      `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type MCPContextSpec struct {
	Attachments []MCPContextAttachment `yaml:"attachments,omitempty" json:"attachments,omitempty"`
}

type MCPContextAttachment struct {
	Name string `yaml:"name" json:"name"`
	Kind string `yaml:"kind" json:"kind"`
	Path string `yaml:"path" json:"path"`
}

type MCPProposalPolicySpec struct {
	Config           string `yaml:"config,omitempty" json:"config,omitempty"`
	Policy           string `yaml:"policy,omitempty" json:"policy,omitempty"`
	RuntimeConfig    string `yaml:"runtime_config,omitempty" json:"runtime_config,omitempty"`
	RequirePreflight bool   `yaml:"require_preflight,omitempty" json:"require_preflight,omitempty"`
}

type RetrievalContextSpec struct {
	Attachments           []RetrievalContextAttachment `yaml:"attachments,omitempty" json:"attachments,omitempty"`
	MaterializedInjection *MaterializedInjectionSpec   `yaml:"materialized_injection,omitempty" json:"materialized_injection,omitempty"`
}

type MaterializedInjectionSpec struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	GovernanceBundle   string `yaml:"governance_bundle" json:"governance_bundle"`
	PromptPreview      string `yaml:"prompt_preview" json:"prompt_preview"`
	RequireConfirmFlag bool   `yaml:"require_confirm_flag" json:"require_confirm_flag"`
	MaxTotalChars      int    `yaml:"max_total_chars" json:"max_total_chars"`
}

type RetrievalContextAttachment struct {
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Kind       string `yaml:"kind" json:"kind"`
	Path       string `yaml:"path" json:"path"`
	ReportPath string `yaml:"report_path" json:"report_path"`
	Policy     string `yaml:"policy" json:"policy"`
	MaxResults int    `yaml:"max_results" json:"max_results"`
}
