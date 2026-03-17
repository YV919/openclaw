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

func TestSaveFullConfig_DeletesRemovedNamedAgent(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 预写一个有两个 named agent 的配置
	initial := map[string]any{
		"agents": map[string]any{
			"list": []any{
				map[string]any{"id": "agent-a", "model": map[string]any{"primary": "p/m"}},
				map[string]any{"id": "agent-b", "model": map[string]any{"primary": "p/m"}},
				map[string]any{"id": "no-model-agent"}, // 无 model 字段，非本工具管理
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), initial)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	// 只保留 agent-a，删除 agent-b
	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "dmxapi", BaseUrl: "https://api.dmxapi.cn/v1", ApiKey: "sk-x",
				Models: []string{"claude-opus-4-6"}, ApiFormat: "anthropic-messages"},
		},
		MainAgent: AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"},
		NamedAgents: []NamedAgentConfig{
			{ID: "agent-a", Model: AgentModelConfig{Primary: "p/m"}},
		},
	}
	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	// 重新读取，验证 agent-b 已被删除，agent-a 和无 model 的条目保留
	raw, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	agents := raw["agents"].(map[string]any)
	list := agents["list"].([]any)

	ids := map[string]bool{}
	for _, item := range list {
		m := item.(map[string]any)
		ids[m["id"].(string)] = true
	}

	if !ids["agent-a"] {
		t.Error("expected agent-a to be preserved")
	}
	if ids["agent-b"] {
		t.Error("expected agent-b to be deleted")
	}
	if !ids["no-model-agent"] {
		t.Error("expected no-model-agent (external) to be preserved")
	}
}

func TestSaveFullConfig_DeletesAllNamedAgents(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	initial := map[string]any{
		"agents": map[string]any{
			"list": []any{
				map[string]any{"id": "agent-x", "model": map[string]any{"primary": "p/m"}},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), initial)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]any{
		"version":  1,
		"profiles": map[string]any{},
	})

	// 删除所有命名 agent
	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "dmxapi", BaseUrl: "https://api.dmxapi.cn/v1", ApiKey: "sk-x",
				Models: []string{"claude-opus-4-6"}, ApiFormat: "anthropic-messages"},
		},
		MainAgent:   AgentModelConfig{Primary: "dmxapi/claude-opus-4-6"},
		NamedAgents: []NamedAgentConfig{},
	}
	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	raw, err := cm.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	agents := raw["agents"].(map[string]any)
	list, _ := agents["list"].([]any)
	for _, item := range list {
		m := item.(map[string]any)
		if _, hasModel := m["model"]; hasModel {
			t.Errorf("expected no managed agents in list, found: %v", m["id"])
		}
	}
}

func TestSaveFullConfig_CleansOrphanedAuthProfiles(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 预写含两个 provider 的 auth-profiles
	authPath := filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile)
	initialAuth := map[string]any{
		"version": 1,
		"profiles": map[string]any{
			"provider-a:default": map[string]any{"type": "api_key", "provider": "provider-a", "key": "sk-a"},
			"provider-b:default": map[string]any{"type": "api_key", "provider": "provider-b", "key": "sk-b"},
		},
	}
	writeJSON(t, authPath, initialAuth)

	// 只保留 provider-a，删除 provider-b
	cfg := &FullConfig{
		Providers: []ProviderConfig{
			{Name: "provider-a", BaseUrl: "https://a.example.com/v1", ApiKey: "sk-a", Models: []string{"m1"}, ApiFormat: "openai-completions"},
		},
	}
	if err := cm.SaveFullConfig(cfg); err != nil {
		t.Fatalf("SaveFullConfig failed: %v", err)
	}

	// 重新读取 auth-profiles
	authData, err := cm.LoadAuthProfiles()
	if err != nil {
		t.Fatalf("LoadAuthProfiles failed: %v", err)
	}
	profiles := authData["profiles"].(map[string]any)

	if _, ok := profiles["provider-b:default"]; ok {
		t.Error("expected provider-b:default to be deleted from auth-profiles")
	}
	if _, ok := profiles["provider-a:default"]; !ok {
		t.Error("expected provider-a:default to be preserved")
	}
}

