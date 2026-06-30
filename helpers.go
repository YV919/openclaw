package main

import (
	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
)

func cloneProviders(providers []config.ProviderConfig) []config.ProviderConfig {
	if len(providers) == 0 {
		return nil
	}
	cloned := make([]config.ProviderConfig, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		if len(provider.Models) > 0 {
			cloned[i].Models = append([]string(nil), provider.Models...)
		}
	}
	return cloned
}

func cloneNamedAgents(agents []config.NamedAgentConfig) []config.NamedAgentConfig {
	if len(agents) == 0 {
		return nil
	}
	cloned := make([]config.NamedAgentConfig, len(agents))
	copy(cloned, agents)
	return cloned
}

// containsOptValue 检查 opts 中是否存在 value 对应的选项
func containsOptValue(opts []huh.Option[string], value string) bool {
	for _, o := range opts {
		if o.Value == value {
			return true
		}
	}
	return false
}

func appendUniqueStrings(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
