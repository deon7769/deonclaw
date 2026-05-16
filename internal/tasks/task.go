package tasks

type Task struct {
	ID               string        `yaml:"id" json:"id"`
	Title            string        `yaml:"title" json:"title"`
	Domain           string        `yaml:"domain" json:"domain"`
	Worker           string        `yaml:"worker" json:"worker"`
	Goal             string        `yaml:"goal" json:"goal"`
	Mode             string        `yaml:"mode" json:"mode"`
	Workspace        WorkspaceSpec `yaml:"workspace" json:"workspace"`
	Memory           MemorySpec    `yaml:"memory" json:"memory"`
	AllowedPaths     []string      `yaml:"allowed_paths" json:"allowed_paths"`
	ForbiddenPaths   []string      `yaml:"forbidden_paths" json:"forbidden_paths"`
	ExpectedOutputs  []string      `yaml:"expected_outputs" json:"expected_outputs"`
	DefinitionOfDone []string      `yaml:"definition_of_done" json:"definition_of_done"`
}

type WorkspaceSpec struct {
	Strategy string `yaml:"strategy" json:"strategy"`
	Path     string `yaml:"path" json:"path"`
}

type MemorySpec struct {
	Scope string `yaml:"scope" json:"scope"`
}
