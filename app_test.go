package main

import (
	"reflect"
	"testing"

	"openclaw_config/internal/config"
)

func TestDetectFormatFromModels(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{"claude 前缀", []string{"claude-sonnet-4-6"}, "anthropic-messages"},
		{"-cc 后缀", []string{"MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"gemini 前缀", []string{"gemini-3.1-pro-preview"}, "google-generative-ai"},
		{"gpt-5 前缀", []string{"gpt-5.2"}, "openai-responses"},
		{"默认 openai-completions", []string{"qwen-turbo"}, "openai-completions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFormatFromModels(tt.models)
			if got != tt.expected {
				t.Errorf("detectFormatFromModels(%v) = %q, want %q", tt.models, got, tt.expected)
			}
		})
	}
}

func TestParseCustomModelInputSupportsMultipleEntries(t *testing.T) {
	got := parseCustomModelInput("custom-alpha, custom-beta\ncustom-gamma，custom-beta ; custom-delta")
	want := []string{"custom-alpha", "custom-beta", "custom-gamma", "custom-delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCustomModelInput() = %v, want %v", got, want)
	}
}

func TestHasAdvancedConfigDetectsSubOrNamedOverrides(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.FullConfig
		want bool
	}{
		{"仅主模型不算高级", config.FullConfig{MainAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2"}}, false},
		{"子 Agent 单独配置算高级", config.FullConfig{SubAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2-mini"}}, true},
		{"命名 Agent 算高级", config.FullConfig{NamedAgents: []config.NamedAgentConfig{{ID: "coder"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAdvancedConfig(&tt.cfg); got != tt.want {
				t.Fatalf("hasAdvancedConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrepareQuickSetupBacksUpAndClearsAdvancedConfig(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent:   config.AgentModelConfig{Primary: "demo/gpt-5.2", Fallback: "demo/gpt-4.1"},
		SubAgent:    config.AgentModelConfig{Primary: "demo/gpt-5.2-mini"},
		NamedAgents: []config.NamedAgentConfig{{ID: "coder", Model: config.AgentModelConfig{Primary: "demo/coder"}}},
	}
	snapshot := prepareQuickSetup(cfg)
	if snapshot.SubAgent.Primary != "demo/gpt-5.2-mini" {
		t.Fatalf("snapshot.SubAgent = %+v", snapshot.SubAgent)
	}
	if cfg.SubAgent.Primary != "" {
		t.Fatalf("cfg.SubAgent should be cleared")
	}
	if cfg.NamedAgents != nil {
		t.Fatalf("cfg.NamedAgents should be nil")
	}
}

func TestRestoreQuickSetupSnapshotRestores(t *testing.T) {
	cfg := &config.FullConfig{MainAgent: config.AgentModelConfig{Primary: "new/model"}}
	snapshot := quickSetupSnapshot{
		MainAgent: config.AgentModelConfig{Primary: "old/model", Fallback: "old/fb"},
	}
	restoreQuickSetupSnapshot(cfg, snapshot)
	if cfg.MainAgent.Primary != "old/model" {
		t.Fatalf("MainAgent.Primary = %q, want %q", cfg.MainAgent.Primary, "old/model")
	}
}

func TestApplyQuickPrimaryModelSetsProviderQualifiedPrimary(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent: config.AgentModelConfig{Primary: "old/model", Fallback: "old/fb"},
		SubAgent:  config.AgentModelConfig{Primary: "sub/model"},
	}
	applyQuickPrimaryModel(cfg, "openai", "gpt-5")
	if cfg.MainAgent.Primary != "openai/gpt-5" {
		t.Fatalf("MainAgent.Primary = %q", cfg.MainAgent.Primary)
	}
	if cfg.MainAgent.Fallback != "" {
		t.Fatalf("MainAgent.Fallback should be empty")
	}
}

func TestAppendUniqueStrings(t *testing.T) {
	got := appendUniqueStrings([]string{"a", "b"}, "c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendUniqueStrings = %v, want %v", got, want)
	}
	got = appendUniqueStrings([]string{"a", "b"}, "b")
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("should not add duplicate")
	}
}
