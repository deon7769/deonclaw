package main

import (
	"fmt"
	"os"

	"github.com/deon7769/deonclaw/internal/config"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const usage = `deonctl - DeonClaw control CLI

Usage:
  deonctl version
  deonctl task validate <path>
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("deonclaw %s\n", config.Version)
	case "task":
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		switch os.Args[2] {
		case "validate":
			if len(os.Args) != 4 {
				fmt.Fprint(os.Stderr, usage)
				os.Exit(2)
			}
			runTaskValidate(os.Args[3])
		default:
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func runTaskValidate(path string) {
	task, err := tasks.LoadFromFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(os.Stderr, "validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("task %s: valid\n", task.ID)
}
