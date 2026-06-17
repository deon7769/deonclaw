package tasks

type Task struct {
	ID               string         `yaml:"id" json:"id"`
	Title            string         `yaml:"title" json:"title"`
	Domain           string         `yaml:"domain" json:"domain"`
	Worker           string         `yaml:"worker" json:"worker"`
	ModelProfile     string         `yaml:"model_profile,omitempty" json:"model_profile,omitempty"`
	ModelStrategy    *ModelStrategy `yaml:"model_strategy,omitempty" json:"model_strategy,omitempty"`
	Goal             string         `yaml:"goal" json:"goal"`
	Mode             string         `yaml:"mode" json:"mode"`
	Workspace        WorkspaceSpec  `yaml:"workspace" json:"workspace"`
	Memory           MemorySpec     `yaml:"memory" json:"memory"`
	Validation       ValidationSpec `yaml:"validation" json:"validation"`
	MCPContext       MCPContextSpec `yaml:"mcp_context,omitempty" json:"mcp_context,omitempty"`
	AllowedPaths     []string       `yaml:"allowed_paths" json:"allowed_paths"`
	ForbiddenPaths   []string       `yaml:"forbidden_paths" json:"forbidden_paths"`
	ExpectedOutputs  []string       `yaml:"expected_outputs" json:"expected_outputs"`
	DefinitionOfDone []string       `yaml:"definition_of_done" json:"definition_of_done"`
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
