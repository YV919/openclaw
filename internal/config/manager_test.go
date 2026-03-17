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
