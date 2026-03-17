# Multi-Provider Agent Model Config Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 openclaw_config 从单一 dmxapi provider + 单模型扩展为支持多 provider、多模型、主/子/命名 agent 分别配置的 4 步向导工具。

**Architecture:** 新增 `FullConfig` 内部视图模型，通过 `LoadFullConfig`/`SaveFullConfig` 与 openclaw.json + auth-profiles.json + models.json 解耦。`migration.go` 负责向后兼容性检测与修复，`app.go` 完全重写为 4 步 huh 向导。

**Tech Stack:** Go 1.23, charmbracelet/huh v0.6.0, charmbracelet/lipgloss v0.13.0, 标准库 `testing` + `encoding/json` + `os` + `net/url` + `regexp`

**Spec:** `docs/superpowers/specs/2026-03-17-multi-provider-agent-model-config-design.md`

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/config/types.go` | 修改（追加） | 新增 `ProviderConfig`、`AgentModelConfig`、`NamedAgentConfig`、`FullConfig` |
| `internal/config/migration.go` | 新建 | slug 规范化、各项兼容性检测与修复 |
| `internal/config/migration_test.go` | 新建 | migration 逻辑单元测试 |
| `internal/config/manager.go` | 修改（追加） | 新增 `LoadFullConfig`、`SaveFullConfig`；重构 `UpdateModelsJson` 支持多 provider |
| `internal/config/manager_test.go` | 新建 | LoadFullConfig/SaveFullConfig 集成测试（操作临时目录） |
| `app.go` | 重写 | 4 步向导：provider 管理、主 agent、子 agent、命名 agent |

---

## Task 1: 新增类型定义

**Files:**
- Modify: `internal/config/types.go`

- [ ] **Step 1.1: 在 types.go 末尾追加新结构体**

打开 `internal/config/types.go`，在文件末尾追加：

```go
// ProviderConfig 单个 API provider 的完整配置
type ProviderConfig struct {
	Name      string   // provider 唯一标识 slug，如 "dmxapi"、"my-proxy"
	BaseUrl   string
	ApiKey    string
	Models    []string // 该 provider 下注册的模型 ID 列表（不含 provider 前缀）
	ApiFormat string   // openai-completions / anthropic-messages / openai-responses / google-generative-ai
}

// AgentModelConfig 某个 agent 的模型分配
type AgentModelConfig struct {
	Primary  string // 完整模型 ID，格式 "provider/model-id"；空表示未配置（沿用默认）
	Fallback string // 单个 fallback；空表示不配置
}

// NamedAgentConfig 命名 agent 配置
type NamedAgentConfig struct {
	ID    string           // agent id，如 "my-coder"
	Model AgentModelConfig
}

// FullConfig 工具内部视图（不对应单一文件格式）
type FullConfig struct {
	Providers   []ProviderConfig
	MainAgent   AgentModelConfig   // agents.defaults.model
	SubAgent    AgentModelConfig   // agents.defaults.subagents.model；Primary 空 = 同主 agent
	NamedAgents []NamedAgentConfig // agents.list 中本工具管理的条目
}
```

- [ ] **Step 1.2: 确认编译通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

Expected: 无错误输出

- [ ] **Step 1.3: Commit**

```bash
git add internal/config/types.go
git commit -m "feat: 新增 FullConfig 相关类型定义"
```

---

## Task 2: 实现 migration.go（兼容性检测与修复）

**Files:**
- Create: `internal/config/migration.go`
- Create: `internal/config/migration_test.go`

### 2.1 实现 migration.go

- [ ] **Step 2.1: 写入 migration.go**

创建 `internal/config/migration.go`：

```go
package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	slugInvalid   = regexp.MustCompile(`[^a-z0-9-]+`)
	slugMultiDash = regexp.MustCompile(`-{2,}`)
)

// NormalizeSlug 将任意字符串转为合法的 provider slug（小写字母、数字、连字符）
func NormalizeSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "-")
	s = slugInvalid.ReplaceAllString(s, "-")
	s = slugMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// SlugFromURL 从 BaseUrl 的 hostname 生成 slug
func SlugFromURL(baseUrl string) string {
	u, err := url.Parse(baseUrl)
	if err != nil || u.Hostname() == "" {
		return NormalizeSlug(baseUrl)
	}
	return NormalizeSlug(u.Hostname())
}

