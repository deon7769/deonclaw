package main

import "fmt"

func parseAgentsValidateOptions(args []string) (agentsValidateOptions, error) {
	var opts agentsValidateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return agentsValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return agentsValidateOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return agentsValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return agentsValidateOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func parseAgentsSyncOptions(args []string) (agentsSyncOptions, error) {
	var opts agentsSyncOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return agentsSyncOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return agentsSyncOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--workers-config":
			if i+1 >= len(args) {
				return agentsSyncOptions{}, fmt.Errorf("missing value for --workers-config")
			}
			opts.workersConfigPath = args[i+1]
			i++
		default:
			return agentsSyncOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return agentsSyncOptions{}, fmt.Errorf("missing --config")
	}
	if opts.storePath == "" {
		return agentsSyncOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsStoreOptions(args []string) (agentsStoreOptions, error) {
	var opts agentsStoreOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return agentsStoreOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		default:
			return agentsStoreOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return agentsStoreOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsLifecycleOptions(agentID string, args []string) (agentsLifecycleOptions, error) {
	opts := agentsLifecycleOptions{agentID: agentID}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return agentsLifecycleOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return agentsLifecycleOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--actor":
			if i+1 >= len(args) {
				return agentsLifecycleOptions{}, fmt.Errorf("missing value for --actor")
			}
			opts.actor = args[i+1]
			i++
		default:
			return agentsLifecycleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return agentsLifecycleOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsSessionCreateOptions(args []string) (agentsSessionCreateOptions, error) {
	var opts agentsSessionCreateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--kind":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --kind")
			}
			opts.kind = args[i+1]
			i++
		case "--session":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --session")
			}
			opts.sessionID = args[i+1]
			i++
		case "--workspace":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --workspace")
			}
			opts.workspacePath = args[i+1]
			i++
		case "--skill-snapshot":
			if i+1 >= len(args) {
				return agentsSessionCreateOptions{}, fmt.Errorf("missing value for --skill-snapshot")
			}
			opts.skillSnapshotPath = args[i+1]
			i++
		default:
			return agentsSessionCreateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.agentID == "" {
		return agentsSessionCreateOptions{}, fmt.Errorf("missing --agent")
	}
	if opts.storePath == "" {
		return agentsSessionCreateOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsSessionShowOptions(sessionID string, args []string) (agentsSessionShowOptions, error) {
	opts := agentsSessionShowOptions{sessionID: sessionID, outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return agentsSessionShowOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return agentsSessionShowOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return agentsSessionShowOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return agentsSessionShowOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsAssignOptions(args []string) (agentsAssignOptions, error) {
	var opts agentsAssignOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return agentsAssignOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return agentsAssignOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return agentsAssignOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--created-by":
			if i+1 >= len(args) {
				return agentsAssignOptions{}, fmt.Errorf("missing value for --created-by")
			}
			opts.createdBy = args[i+1]
			i++
		default:
			return agentsAssignOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.agentID == "" {
		return agentsAssignOptions{}, fmt.Errorf("missing --agent")
	}
	if opts.taskPath == "" {
		return agentsAssignOptions{}, fmt.Errorf("missing --task")
	}
	if opts.storePath == "" {
		return agentsAssignOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsSessionsListOptions(args []string) (agentsInboxOptions, error) {
	return parseAgentsInboxListOptions(args)
}

func parseAgentsInboxListOptions(args []string) (agentsInboxOptions, error) {
	var opts agentsInboxOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return agentsInboxOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return agentsInboxOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		default:
			return agentsInboxOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.agentID == "" {
		return agentsInboxOptions{}, fmt.Errorf("missing --agent")
	}
	if opts.storePath == "" {
		return agentsInboxOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsInboxItemOptions(itemID string, args []string) (agentsInboxOptions, error) {
	opts := agentsInboxOptions{itemID: itemID}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return agentsInboxOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		default:
			return agentsInboxOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return agentsInboxOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseAgentsDelegateProposeOptions(args []string) (agentsDelegateProposeOptions, error) {
	var opts agentsDelegateProposeOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--parent-agent":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --parent-agent")
			}
			opts.parentAgentID = args[i+1]
			i++
		case "--child-agent":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --child-agent")
			}
			opts.childAgentID = args[i+1]
			i++
		case "--parent-work-item":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --parent-work-item")
			}
			opts.parentWorkItemID = args[i+1]
			i++
		case "--task":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --task")
			}
			opts.taskPath = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return agentsDelegateProposeOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		default:
			return agentsDelegateProposeOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.parentAgentID == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --parent-agent")
	}
	if opts.childAgentID == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --child-agent")
	}
	if opts.parentWorkItemID == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --parent-work-item")
	}
	if opts.taskPath == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --task")
	}
	if opts.reason == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --output")
	}
	if opts.storePath == "" {
		return agentsDelegateProposeOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}
