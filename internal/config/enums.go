package config

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func IsKnownDomain(domain string) bool {
	return contains(KnownDomains, domain)
}

func IsKnownWorker(worker string) bool {
	return contains(KnownWorkers, worker)
}

func IsKnownMode(mode string) bool {
	return contains(KnownModes, mode)
}

func IsKnownWorkspaceStrategy(strategy string) bool {
	return contains(KnownWorkspaceStrategies, strategy)
}

func IsKnownMemoryScope(scope string) bool {
	return contains(KnownMemoryScopes, scope)
}