// MigrateProviders 对从 openclaw.json 解析出的 provider map 执行兼容性检测与修复。
// 返回修复后的 []ProviderConfig 和每次修复的描述日志。
// primary 参数是 agents.defaults.model.primary 的值，用于 Models 为空时反推模型 ID。
func MigrateProviders(
	rawProviders map[string]interface{},
	primary string,
) ([]ProviderConfig, []string) {
	var providers []ProviderConfig
	var logs []string

	for key, val := range rawProviders {
		pMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}

		name := key

		// 修复 1：provider key 含非法字符或大写 → 规范化
		normalized := NormalizeSlug(name)
		if normalized == "" {
			// key 规范化后为空，从 baseUrl 生成
			rawBaseUrl, _ := pMap["baseUrl"].(string)
			normalized = SlugFromURL(rawBaseUrl)
			if normalized == "" {
				normalized = "provider"
			}
			logs = append(logs, fmt.Sprintf("provider key %q 规范化失败，已重命名为 %q", key, normalized))
		} else if normalized != name {
			logs = append(logs, fmt.Sprintf("provider key %q 含非法字符，已规范化为 %q", key, normalized))
		}
		name = normalized

		baseUrl, _ := pMap["baseUrl"].(string)
		apiKey, _ := pMap["apiKey"].(string)
		apiFormat, _ := pMap["api"].(string)

		// 修复 2：旧默认 URL（无 /v1）
		if baseUrl == "https://www.dmxapi.cn" {
			baseUrl = "https://www.dmxapi.cn/v1"
			logs = append(logs, fmt.Sprintf("provider %q 的 baseUrl 已补全 /v1 路径", name))
		}

		// 解析已有模型列表
		var modelIDs []string
		if rawModels, ok := pMap["models"].([]interface{}); ok {
			for _, m := range rawModels {
				if mMap, ok := m.(map[string]interface{}); ok {
					if id, ok := mMap["id"].(string); ok && id != "" {
						modelIDs = append(modelIDs, id)
					}
				}
			}
		}

		// 修复 3：models 数组为空 → 从 primary 反推
		if len(modelIDs) == 0 && primary != "" {
			// primary 格式为 "providerName/modelId"
			prefix := name + "/"
			if strings.HasPrefix(primary, prefix) {
				inferredID := primary[len(prefix):]
				modelIDs = append(modelIDs, inferredID)
				logs = append(logs, fmt.Sprintf("provider %q 的模型列表为空，已从 primary 推断补全: %q", name, inferredID))
			}
		}

		if apiFormat == "" && len(modelIDs) > 0 {
			apiFormat = DetectAPIFormat(modelIDs[0])
		}
		if apiFormat == "" {
			apiFormat = "openai-completions"
		}

		providers = append(providers, ProviderConfig{
			Name:      name,
			BaseUrl:   baseUrl,
			ApiKey:    apiKey,
			Models:    modelIDs,
			ApiFormat: apiFormat,
		})
	}

	return providers, logs
}
```

- [ ] **Step 2.2: 编译确认**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

Expected: 无错误

### 2.2 编写 migration 测试

- [ ] **Step 2.3: 写 migration_test.go**

创建 `internal/config/migration_test.go`：

```go
package config

