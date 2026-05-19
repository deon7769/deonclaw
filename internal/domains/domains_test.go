package domains

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFileLoadsExampleDomainsConfig(t *testing.T) {
	configPath := filepath.Join("..", "..", "configs", "examples", "domains.yaml")

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if err := Validate(config); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if len(config.Domains) != 2 {
		t.Fatalf("len(domains) = %d, want 2", len(config.Domains))
	}

	general := config.Domains["general"]
	if general.Name != "general" || general.Type != "canonical_memory" || general.Root != "/vault/mysecondbrain" {
		t.Fatalf("general domain = %#v, want normalized example domain", general)
	}
	if !general.Default {
		t.Fatalf("general default = false, want true")
	}
	if general.IsIsolated() {
		t.Fatalf("general IsIsolated() = true, want false")
	}

	escalasoft := config.Domains["escalasoft"]
	if escalasoft.Name != "escalasoft" || escalasoft.Type != "isolated_domain" {
		t.Fatalf("escalasoft domain = %#v, want isolated domain", escalasoft)
	}
	if !escalasoft.IsIsolated() {
		t.Fatalf("escalasoft IsIsolated() = false, want true")
	}
	if escalasoft.Default {
		t.Fatalf("escalasoft default = true, want false")
	}
	if escalasoft.DefaultAgent != "escalasoft-agent" {
		t.Fatalf("default_agent = %q, want escalasoft-agent", escalasoft.DefaultAgent)
	}
	if len(escalasoft.BridgeFiles) != 3 {
		t.Fatalf("len(bridge_files) = %d, want 3", len(escalasoft.BridgeFiles))
	}
	if escalasoft.StructuredData["historical_sqlite"] == "" || escalasoft.StructuredData["live_sqlite"] == "" {
		t.Fatalf("structured_data = %#v, want sqlite entries", escalasoft.StructuredData)
	}
	if len(escalasoft.Staging) != 2 {
		t.Fatalf("len(staging) = %d, want 2", len(escalasoft.Staging))
	}
}

func TestValidateRejectsInvalidDomainsConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *DomainsConfig
		want   string
	}{
		{
			name:   "no domains",
			config: &DomainsConfig{},
			want:   "at least one domain is required",
		},
		{
			name: "missing default",
			config: &DomainsConfig{Domains: map[string]DomainConfig{
				"general": {Type: "canonical_memory", Root: "/vault/general"},
			}},
			want: "exactly one default domain is required",
		},
		{
			name: "multiple defaults",
			config: &DomainsConfig{Domains: map[string]DomainConfig{
				"general": {Type: "canonical_memory", Root: "/vault/general", Default: true},
				"infra":   {Type: "canonical_memory", Root: "/vault/infra", Default: true},
			}},
			want: "exactly one default domain is required",
		},
		{
			name: "missing root",
			config: &DomainsConfig{Domains: map[string]DomainConfig{
				"general": {Type: "canonical_memory", Default: true},
			}},
			want: "domains[general].root is required",
		},
		{
			name: "missing type",
			config: &DomainsConfig{Domains: map[string]DomainConfig{
				"general": {Root: "/vault/general", Default: true},
			}},
			want: "domains[general].type is required",
		},
		{
			name: "isolated domain cannot be default",
			config: &DomainsConfig{Domains: map[string]DomainConfig{
				"escalasoft": {Type: "isolated_domain", Root: "/domains/escalasoft", Default: true},
			}},
			want: "domains[escalasoft] isolated domain must not be default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalize(tt.config)
			err := Validate(tt.config)
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestListReturnsDomainsSortedByName(t *testing.T) {
	config := &DomainsConfig{Domains: map[string]DomainConfig{
		"zeta":  {Type: "canonical_memory", Root: "/vault/zeta"},
		"alpha": {Type: "canonical_memory", Root: "/vault/alpha", Default: true},
	}}
	normalize(config)

	list := config.List()
	if len(list) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "zeta" {
		t.Fatalf("List() order = %q, %q; want alpha, zeta", list[0].Name, list[1].Name)
	}
}

func TestLoadFromFileWrapsParseErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "domains.yaml")
	if err := os.WriteFile(path, []byte("domains:\n  general: ["), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("LoadFromFile() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parse domains yaml") {
		t.Fatalf("LoadFromFile() error = %q, want parse wrapper", err.Error())
	}
}
