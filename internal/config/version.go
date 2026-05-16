package config

const Version = "0.1.0-dev"

var KnownDomains = []string{
	"general",
	"escalasoft",
	"infra",
	"nutri",
}

var KnownWorkers = []string{
	"codex",
	"opencode",
	"claude-code",
}

var KnownModes = []string{
	"read_only",
	"workspace_write",
}

var KnownWorkspaceStrategies = []string{
	"local_repo",
}

var KnownMemoryScopes = []string{
	"none",
	"general",
	"domain",
}