import (
	"testing"
)

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dmxapi", "dmxapi"},
		{"MyProxy", "myproxy"},
		{"my proxy", "my-proxy"},
		{"my.proxy.com", "my-proxy-com"},
		{"api.example.com:8080", "api-example-com-8080"},
		{"  --bad-- ", "bad"},
		{"", ""},
	}
	for _, tc := range tests {
		got := NormalizeSlug(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://api.dmxapi.cn/v1", "api-dmxapi-cn"},
		{"https://192.168.1.1:8080/v1", "192-168-1-1"},
		{"https://my.proxy.com", "my-proxy-com"},
		{"not-a-url", "not-a-url"},
	}
	for _, tc := range tests {
		got := SlugFromURL(tc.input)
		if got != tc.want {
			t.Errorf("SlugFromURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMigrateProviders_NormalizesKey(t *testing.T) {
	raw := map[string]interface{}{
		"My Provider": map[string]interface{}{
			"baseUrl": "https://api.example.com/v1",
			"apiKey":  "sk-test",
			"api":     "openai-completions",
			"models": []interface{}{
				map[string]interface{}{"id": "gpt-4o"},
			},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name != "my-provider" {
		t.Errorf("expected name %q, got %q", "my-provider", providers[0].Name)
	}
	if len(logs) == 0 {
		t.Error("expected migration log entry for key normalization")
	}
}

func TestMigrateProviders_FixesOldDefaultURL(t *testing.T) {
	raw := map[string]interface{}{
		"dmxapi": map[string]interface{}{
			"baseUrl": "https://www.dmxapi.cn",
			"apiKey":  "sk-test",
			"api":     "anthropic-messages",
			"models": []interface{}{
				map[string]interface{}{"id": "claude-opus-4-6"},
			},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if providers[0].BaseUrl != "https://www.dmxapi.cn/v1" {
		t.Errorf("expected URL to be fixed, got %q", providers[0].BaseUrl)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for URL fix")
	}
}

func TestMigrateProviders_InfersModelsFromPrimary(t *testing.T) {
	raw := map[string]interface{}{
		"dmxapi": map[string]interface{}{
			"baseUrl": "https://api.dmxapi.cn/v1",
			"apiKey":  "sk-test",
			"api":     "anthropic-messages",
			"models":  []interface{}{}, // 空
		},
	}
	providers, logs := MigrateProviders(raw, "dmxapi/claude-opus-4-6")
	if len(providers[0].Models) != 1 || providers[0].Models[0] != "claude-opus-4-6" {
		t.Errorf("expected models to be inferred, got %v", providers[0].Models)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for model inference")
	}
}

func TestMigrateProviders_EmptyModels_NoInference(t *testing.T) {
	raw := map[string]interface{}{
		"dmxapi": map[string]interface{}{
			"baseUrl": "https://api.dmxapi.cn/v1",
			"apiKey":  "sk-test",
			"api":     "openai-completions",
			"models":  []interface{}{},
		},
	}
	// primary 来自不同 provider，不应推断
	providers, _ := MigrateProviders(raw, "other/some-model")
	if len(providers[0].Models) != 0 {
		t.Errorf("expected no models inferred, got %v", providers[0].Models)
	}
}
```

- [ ] **Step 2.4: 运行测试（预期全部通过）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestNormalize -v
go test ./internal/config/... -run TestMigrateProviders -v
```

Expected: 所有测试 PASS

- [ ] **Step 2.5: Commit**

```bash
git add internal/config/migration.go internal/config/migration_test.go
git commit -m "feat: 实现 migration.go — provider slug 规范化与兼容性修复"
```

---

## Task 3: 实现 LoadFullConfig

**Files:**
- Modify: `internal/config/manager.go`（追加方法）
- Create: `internal/config/manager_test.go`

- [ ] **Step 3.1: 写 LoadFullConfig 测试（先写测试）**

创建 `internal/config/manager_test.go`：

```go
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

func writeJSON(t *testing.T, path string, v interface{}) {
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
	if logs != nil && len(logs) != 0 {
		t.Errorf("expected no fix logs for empty config, got %v", logs)
	}
}

func TestLoadFullConfig_ReadsProviders(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	raw := map[string]interface{}{
		"models": map[string]interface{}{
			"providers": map[string]interface{}{
				"dmxapi": map[string]interface{}{
					"baseUrl": "https://api.dmxapi.cn/v1",
					"apiKey":  "sk-test",
					"api":     "anthropic-messages",
					"models": []interface{}{
						map[string]interface{}{"id": "claude-opus-4-6"},
					},
				},
			},
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": map[string]interface{}{
					"primary": "dmxapi/claude-opus-4-6",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), raw)

	// auth-profiles
	authRaw := map[string]interface{}{
		"version": 1,
		"profiles": map[string]interface{}{
			"dmxapi:default": map[string]interface{}{
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

	raw := map[string]interface{}{
		"models": map[string]interface{}{
			"providers": map[string]interface{}{
				"dmxapi": map[string]interface{}{
					"baseUrl": "https://www.dmxapi.cn", // 旧 URL
					"apiKey":  "sk-test",
					"api":     "anthropic-messages",
					"models": []interface{}{
						map[string]interface{}{"id": "claude-opus-4-6"},
					},
				},
			},
		},
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model": map[string]interface{}{
					"primary": "dmxapi/claude-opus-4-6",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), raw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]interface{}{
		"version":  1,
		"profiles": map[string]interface{}{},
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
```

- [ ] **Step 3.2: 运行测试（预期失败 — LoadFullConfig 未实现）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestLoadFullConfig -v 2>&1 | head -20
```

Expected: 编译错误 "undefined: (*ConfigManager).LoadFullConfig"

- [ ] **Step 3.3: 在 manager.go 追加 LoadFullConfig 实现**

在 `internal/config/manager.go` 末尾追加：

```go
// LoadFullConfig 读取并解析为工具视图，执行兼容性检测（不写文件）。
// fixLogs: 每条描述一处自动修复，调用方负责展示。
// 若 openclaw.json 不存在，返回空 FullConfig（非错误）。
func (cm *ConfigManager) LoadFullConfig() (*FullConfig, []string, error) {
	cfg := &FullConfig{}

	// 先用 os.Stat 判断文件是否存在，避免依赖错误字符串匹配
	if _, err := os.Stat(cm.GetConfigPath()); os.IsNotExist(err) {
		return cfg, nil, nil
	}

	raw, err := cm.LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	// 提取 primary 用于 migration 推断
	primary := extractPrimary(raw)

	// 解析并迁移 providers
	var rawProviders map[string]interface{}
	if models, ok := raw["models"].(map[string]interface{}); ok {
		rawProviders, _ = models["providers"].(map[string]interface{})
	}
	providers, logs := MigrateProviders(rawProviders, primary)

	// 从 auth-profiles.json 补充 ApiKey（覆盖 openclaw.json 中的 apiKey）
	authProfiles, err := cm.LoadAuthProfiles()
	if err == nil {
		if profiles, ok := authProfiles["profiles"].(map[string]interface{}); ok {
			for i, p := range providers {
				profileKey := p.Name + ":default"
				if prof, ok := profiles[profileKey].(map[string]interface{}); ok {
					if key, ok := prof["key"].(string); ok && key != "" {
						providers[i].ApiKey = key
					}
				}
			}
		}
	}

	cfg.Providers = providers

	// 主 agent 模型
	cfg.MainAgent.Primary = primary
	if fa := extractFallback(raw, "agents", "defaults", "model"); fa != "" {
		cfg.MainAgent.Fallback = fa
	}

	// 子 agent 模型
	if subPrimary := extractSubagentPrimary(raw); subPrimary != "" {
		cfg.SubAgent.Primary = subPrimary
		if subFallback := extractSubagentFallback(raw); subFallback != "" {
			cfg.SubAgent.Fallback = subFallback
		}
	}

	// 命名 agent
	cfg.NamedAgents = extractNamedAgents(raw)

	return cfg, logs, nil
}

func extractPrimary(raw map[string]interface{}) string {
	agents, ok := raw["agents"].(map[string]interface{})
	if !ok {
		return ""
	}
	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		return ""
	}
	model := defaults["model"]
	switch v := model.(type) {
	case string:
		return v
	case map[string]interface{}:
		p, _ := v["primary"].(string)
		return p
	}
	return ""
}

func extractFallback(raw map[string]interface{}, keys ...string) string {
	cur := interface{}(raw)
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = m[k]
	}
	switch v := cur.(type) {
	case map[string]interface{}:
		if fallbacks, ok := v["fallbacks"].([]interface{}); ok && len(fallbacks) > 0 {
			s, _ := fallbacks[0].(string)
			return s
		}
	}
	return ""
}

func extractSubagentPrimary(raw map[string]interface{}) string {
	agents, ok := raw["agents"].(map[string]interface{})
	if !ok {
		return ""
	}
	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		return ""
	}
	subagents, ok := defaults["subagents"].(map[string]interface{})
	if !ok {
		return ""
	}
	model := subagents["model"]
	switch v := model.(type) {
	case string:
		return v
	case map[string]interface{}:
		p, _ := v["primary"].(string)
		return p
	}
	return ""
}

func extractSubagentFallback(raw map[string]interface{}) string {
	agents, ok := raw["agents"].(map[string]interface{})
	if !ok {
		return ""
	}
	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		return ""
	}
	subagents, ok := defaults["subagents"].(map[string]interface{})
	if !ok {
		return ""
	}
	model, ok := subagents["model"].(map[string]interface{})
	if !ok {
		return ""
	}
	if fallbacks, ok := model["fallbacks"].([]interface{}); ok && len(fallbacks) > 0 {
		s, _ := fallbacks[0].(string)
		return s
	}
	return ""
}

func extractNamedAgents(raw map[string]interface{}) []NamedAgentConfig {
	agents, ok := raw["agents"].(map[string]interface{})
	if !ok {
		return nil
	}
	list, ok := agents["list"].([]interface{})
	if !ok {
		return nil
	}
	var result []NamedAgentConfig
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		na := NamedAgentConfig{ID: id}
		if model, ok := m["model"].(map[string]interface{}); ok {
			na.Model.Primary, _ = model["primary"].(string)
			if fallbacks, ok := model["fallbacks"].([]interface{}); ok && len(fallbacks) > 0 {
				na.Model.Fallback, _ = fallbacks[0].(string)
			}
		} else if modelStr, ok := m["model"].(string); ok {
			na.Model.Primary = modelStr
		}
		result = append(result, na)
	}
	return result
}
```

需要在 manager.go 顶部 import 中补充 `"strings"`（如果尚未包含）。

- [ ] **Step 3.4: 运行测试（预期全部通过）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestLoadFullConfig -v
```

Expected: 所有 TestLoadFullConfig_* 测试 PASS

- [ ] **Step 3.5: Commit**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat: 实现 LoadFullConfig — 读取多 provider 配置并执行兼容性迁移"
```

---

## Task 4: 实现 SaveFullConfig

**Files:**
- Modify: `internal/config/manager.go`（追加方法）
- Modify: `internal/config/manager_test.go`（追加测试）

- [ ] **Step 4.1: 追加 SaveFullConfig 测试到 manager_test.go**

在 `internal/config/manager_test.go` 末尾追加：

```go
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
	baseRaw := map[string]interface{}{
		"gateway": map[string]interface{}{"port": 18789},
		"tools":   map[string]interface{}{"profile": "coding"},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]interface{}{
		"version":  1,
		"profiles": map[string]interface{}{},
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

	baseRaw := map[string]interface{}{
		"gateway": map[string]interface{}{"port": 18789},
		"tools":   map[string]interface{}{"profile": "coding"},
		"session": map[string]interface{}{"dmScope": "per-channel-peer"},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]interface{}{
		"version":  1,
		"profiles": map[string]interface{}{},
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
	var saved map[string]interface{}
	json.Unmarshal(data, &saved)

	if gateway, ok := saved["gateway"].(map[string]interface{}); !ok || gateway["port"] != float64(18789) {
		t.Error("gateway field should be preserved")
	}
	if tools, ok := saved["tools"].(map[string]interface{}); !ok || tools["profile"] != "coding" {
		t.Error("tools field should be preserved")
	}
}

func TestSaveFullConfig_NamedAgentUpsert(t *testing.T) {
	cm, ocDir, _ := setupTestHome(t)

	// 原始 config 含一个 agents.list 条目（其他工具管理）
	baseRaw := map[string]interface{}{
		"agents": map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{
					"id":   "existing-agent",
					"name": "Existing",
				},
			},
		},
	}
	writeJSON(t, filepath.Join(ocDir, ConfigFile), baseRaw)
	writeJSON(t, filepath.Join(ocDir, AuthProfilesDir, AuthProfilesFile), map[string]interface{}{
		"version":  1,
		"profiles": map[string]interface{}{},
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
	var saved map[string]interface{}
	json.Unmarshal(data, &saved)

	agents := saved["agents"].(map[string]interface{})
	list := agents["list"].([]interface{})

	ids := map[string]bool{}
	for _, item := range list {
		m := item.(map[string]interface{})
		ids[m["id"].(string)] = true
	}
	if !ids["existing-agent"] {
		t.Error("existing-agent should be preserved")
	}
	if !ids["my-coder"] {
		t.Error("my-coder should be added")
	}
}
```

- [ ] **Step 4.2: 运行测试（预期失败 — SaveFullConfig 未实现）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestSaveFullConfig -v 2>&1 | head -10
```

Expected: 编译错误 "undefined: (*ConfigManager).SaveFullConfig"

- [ ] **Step 4.3: 在 manager.go 追加 SaveFullConfig 实现**

在 `internal/config/manager.go` 末尾追加：

```go
// SaveFullConfig 将工具视图写回磁盘。
// 保留 openclaw.json 中与模型/agent 无关的字段（gateway、tools、session 等）。
func (cm *ConfigManager) SaveFullConfig(cfg *FullConfig) error {
	// 校验
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("至少需要配置一个 provider")
	}
	for _, p := range cfg.Providers {
		if len(p.Models) == 0 {
			return fmt.Errorf("provider %q 的模型列表不能为空", p.Name)
		}
		if p.ApiFormat == "" {
			return fmt.Errorf("provider %q 的 API 格式不能为空", p.Name)
		}
	}

	// 加载现有配置（保留无关字段）
	raw, err := cm.LoadConfig()
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "不存在") {
			raw = map[string]interface{}{}
		} else {
			return err
		}
	}

	// 构建 models.providers
	providersMap := map[string]interface{}{}
	for _, p := range cfg.Providers {
		modelsList := make([]interface{}, 0, len(p.Models))
		for _, id := range p.Models {
			modelsList = append(modelsList, map[string]interface{}{
				"id":            id,
				"name":          id,
				"reasoning":     false,
				"input":         []string{"text"},
				"cost":          map[string]interface{}{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
				"contextWindow": 200000,
				"maxTokens":     8192,
			})
		}
		providersMap[p.Name] = map[string]interface{}{
			"baseUrl":  p.BaseUrl,
			"apiKey":   p.ApiKey,
			"api":      p.ApiFormat,
			"models":   modelsList,
		}
	}

	models, ok := raw["models"].(map[string]interface{})
	if !ok {
		models = map[string]interface{}{}
		raw["models"] = models
	}
	models["providers"] = providersMap
	if models["mode"] == nil {
		models["mode"] = "merge"
	}

	// agents.defaults.model
	agents, ok := raw["agents"].(map[string]interface{})
	if !ok {
		agents = map[string]interface{}{}
		raw["agents"] = agents
	}
	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		defaults = map[string]interface{}{}
		agents["defaults"] = defaults
	}

	// 构建 agents.defaults.models 允许列表
	allowedModels := map[string]interface{}{}
	for _, p := range cfg.Providers {
		for _, m := range p.Models {
			fullID := p.Name + "/" + m
			allowedModels[fullID] = map[string]interface{}{"alias": m}
		}
	}
	defaults["models"] = allowedModels

	// 主 agent model
	if cfg.MainAgent.Primary != "" {
		modelField := map[string]interface{}{"primary": cfg.MainAgent.Primary}
		if cfg.MainAgent.Fallback != "" {
			modelField["fallbacks"] = []string{cfg.MainAgent.Fallback}
		}
		defaults["model"] = modelField
	}

	// 子 agent model
	subagents, ok := defaults["subagents"].(map[string]interface{})
	if !ok {
		subagents = map[string]interface{}{}
	}
	if cfg.SubAgent.Primary != "" {
		subModel := map[string]interface{}{"primary": cfg.SubAgent.Primary}
		if cfg.SubAgent.Fallback != "" {
			subModel["fallbacks"] = []string{cfg.SubAgent.Fallback}
		}
		subagents["model"] = subModel
	} else {
		// Primary 为空 = 同主 agent，清除旧有 subagents.model
		delete(subagents, "model")
	}
	defaults["subagents"] = subagents

	// agents.list upsert（按 ID，保留其他条目）
	if len(cfg.NamedAgents) > 0 {
		var existingList []interface{}
		if raw["agents"] != nil {
			if ag, ok := raw["agents"].(map[string]interface{}); ok {
				existingList, _ = ag["list"].([]interface{})
			}
		}

		// 构建 ID → index 映射
		indexByID := map[string]int{}
		for i, item := range existingList {
			if m, ok := item.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					indexByID[id] = i
				}
			}
		}

		for _, na := range cfg.NamedAgents {
			var modelField interface{}
			if na.Model.Primary != "" {
				mf := map[string]interface{}{"primary": na.Model.Primary}
				if na.Model.Fallback != "" {
					mf["fallbacks"] = []string{na.Model.Fallback}
				}
				modelField = mf
			}

			if idx, exists := indexByID[na.ID]; exists {
				// upsert: 只覆写 model 字段
				entry := existingList[idx].(map[string]interface{})
				if modelField != nil {
					entry["model"] = modelField
				} else {
					delete(entry, "model")
				}
			} else {
				// 追加新条目
				newEntry := map[string]interface{}{"id": na.ID}
				if modelField != nil {
					newEntry["model"] = modelField
				}
				existingList = append(existingList, newEntry)
			}
		}
		agents["list"] = existingList
	}

	// 保存主配置
	if err := cm.SaveConfig(raw); err != nil {
		return err
	}

	// 同步 auth-profiles.json
	if err := cm.saveAllApiKeys(cfg.Providers); err != nil {
		return err
	}

	// 同步 models.json
	cm.syncModelsJSON(cfg.Providers)

	return nil
}

// saveAllApiKeys 将所有 provider 的 ApiKey 写入 auth-profiles.json
func (cm *ConfigManager) saveAllApiKeys(providers []ProviderConfig) error {
	authProfiles, err := cm.LoadAuthProfiles()
	if err != nil {
		authProfiles = map[string]interface{}{
			"version":  1,
			"profiles": map[string]interface{}{},
		}
	}
	profiles, ok := authProfiles["profiles"].(map[string]interface{})
	if !ok {
		profiles = map[string]interface{}{}
		authProfiles["profiles"] = profiles
	}
	for _, p := range providers {
		profileKey := p.Name + ":default"
		profiles[profileKey] = map[string]interface{}{
			"type":     "api_key",
			"provider": p.Name,
			"key":      p.ApiKey,
		}
	}
	return cm.SaveAuthProfiles(authProfiles)
}

// syncModelsJSON 将所有 provider 的 baseUrl/api 同步写入 models.json
func (cm *ConfigManager) syncModelsJSON(providers []ProviderConfig) {
	modelsPath := filepath.Join(cm.homeDir, OpenClawDir, AuthProfilesDir, "models.json")
	data, err := os.ReadFile(modelsPath)
	if err != nil {
		return // 文件不存在时跳过
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	rawProviders, ok := raw["providers"].(map[string]interface{})
	if !ok {
		return
	}
	for _, p := range providers {
		if entry, ok := rawProviders[p.Name].(map[string]interface{}); ok {
			entry["baseUrl"] = p.BaseUrl
			entry["api"] = p.ApiFormat
			rawProviders[p.Name] = entry
		}
	}
	raw["providers"] = rawProviders
	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(modelsPath, append(updated, '\n'), 0600)
}
```

- [ ] **Step 4.4: 确保 manager.go 的 import 包含所有依赖**

检查 `internal/config/manager.go` 顶部 import 是否包含：
- `"encoding/json"` ✓（原有）
- `"fmt"` ✓（原有）
- `"os"` ✓（原有）
- `"path/filepath"` ✓（原有）
- `"strings"` — 若未有，需添加

- [ ] **Step 4.5: 运行测试（预期全部通过）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -v
```

Expected: 所有测试 PASS，无失败

- [ ] **Step 4.6: Commit**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat: 实现 SaveFullConfig — 多 provider 配置写入与 agents.list upsert"
```

---

## Task 5: 重写 app.go — 4 步向导

**Files:**
- Modify: `app.go`（完全重写）

> 注：`huh` 表单是交互式 TUI，无法用单元测试覆盖。手动测试步骤在最后列出。

- [ ] **Step 5.1: 重写 app.go**

用以下内容完全替换 `app.go`（保留 `package main`，保留 `printBanner`/`printSuccess` 函数签名但更新内容）：

```go
package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
)

type App struct {
	configManager *config.ConfigManager
}

func NewApp() *App {
	cm, err := config.NewConfigManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 初始化配置管理器失败: %v\n", err)
	}
	return &App{configManager: cm}
}

func (a *App) Run() error {
	if a.configManager == nil {
		cm, err := config.NewConfigManager()
		if err != nil {
			return fmt.Errorf("初始化配置管理器失败: %w", err)
		}
		a.configManager = cm
	}

	// 加载现有配置（含兼容性迁移）
	fullCfg, fixLogs, err := a.configManager.LoadFullConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	printBanner()

	// 展示兼容性修复日志
	if len(fixLogs) > 0 {
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
		fmt.Println(yellow.Render(fmt.Sprintf("  ⚠ 已自动修正 %d 处配置：", len(fixLogs))))
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		for _, log := range fixLogs {
			fmt.Println(dim.Render("    · " + log))
		}
		fmt.Println()
	}

	// Step 1: Provider 管理
	if err := a.runStep1Providers(fullCfg); err != nil {
		return err
	}

	// 构建全部模型选项（供后续步骤使用）
	allModelOpts := buildAllModelOpts(fullCfg.Providers)
	allModelOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allModelOpts...,
	)

	// Step 2: 主 Agent 模型
	if err := a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 3: 子 Agent 模型
	if err := a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 4: 命名 Agent（可选）
	if err := a.runStep4NamedAgents(fullCfg, allModelOpts); err != nil {
		return err
	}

	// 最终写入
	if err := a.configManager.SaveFullConfig(fullCfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	printSuccess(fullCfg)
	return nil
}

// buildAllModelOpts 从所有 provider 构建完整模型选项列表（格式 "provider/model"）
func buildAllModelOpts(providers []config.ProviderConfig) []huh.Option[string] {
	var opts []huh.Option[string]
	for _, p := range providers {
		for _, m := range p.Models {
			fullID := p.Name + "/" + m
			opts = append(opts, huh.NewOption(fullID, fullID))
		}
	}
	return opts
}

// ── Step 1: Provider 管理 ──────────────────────────────────────────────────

func (a *App) runStep1Providers(fullCfg *config.FullConfig) error {
	for {
		action, err := pickProviderAction(fullCfg.Providers)
		if err != nil {
			return err
		}
		if action == "__continue__" {
			break
		}
		if action == "__add__" {
			p, err := editProvider(config.ProviderConfig{})
			if err != nil {
				return err
			}
			fullCfg.Providers = append(fullCfg.Providers, p)
		} else {
			// 编辑已有 provider（action = provider name）
			for i, p := range fullCfg.Providers {
				if p.Name == action {
					updated, err := editProvider(p)
					if err != nil {
						return err
					}
					fullCfg.Providers[i] = updated
					break
				}
			}
		}
	}
	return nil
}

func pickProviderAction(providers []config.ProviderConfig) (string, error) {
	var opts []huh.Option[string]
	for _, p := range providers {
		label := fmt.Sprintf("[编辑] %s  (%s)", p.Name, p.BaseUrl)
		opts = append(opts, huh.NewOption(label, p.Name))
	}
	opts = append(opts, huh.NewOption("[+ 添加新 Provider]", "__add__"))
	if len(providers) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
	}

	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider 管理").
			Description("选择要编辑的 provider，或添加新的").
			Options(opts...).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}

var apiFormatOpts = []huh.Option[string]{
	huh.NewOption("openai-completions  (GPT / 通用兼容)", "openai-completions"),
	huh.NewOption("anthropic-messages  (Claude)", "anthropic-messages"),
	huh.NewOption("openai-responses    (GPT-5 / o 系列)", "openai-responses"),
	huh.NewOption("google-generative-ai (Gemini)", "google-generative-ai"),
}

func editProvider(p config.ProviderConfig) (config.ProviderConfig, error) {
	name := p.Name
	baseUrl := p.BaseUrl
	apiKey := p.ApiKey
	apiFormat := p.ApiFormat
	if apiFormat == "" {
		apiFormat = "openai-completions"
	}

	// 构建 MultiSelect 选项
	presetOpts := make([]huh.Option[string], 0, len(models.PresetModels)+1)
	for _, m := range models.PresetModels {
		presetOpts = append(presetOpts, huh.NewOption(m, m))
	}
	presetOpts = append(presetOpts, huh.NewOption("自定义...", "__custom__"))

	selectedModels := make([]string, 0)
	for _, m := range p.Models {
		selectedModels = append(selectedModels, m)
	}
	// 若已有自定义模型（不在预设中），先不显示在 MultiSelect 里
	// （自定义输入通过追加输入框收集）

	customModel := ""

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Provider Name (slug)").
			Placeholder("my-proxy").
			Validate(func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("name 不能为空")
				}
				for _, c := range s {
					if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
						return fmt.Errorf("只允许小写字母、数字和连字符，当前输入含非法字符: %c", c)
					}
				}
				return nil
			}).
			Value(&name),
		huh.NewInput().
			Title("Base URL").
			Placeholder("https://api.example.com/v1").
			Validate(func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("Base URL 不能为空")
				}
				u, err := url.ParseRequestURI(s)
				if err != nil || u.Scheme == "" {
					return fmt.Errorf("URL 格式无效（需包含 http:// 或 https://）")
				}
				return nil
			}).
			Value(&baseUrl),
		huh.NewInput().
			Title("API Key").
			Placeholder("sk-...").
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("API Key 不能为空")
				}
				return nil
			}).
			Value(&apiKey),
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description("选择此 provider 支持的模型（至少一项）；可同时选「自定义...」再在下方填入自定义名称").
			Options(presetOpts...).
			Validate(func(selected []string) error {
				// 只检查完全未选的情况；选了 __custom__ 放行，由 form.Run() 后的逻辑做最终校验
				if len(selected) == 0 {
					return fmt.Errorf("请至少选择一个模型")
				}
				return nil
			}).
			Value(&selectedModels),
		huh.NewInput().
			Title("自定义模型名称（可选，留空跳过）").
			Placeholder("my-custom-model").
			Value(&customModel),
		huh.NewSelect[string]().
			Title("API 格式").
			Description("不确定时选 openai-completions。若同一 provider 含不同格式模型，请分拆为多个 provider").
			Options(apiFormatOpts...).
			Value(&apiFormat),
	))

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.ProviderConfig{}, err
	}

	// 整理模型列表
	finalModels := []string{}
	for _, m := range selectedModels {
		if m != "__custom__" {
			finalModels = append(finalModels, m)
		}
	}
	if ct := strings.TrimSpace(customModel); ct != "" {
		finalModels = append(finalModels, ct)
	}
	// 若没有任何选择，返回错误
	if len(finalModels) == 0 {
		return config.ProviderConfig{}, fmt.Errorf("provider %q 的模型列表不能为空", name)
	}

	// 若 apiFormat 为空，根据第一个模型自动推断
	if apiFormat == "" {
		apiFormat = config.DetectAPIFormat(finalModels[0])
	}

	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		BaseUrl:   strings.TrimSpace(baseUrl),
		ApiKey:    strings.TrimSpace(apiKey),
		Models:    finalModels,
		ApiFormat: apiFormat,
	}, nil
}

// ── Step 2: 主 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep2MainAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) error {
	primary := fullCfg.MainAgent.Primary
	fallback := fullCfg.MainAgent.Fallback
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("主 Agent 模型 (Primary)").
			Description("agents.defaults.model.primary").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("主 Agent 备用模型 (Fallback)").
			Description("可选，留空表示不配置备用模型").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}
	fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return nil
}

// ── Step 3: 子 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep3SubAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) error {
	const sameAsMain = "__same__"
	subChoice := sameAsMain
	if fullCfg.SubAgent.Primary != "" {
		subChoice = "__custom__"
	}

	form1 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", "__custom__"),
			).
			Value(&subChoice),
	))
	if err := form1.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}

	if subChoice == sameAsMain {
		fullCfg.SubAgent = config.AgentModelConfig{}
		return nil
	}

	// 单独指定
	primary := fullCfg.SubAgent.Primary
	fallback := fullCfg.SubAgent.Fallback
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	form2 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 主模型 (Primary)").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("子 Agent 备用模型 (Fallback)").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form2.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}
	fullCfg.SubAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return nil
}

// ── Step 4: 命名 Agent（可选） ─────────────────────────────────────────────

func (a *App) runStep4NamedAgents(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
) error {
	const sameAsMain = ""
	allOptsWithSame := append(
		[]huh.Option[string]{huh.NewOption("同主 Agent", sameAsMain)},
		allOpts...,
	)

	var wantConfig bool
	form0 := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("是否配置命名 Agent？").
			Description("为特定 agent id 指定不同模型（可跳过）").
			Value(&wantConfig),
	))
	if err := form0.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}
	if !wantConfig {
		return nil
	}

	for {
		agentID := ""
		modelPrimary := ""
		if len(allOpts) > 0 {
			modelPrimary = allOpts[0].Value
		}

		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Agent ID").
				Placeholder("my-coder").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("Agent ID 不能为空")
					}
					return nil
				}).
				Value(&agentID),
			huh.NewSelect[string]().
				Title("使用模型").
				Options(allOptsWithSame...).
				Value(&modelPrimary),
		))
		if err := form.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr, "已取消")
				os.Exit(0)
			}
			return err
		}

		fullCfg.NamedAgents = append(fullCfg.NamedAgents, config.NamedAgentConfig{
			ID:    strings.TrimSpace(agentID),
			Model: config.AgentModelConfig{Primary: modelPrimary},
		})

		var continueAdding bool
		formContinue := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("是否继续添加命名 Agent？").
				Value(&continueAdding),
		))
		if err := formContinue.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				break
			}
			return err
		}
		if !continueAdding {
			break
		}
	}
	return nil
}

// ── 输出 ──────────────────────────────────────────────────────────────────

func printSuccess(cfg *config.FullConfig) {
	green := lipgloss.Color("42")

	var lines []string
	for _, p := range cfg.Providers {
		lines = append(lines, fmt.Sprintf("  Provider  : %s  (%s)  [%s]", p.Name, p.BaseUrl, p.ApiFormat))
		lines = append(lines, fmt.Sprintf("  模型      : %s", strings.Join(p.Models, ", ")))
		lines = append(lines, "")
	}
	lines = append(lines, fmt.Sprintf("  主 Agent  : %s", cfg.MainAgent.Primary))
	if cfg.SubAgent.Primary != "" {
		lines = append(lines, fmt.Sprintf("  子 Agent  : %s", cfg.SubAgent.Primary))
	}
	if len(cfg.NamedAgents) > 0 {
		for _, na := range cfg.NamedAgents {
			lines = append(lines, fmt.Sprintf("  Named [%s]: %s", na.ID, na.Model.Primary))
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(green).
		Padding(1, 2).
		Render("✓ 配置已保存！\n\n" + strings.Join(lines, "\n"))

	fmt.Println()
	fmt.Println(box)

	huh.NewForm(huh.NewGroup( //nolint:errcheck
		huh.NewNote().
			Title("提示").
			Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
			Next(true).
			NextLabel("按 Enter 退出"),
	)).Run() //nolint:errcheck
}

func printBanner() {
	purple := lipgloss.Color("63")
	gray := lipgloss.Color("240")

	art := "  ██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗\n" +
		"  ██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║\n" +
		"  ██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║\n" +
		"  ██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║\n" +
		"  ██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║\n" +
		"  ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝"

	logo := lipgloss.NewStyle().Foreground(purple).Render(art)
	subtitle := lipgloss.NewStyle().Bold(true).
		Render("  OpenClaw 配置工具  ·  openclaw-config " + Version)
	sep := lipgloss.NewStyle().Foreground(gray).
		Render("  ────────────────────────────────────────────────")
	note := lipgloss.NewStyle().Foreground(gray).
		Render("  多 Provider · 多模型 · 主/子/命名 Agent 独立配置")

	fmt.Println(logo)
	fmt.Println()
	fmt.Println(subtitle)
	fmt.Println(sep)
	fmt.Println(note)
	fmt.Println()
}
```

- [ ] **Step 5.2: 编译确认**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

Expected: 无编译错误

- [ ] **Step 5.3: 运行全部测试**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./... -v
```

Expected: 所有测试 PASS

- [ ] **Step 5.4: Commit**

```bash
git add app.go
git commit -m "feat: 重写 app.go — 4 步向导支持多 provider 与多 agent 模型配置"
```

---

## Task 6: 手动验证

- [ ] **Step 6.1: 本地运行测试（全新配置场景）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go run .
```

操作路径：
1. Step 1：选"添加新 Provider"，填入测试用的 URL/Key/模型，选 API 格式，选"继续 →"
2. Step 2：选主 agent 模型
3. Step 3：选"同主 Agent"
4. Step 4：跳过命名 agent
5. 验证成功保存提示，检查 `~/.openclaw/openclaw.json` 和 `auth-profiles.json`

- [ ] **Step 6.2: 验证已有配置兼容性（若有旧配置）**

若 `~/.openclaw/openclaw.json` 已存在：
- 运行 `go run .`
- 确认 Step 1 展示已有 provider 列表（非空）
- 若有旧 `https://www.dmxapi.cn`，确认已展示修复日志

- [ ] **Step 6.3: 验证多 provider 场景**

在 Step 1 添加第二个 provider，完成配置，检查 `openclaw.json` 中 `models.providers` 包含两个 provider。

- [ ] **Step 6.4: 最终 commit（如有未提交修改）**

```bash
cd /Users/yesongyun/代码/openclaw_config && git status
# 若有改动：
git add -A && git commit -m "fix: 手动验证后的修正"
```

---

## 完成标准

- [ ] `go test ./...` 全部通过
- [ ] `go build ./...` 无错误
- [ ] `go run .` 可正常运行 4 步向导
- [ ] 多 provider 场景：openclaw.json 正确写入多个 providers
- [ ] 兼容性：旧配置（单 dmxapi）可被正确读取并展示，旧 URL 被修复
- [ ] 命名 agent：agents.list 中其他条目未被删除，新条目正确写入
