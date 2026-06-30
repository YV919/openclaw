package main

import (
	"strings"

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

func appendUniqueStrings(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func parseCustomModelInput(input string) []string {
	normalized := strings.NewReplacer(
		"；", ",",
		";", ",",
		"，", ",",
		"\r\n", "\n",
		"\r", "\n",
		"\n", ",",
	).Replace(input)

	parts := strings.Split(normalized, ",")
	var models []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			models = appendUniqueStrings(models, trimmed)
		}
	}
	return models
}
