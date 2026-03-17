package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestHome 创建临时目录模拟 ~/.openclaw 结构，返回 ConfigManager 和清理函数
func setupTestHome(t *testing.T) (*ConfigManager, string, func()) {
	t.Helper()
	dir := t.TempDir()
	ocDir := filepath.Join(dir, ".openclaw")
	if err := os.MkdirAll(filepath.Join(ocDir, AuthProfilesDir), 0700); err != nil {
		t.Fatal(err)
	}
	cm := &ConfigManager{homeDir: dir}
	return cm, ocDir, func() {} // TempDir 自动清理
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFullConfig_FileNotExist(t *testing.T) {
	cm, _, _ := setupTestHome(t)
	cfg, logs, err := cm.LoadFullConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil FullConfig")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(cfg.Providers))
	}
	if len(logs) != 0 {
		t.Errorf("expected no fix logs for empty config, got %v", logs)
	}
}

func TestLoadFullConfig_ReadsProviders(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	raw := map[string]any{
		"models": map[string]any{
			"providers": map[string]any{
				"dmxapi": map[string]any{
					"baseUrl": "https://api.dmxapi.cn/v1",
					"apiKey":  "sk-test",
					"api":     "anthropic-messages",
					"models": []any{
						map[string]any{"id": "claude-opus-4-6"},
					},
				},
			},
		},
		"agents": map[string]any{
			"defaults": map[string]any{
				"model": map[string]any{
					"primary": "dmxapi/claude-opus-4-6",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), raw)

	// auth-profiles
	authRaw := map[string]any{
		"version": 1,
		"profiles": map[string]any{
			"dmxapi:default": map[string]any{
				"type":     "api_key",
				"provider": "dmxapi",
				"key":      "sk-test",
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), authRaw)

	cfg, _, err := cm.LoadFullConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].Name != "dmxapi" {
		t.Errorf("expected name dmxapi, got %q", cfg.Providers[0].Name)
	}
	if cfg.MainAgent.Primary != "dmxapi/claude-opus-4-6" {
		t.Errorf("expected primary model, got %q", cfg.MainAgent.Primary)
	}
}

func TestLoadFullConfig_AppliesMigration(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	raw := map[string]any{
		"models": map[string]any{
			"providers": map[string]any{
				"dmxapi": map[string]any{
					"baseUrl": "https://www.dmxapi.cn", // 旧 URL
					"apiKey":  "sk-test",
					"api":     "anthropic-messages",
					"models": []any{
						map[string]any{"id": "claude-opus-4-6"},
					},
				},
			},
		},
		"agents": map[string]any{
			"defaults": map[string]any{
				"model": map[string]any{
					"primary": "dmxapi/claude-opus-4-6",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), raw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	cfg, logs, err := cm.LoadFullConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Providers[0].BaseUrl != "https://www.dmxapi.cn/v1" {
		t.Errorf("expected URL to be migrated, got %q", cfg.Providers[0].BaseUrl)
	}
	if len(logs) == 0 {
		t.Error("expected migration logs")
	}
}

func TestSaveFullConfig_RejectsEmptyProviders(t *testing.T) {
	cm, _, _ := setupTestHome(t)
	err := cm.SaveFullConfig(&FullConfig{})
	if err == nil {
		t.Fatal("expected error for empty providers")
	}
}

func TestSaveFullConfig_RejectsEmptyModels(t *testing.T) {
	cm, _, _ := setupTestHome(t)
	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "dmxapi", BaseUrl: "https://api.dmxapi.cn/v1", ApiKey: "sk-test",
				Models: []string{}, ApiFormat: "anthropic-messages"},
		},
	}
	err := cm.SaveFullConfig(cfg)
	if err == nil {
		t.Fatal("expected error for provider with empty models")
	}
}

func TestSaveFullConfig_WritesAndReloads(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 先写一个基础 openclaw.json（模拟 onboard 后状态）
	baseRaw := map[string]any{
		"gateway": map[string]any{"port": 18789},
		"tools":   map[string]any{"profile": "coding"},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{
				Name:      "dmxapi",
				BaseUrl:   "https://api.dmxapi.cn/v1",
				ApiKey:    "sk-test-key",
				Models:    []string{"claude-opus-4-6", "claude-sonnet-4-6"},
				ApiFormat: "anthropic-messages",
			},
		},
		MainAgent: AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"},
		SubAgent:  AgentModelConfig{Primary: "dmxapi/claude-sonnet-4-6"},
	}

	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	// 重新加载验证
	reloaded, _, err := cm.LoadFullConfig()
	if err != nil {
		t.Fatalf("LoadFullConfig after save failed: %v", err)
	}
	if len(reloaded.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(reloaded.Providers))
	}
	if reloaded.Providers[0].ApiKey != "sk-test-key" {
		t.Errorf("expected api key, got %q", reloaded.Providers[0].ApiKey)
	}
	if reloaded.MainAgent.Primary != "dmxapi/claude-opus-4-6" {
		t.Errorf("expected main agent primary, got %q", reloaded.MainAgent.Primary)
	}
	if reloaded.SubAgent.Primary != "dmxapi/claude-sonnet-4-6" {
		t.Errorf("expected sub agent primary, got %q", reloaded.SubAgent.Primary)
	}
}

func TestSaveFullConfig_PreservesUnrelatedFields(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	baseRaw := map[string]any{
		"gateway": map[string]any{"port": 18789},
		"tools":   map[string]any{"profile": "coding"},
		"session": map[string]any{"dmScope": "per-channel-peer"},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "dmxapi", BaseUrl: "https://api.dmxapi.cn/v1", ApiKey: "sk-x",
				Models: []string{"claude-opus-4-6"}, ApiFormat: "anthropic-messages"},
		},
		MainAgent: AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"},
	}
	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	// 读取原始 JSON 检查无关字段是否保留
	data, _ := os.ReadFile(filepath.Join(ocDir, ConfigFile))
	var saved map[string]any
	json.Unmarshal(data, &saved)

	if gateway, ok := saved["gateway"].(map[string]any); !ok || gateway["port"] != float64(18789) {
		t.Error("gateway field should be preserved")
	}
	if tools, ok := saved["tools"].(map[string]any); !ok || tools["profile"] != "coding" {
		t.Error("tools field should be preserved")
	}
}

func TestSaveFullConfig_NamedAgentUpsert(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 原始 config 含一个 agents.list 条目（其他工具管理）
	baseRaw := map[string]any{
		"agents": map[string]any{
			"list": []any{
				map[string]any{
					"id":   "existing-agent",
					"name": "Existing",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "dmxapi", BaseUrl: "https://api.dmxapi.cn/v1", ApiKey: "sk-x",
				Models: []string{"claude-opus-4-6"}, ApiFormat: "anthropic-messages"},
		},
		MainAgent: AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"},
		NamedAgents: []NamedAgentConfig{
			{ID: "my-coder", Model: AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"}},
		},
	}
	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	// 验证 existing-agent 未被删除，my-coder 已添加
	data, _ := os.ReadFile(filepath.Join(ocDir, ConfigFile))
	var saved map[string]any
	json.Unmarshal(data, &saved)

	agents := saved["agents"].(map[string]any)
	list := agents["list"].([]any)

	ids := map[string]bool{}
	for _, item := range list {
		m := item.(map[string]any)
		ids[m["id"].(string)] = true
	}
	if !ids["existing-agent"] {
		t.Error("existing-agent should be preserved")
	}
	if !ids["my-coder"] {
		t.Error("my-coder should be added")
	}
}
