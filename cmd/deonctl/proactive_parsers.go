package main

import "fmt"

func parseDaemonStoreArg(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--store" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --store")
			}
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing --store")
}

func parseDaemonRunOnceOptions(args []string) (daemonRunOnceOptions, error) {
	var opts daemonRunOnceOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return daemonRunOnceOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--heartbeat-config":
			if i+1 >= len(args) {
				return daemonRunOnceOptions{}, fmt.Errorf("missing value for --heartbeat-config")
			}
			opts.heartbeatConfig = args[i+1]
			i++
		case "--hooks-config":
			if i+1 >= len(args) {
				return daemonRunOnceOptions{}, fmt.Errorf("missing value for --hooks-config")
			}
			opts.hooksConfig = args[i+1]
			i++
		default:
			return daemonRunOnceOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return daemonRunOnceOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

func parseSchedulesSyncOptions(args []string) (string, string, error) {
	var configPath, storePath string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --config")
			}
			configPath = args[i+1]
			i++
		case "--store":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --store")
			}
			storePath = args[i+1]
			i++
		default:
			return "", "", fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if configPath == "" {
		return "", "", fmt.Errorf("missing --config")
	}
	if storePath == "" {
		return "", "", fmt.Errorf("missing --store")
	}
	return configPath, storePath, nil
}

func parseConfigPathArg(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --config")
			}
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing --config")
}

func parseHeartbeatDryRunOptions(args []string) (string, string, string, bool, error) {
	var agentID, policyID, configPath string
	agentBusy := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				return "", "", "", false, fmt.Errorf("missing value for --agent")
			}
			agentID = args[i+1]
			i++
		case "--policy":
			if i+1 >= len(args) {
				return "", "", "", false, fmt.Errorf("missing value for --policy")
			}
			policyID = args[i+1]
			i++
		case "--config":
			if i+1 >= len(args) {
				return "", "", "", false, fmt.Errorf("missing value for --config")
			}
			configPath = args[i+1]
			i++
		case "--agent-busy":
			agentBusy = true
		default:
			return "", "", "", false, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if agentID == "" {
		return "", "", "", false, fmt.Errorf("missing --agent")
	}
	if policyID == "" {
		return "", "", "", false, fmt.Errorf("missing --policy")
	}
	if configPath == "" {
		return "", "", "", false, fmt.Errorf("missing --config")
	}
	return agentID, policyID, configPath, agentBusy, nil
}

func parseHooksPlanOptions(args []string) (string, string, error) {
	var configPath, event string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --config")
			}
			configPath = args[i+1]
			i++
		case "--event":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("missing value for --event")
			}
			event = args[i+1]
			i++
		default:
			return "", "", fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if configPath == "" {
		return "", "", fmt.Errorf("missing --config")
	}
	if event == "" {
		return "", "", fmt.Errorf("missing --event")
	}
	return configPath, event, nil
}
