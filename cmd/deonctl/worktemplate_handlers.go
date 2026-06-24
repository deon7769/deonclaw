package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/worktemplates"
)

func runWorkTemplatesValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := worktemplates.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "work templates validate failed: %v\n", err)
		return 1
	}
	if err := worktemplates.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "work templates validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "work templates validate: ok")
	return 0
}

func runWorkTemplatesSync(storePath, configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := worktemplates.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "work templates sync failed: %v\n", err)
		return 1
	}
	if err := worktemplates.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "work templates sync failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work templates sync failed: %v\n", err)
		return 1
	}
	defer db.Close()
	templates := make([]worktemplates.Template, 0, len(cfg.WorkTemplates))
	for _, tmpl := range cfg.WorkTemplates {
		templates = append(templates, tmpl)
	}
	if err := db.SyncWorkTemplates(context.Background(), templates); err != nil {
		fmt.Fprintf(stderr, "work templates sync failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "work templates sync: ok")
	return 0
}

func runWorkTemplatesList(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work templates list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	templates, err := db.ListWorkTemplates(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "work templates list failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(templates)
	return 0
}