func TestListAgentIDsFromDisk(t *testing.T) {
	// 目录不存在时应返回空切片
	cm := &ConfigManager{homeDir: t.TempDir()}
	got := cm.ListAgentIDsFromDisk()
	if len(got) != 0 {
		t.Errorf("期望空切片，得到 %v", got)
	}

	// 建立目录结构
	base := filepath.Join(cm.homeDir, OpenClawDir, "agents")
	for _, d := range []string{"main", "feishu", "my-coder"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// 建一个普通文件（不是目录），应被忽略
	if err := os.WriteFile(filepath.Join(base, "not-a-dir.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	got = cm.ListAgentIDsFromDisk()

	// main 应被过滤
	for _, id := range got {
		if id == "main" {
			t.Errorf("main 应被过滤，但出现在结果中")
		}
	}
	// feishu 和 my-coder 应存在
	gotMap := map[string]bool{}
	for _, id := range got {
		gotMap[id] = true
	}
	for _, want := range []string{"feishu", "my-coder"} {
		if !gotMap[want] {
			t.Errorf("期望 %q 在结果中，但未找到", want)
		}
	}
	// 普通文件不应出现
	if gotMap["not-a-dir.txt"] {
		t.Error("文件 not-a-dir.txt 不应出现在结果中")
	}
}

func TestLoadFullConfigMergesDiskAgents(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 建立磁盘目录：feishu 仅在磁盘，my-coder 在两处都有
	for _, d := range []string{"feishu", "my-coder"} {
		if err := os.MkdirAll(filepath.Join(cm.homeDir, OpenClawDir, "agents", d), 0700); err != nil {
			t.Fatal(err)
		}
	}

	// 写最小 openclaw.json（my-coder 在 agents.list，feishu 不在）
	writeJSON(t, filepath.Join(ocDir, ConfigFile), map[string]any{
		"models": map[string]any{
			"providers": map[string]any{
				"dmxapi": map[string]any{
					"baseUrl": "https://x.com/v1",
					"apiKey":  "sk-x",
					"api":     "openai-completions",
					"models": []any{
						map[string]any{
							"id": "gpt-4o", "name": "gpt-4o",
							"reasoning": false, "input": []string{"text"},
							"cost":          map[string]any{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
							"contextWindow": 200000, "maxTokens": 8192,
						},
					},
				},
			},
		},
		"agents": map[string]any{
			"list": []any{
				map[string]any{"id": "my-coder", "model": map[string]any{"primary": "dmxapi/gpt-4o"}},
			},
		},
	})

	cfg, _, err := cm.LoadFullConfig()
	if err != nil {
		t.Fatalf("LoadFullConfig 失败: %v", err)
	}

	idMap := map[string]bool{}
	for _, na := range cfg.NamedAgents {
		idMap[na.ID] = true
	}

	// feishu 来自磁盘，应被合并
	if !idMap["feishu"] {
		t.Error("feishu 应从磁盘发现并合并，但不在 NamedAgents 中")
	}
	// my-coder 不应重复
	count := 0
	for _, na := range cfg.NamedAgents {
		if na.ID == "my-coder" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("my-coder 应恰好出现一次，实际出现 %d 次", count)
	}
	// main 不应出现
	if idMap["main"] {
		t.Error("main 不应出现在 NamedAgents 中")
	}
}

func TestExtractNamedAgentsFiltersMain(t *testing.T) {
	raw := map[string]any{
		"agents": map[string]any{
			"list": []any{
				map[string]any{"id": "main"},
				map[string]any{"id": "feishu"},
			},
		},
	}
	result := extractNamedAgents(raw)
	for _, na := range result {
		if na.ID == "main" {
			t.Error("main 不应出现在命名 Agent 列表中")
		}
	}
	if len(result) != 1 || result[0].ID != "feishu" {
		t.Errorf("期望只有 feishu，得到 %v", result)
	}
}
