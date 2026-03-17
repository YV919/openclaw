# Delete Group Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Provider 和命名 Agent 添加删除功能，采用二级操作菜单模式，并修复底层持久化逻辑确保删除落盘。

**Architecture:** 在 `app.go` 中通过二级菜单（先选对象，再选操作）实现 Provider 和命名 Agent 的增/改/删管理环；在 `manager.go` 中修复 `SaveFullConfig`（agents.list 先过滤删除再 upsert）和 `saveAllApiKeys`（清理孤立 auth-profiles 条目）的持久化逻辑。

**Tech Stack:** Go 1.23、charmbracelet/huh v0.6.0、charmbracelet/lipgloss v0.13.0

**Spec:** `docs/superpowers/specs/2026-03-17-delete-group-design.md`

---

## 文件变更总览

| 文件 | 类型 | 改动摘要 |
|------|------|---------|
| `internal/config/manager.go` | 修改 | `SaveFullConfig` agents.list 先过滤再 upsert；`saveAllApiKeys` 清理孤立 profileKey |
| `internal/config/manager_test.go` | 修改 | 新增测试：agents.list 删除落盘、auth-profiles 清理 |
| `app.go` | 修改 | Step 1 和 Step 4 管理环重构，新增 8 个函数 |

---

## Task 1：修复 SaveFullConfig — agents.list 删除持久化

**Files:**
- Modify: `internal/config/manager.go:651-698`
- Test: `internal/config/manager_test.go`

### 背景

当前代码（第 651 行）：`if len(cfg.NamedAgents) > 0 { ...upsert only... }`

- 只做 upsert，不删除已从 `cfg.NamedAgents` 移除的条目
- 当 `NamedAgents` 为空时整个块被跳过，旧条目永远残留

### 修复策略

"含 model 字段的条目"视为本工具管理，"不含 model 字段的条目"视为外部工具管理（保留）。

- [ ] **Step 1：写失败测试**

在 `internal/config/manager_test.go` 末尾追加：

```go
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

	// 只保留 agent-a，删除 agent-b
	cfg := &FullConfig{
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

	// 删除所有命名 agent
	cfg := &FullConfig{NamedAgents: []NamedAgentConfig{}}
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
```

- [ ] **Step 2：运行测试确认失败**

```bash
cd /Users/yesongyun/代码/openclaw_config
go test ./internal/config/... -run "TestSaveFullConfig_Deletes" -v
```

预期：FAIL（`agent-b` 仍存在 / `agent-x` 仍存在）

- [ ] **Step 3：修改 manager.go agents.list 写入块**

将 `manager.go` 第 651-698 行的 `if len(cfg.NamedAgents) > 0 { ... }` 块**整体替换**为以下代码。新代码仍位于第 603 行声明的 `agents` 变量作用域内，用 `agents["list"]` 直接写入，无需修改 import：

```go
// agents.list：先过滤删除，再 upsert（按 ID，保留无 model 字段的外部条目）
{
    var existingList []any
    if ag, ok := raw["agents"].(map[string]any); ok {
        existingList, _ = ag["list"].([]any)
    }

    // 构建本轮管理的 ID 集合
    managedIDs := make(map[string]bool, len(cfg.NamedAgents))
    for _, na := range cfg.NamedAgents {
        managedIDs[na.ID] = true
    }

    // 过滤：保留无 model 字段（外部管理）或 ID 仍在管理列表中的条目
    filtered := make([]any, 0, len(existingList))
    for _, item := range existingList {
        m, ok := item.(map[string]any)
        if !ok {
            continue
        }
        _, hasModel := m["model"]
        id, _ := m["id"].(string)
        if !hasModel || managedIDs[id] {
            filtered = append(filtered, item)
        }
    }

    // 构建 ID → index 映射（基于 filtered）
    indexByID := make(map[string]int, len(filtered))
    for i, item := range filtered {
        if m, ok := item.(map[string]any); ok {
            if id, ok := m["id"].(string); ok {
                indexByID[id] = i
            }
        }
    }

    // upsert cfg.NamedAgents
    for _, na := range cfg.NamedAgents {
        var modelField any
        if na.Model.Primary != "" {
            mf := map[string]any{"primary": na.Model.Primary}
            if na.Model.Fallback != "" {
                mf["fallbacks"] = []string{na.Model.Fallback}
            }
            modelField = mf
        }

        if idx, exists := indexByID[na.ID]; exists {
            entry, ok := filtered[idx].(map[string]any)
            if !ok {
                continue
            }
            if modelField != nil {
                entry["model"] = modelField
            } else {
                delete(entry, "model")
            }
        } else {
            newEntry := map[string]any{"id": na.ID}
            if modelField != nil {
                newEntry["model"] = modelField
            }
            filtered = append(filtered, newEntry)
        }
    }
    agents["list"] = filtered
}
```

- [ ] **Step 4：运行测试确认通过**

```bash
go test ./internal/config/... -run "TestSaveFullConfig_Deletes" -v
```

预期：PASS

- [ ] **Step 5：运行全量测试确认无回归**

```bash
go test ./internal/config/... -v
```

预期：全部 PASS

- [ ] **Step 6：提交**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "fix: SaveFullConfig agents.list 改为先过滤删除再 upsert，修复命名 Agent 删除不落盘问题"
```

---

## Task 2：修复 saveAllApiKeys — 清理孤立 auth-profiles 条目

**Files:**
- Modify: `internal/config/manager.go:719-741`
- Test: `internal/config/manager_test.go`

### 背景

删除 Provider 后，`auth-profiles.json` 中该 provider 的 `name:default` 条目会残留，导致废弃的 API Key 一直保存在磁盘上。

- [ ] **Step 1：写失败测试**

在 `internal/config/manager_test.go` 末尾追加：

```go
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
```

- [ ] **Step 2：运行测试确认失败**

```bash
go test ./internal/config/... -run "TestSaveFullConfig_CleansOrphanedAuthProfiles" -v
```

预期：FAIL（`provider-b:default` 仍存在）

- [ ] **Step 3：修改 saveAllApiKeys**

将 `manager.go` 第 719-741 行的 `saveAllApiKeys` **整体替换**为（`strings` 包已在 manager.go 顶部导入，无需新增）：

```go
// saveAllApiKeys 将所有 provider 的 ApiKey 写入 auth-profiles.json，并清理已删除 provider 的孤立条目
func (cm *ConfigManager) saveAllApiKeys(providers []ProviderConfig) error {
	authProfiles, err := cm.LoadAuthProfiles()
	if err != nil {
		authProfiles = map[string]any{
			"version":  1,
			"profiles": map[string]any{},
		}
	}
	profiles, ok := authProfiles["profiles"].(map[string]any)
	if !ok {
		profiles = map[string]any{}
		authProfiles["profiles"] = profiles
	}

	// 构建当前 provider 名称集合
	currentNames := make(map[string]bool, len(providers))
	for _, p := range providers {
		currentNames[p.Name] = true
	}

	// 清理孤立条目：key 格式为 "{name}:default"，且 name 不在当前 providers 中
	for key := range profiles {
		if idx := strings.LastIndex(key, ":"); idx > 0 {
			suffix := key[idx+1:]
			name := key[:idx]
			if suffix == "default" && !currentNames[name] {
				delete(profiles, key)
			}
		}
	}

	// 写入当前 providers 的 API Key
	for _, p := range providers {
		profileKey := p.Name + ":default"
		profiles[profileKey] = map[string]any{
			"type":     "api_key",
			"provider": p.Name,
			"key":      p.ApiKey,
		}
	}
	return cm.SaveAuthProfiles(authProfiles)
}
```

- [ ] **Step 4：运行测试确认通过**

```bash
go test ./internal/config/... -run "TestSaveFullConfig_CleansOrphanedAuthProfiles" -v
```

预期：PASS

- [ ] **Step 5：运行全量测试确认无回归**

```bash
go test ./internal/config/... -v
```

预期：全部 PASS

- [ ] **Step 6：提交**

```bash
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "fix: saveAllApiKeys 删除 Provider 时清理孤立 auth-profiles 条目"
```

---

## Task 3：app.go — checkProviderDeps + deleteProvider

**Files:**
- Modify: `app.go`

### 背景

这两个函数是 Provider 删除流程的核心逻辑，先实现再接入 UI。

- [ ] **Step 1：在 app.go 中新增 checkProviderDeps**

在 `containsOptValue` 函数之后追加：

```go
// checkProviderDeps 返回引用了指定 provider 模型的 Agent 描述列表
func checkProviderDeps(providerName string, cfg *config.FullConfig) []string {
	prefix := providerName + "/"
	var deps []string
	if strings.HasPrefix(cfg.MainAgent.Primary, prefix) {
		deps = append(deps, "主 Agent (primary)")
	}
	if strings.HasPrefix(cfg.MainAgent.Fallback, prefix) {
		deps = append(deps, "主 Agent (fallback)")
	}
	if strings.HasPrefix(cfg.SubAgent.Primary, prefix) {
		deps = append(deps, "子 Agent (primary)")
	}
	if strings.HasPrefix(cfg.SubAgent.Fallback, prefix) {
		deps = append(deps, "子 Agent (fallback)")
	}
	for _, na := range cfg.NamedAgents {
		if strings.HasPrefix(na.Model.Primary, prefix) {
			deps = append(deps, fmt.Sprintf("命名 Agent [%s] (primary)", na.ID))
		}
		if strings.HasPrefix(na.Model.Fallback, prefix) {
			deps = append(deps, fmt.Sprintf("命名 Agent [%s] (fallback)", na.ID))
		}
	}
	return deps
}
```

- [ ] **Step 2：在 app.go 中新增 deleteProvider**

紧接 `checkProviderDeps` 之后追加：

```go
// deleteProvider 删除指定 provider，删除前检查依赖并警告，确认后清空相关 NamedAgent 的模型引用
func deleteProvider(fullCfg *config.FullConfig, name string) error {
	deps := checkProviderDeps(name, fullCfg)
	if len(deps) > 0 {
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		fmt.Println(yellow.Render(fmt.Sprintf("  ⚠ Provider %q 被以下配置引用：", name)))
		for _, d := range deps {
			fmt.Println(dim.Render("    · " + d))
		}
		fmt.Println()
	}

	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("确认删除 Provider %q？", name)).
			Description("删除后相关 NamedAgent 的模型引用将被清空，主/子 Agent 的引用将在下一步重新选择。").
			Value(&confirmed),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	if !confirmed {
		return nil
	}

	// 从 Providers 中移除
	newProviders := fullCfg.Providers[:0]
	for _, p := range fullCfg.Providers {
		if p.Name != name {
			newProviders = append(newProviders, p)
		}
	}
	fullCfg.Providers = newProviders

	// 清空引用该 provider 的 NamedAgent 模型字段
	prefix := name + "/"
	for i := range fullCfg.NamedAgents {
		if strings.HasPrefix(fullCfg.NamedAgents[i].Model.Primary, prefix) {
			fullCfg.NamedAgents[i].Model.Primary = ""
		}
		if strings.HasPrefix(fullCfg.NamedAgents[i].Model.Fallback, prefix) {
			fullCfg.NamedAgents[i].Model.Fallback = ""
		}
	}
	return nil
}
```

- [ ] **Step 3：构建确认编译通过**

```bash
go build ./...
```

预期：无报错

- [ ] **Step 4：提交**

```bash
git add app.go
git commit -m "feat: 新增 checkProviderDeps 和 deleteProvider 函数"
```

---

## Task 4：app.go — pickProviderItemAction + 接入 Step 1

**Files:**
- Modify: `app.go`

### 背景

新增二级菜单函数，并修改 `pickProviderAction` 标签和 `runStep1Providers` 主循环。

- [ ] **Step 1：新增 pickProviderItemAction**

在 `pickProviderAction` 函数之后追加：

```go
// pickProviderItemAction 弹出 Provider 二级操作菜单
func pickProviderItemAction(name string) (string, error) {
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Provider: %s", name)).
			Options(
				huh.NewOption("编辑", "__edit__"),
				huh.NewOption("删除", "__delete__"),
				huh.NewOption("← 返回", "__back__"),
			).
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
```

- [ ] **Step 2：修改 pickProviderAction 的主菜单标签**

将 `pickProviderAction` 中的：

```go
label := fmt.Sprintf("[编辑] %s  (%s)", p.Name, p.BaseUrl)
```

改为：

```go
label := fmt.Sprintf("%s  (%s)", p.Name, p.BaseUrl)
```

同时将 `Title` 和 `Description` 改为：

```go
Title("Provider 管理").
Description("选择要操作的 Provider，或添加新的").
```

（Description 原为"选择要编辑的 provider，或添加新的"，改为不涉及"编辑"。）

- [ ] **Step 3：修改 runStep1Providers 接入二级菜单**

将 `runStep1Providers` 的 else 分支（第 131-143 行）替换为：

```go
} else {
    // 选中已有 provider → 弹二级菜单
    subAction, err := pickProviderItemAction(action)
    if err != nil {
        return err
    }
    switch subAction {
    case "__edit__":
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
    case "__delete__":
        if err := deleteProvider(fullCfg, action); err != nil {
            return err
        }
    }
    // "__back__" 直接继续外层 for
}
```

- [ ] **Step 4：构建确认编译通过**

```bash
go build ./...
```

预期：无报错

- [ ] **Step 5：提交**

```bash
git add app.go
git commit -m "feat: Step 1 Provider 管理接入二级菜单，支持编辑/删除/返回"
```

---

## Task 5：确认 allModelOpts 构建顺序（只读验证，无代码改动）

**Files:** 无

### 背景

规格要求 Step 1 结束后重建 `allModelOpts`。经检查，现有代码顺序已经正确：先 `runStep1Providers`，再 `buildAllModelOpts(fullCfg.Providers)`，**无需修改代码**。

- [ ] **Step 1：验证顺序正确即可跳过**

```bash
grep -n "runStep1Providers\|buildAllModelOpts" /Users/yesongyun/代码/openclaw_config/app.go
```

预期：`runStep1Providers` 的行号（约 57）小于 `buildAllModelOpts` 的行号（约 62）

> ✅ 顺序正确则此 Task 完成，无需提交。

---

## Task 6：app.go — 命名 Agent 管理环（Step 4 重构）

**Files:**
- Modify: `app.go:431-528`（`runStep4NamedAgents`）

### 背景

Step 4 当前逻辑：前置 Confirm → 直接循环新增（upsert）。需重构为：管理环 → 一级选单 → 二级操作（编辑/删除/返回）。

- [ ] **Step 1：新增 pickNamedAgentAction**

在 `runStep4NamedAgents` 之前追加：

```go
// pickNamedAgentAction 命名 Agent 一级选单
func pickNamedAgentAction(agents []config.NamedAgentConfig) (string, error) {
	var opts []huh.Option[string]
	for _, na := range agents {
		modelLabel := na.Model.Primary
		if modelLabel == "" {
			modelLabel = "同主 Agent"
		}
		label := fmt.Sprintf("%s  (%s)", na.ID, modelLabel)
		opts = append(opts, huh.NewOption(label, na.ID))
	}
	opts = append(opts, huh.NewOption("[+ 添加新命名 Agent]", "__add__"))
	if len(agents) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
	}

	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("命名 Agent 管理").
			Description("为特定 agent id 指定不同模型").
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
```

- [ ] **Step 2：新增 pickNamedAgentItemAction**

紧接追加：

```go
// pickNamedAgentItemAction 命名 Agent 二级操作菜单
func pickNamedAgentItemAction(id string) (string, error) {
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("命名 Agent: %s", id)).
			Options(
				huh.NewOption("编辑", "__edit__"),
				huh.NewOption("删除", "__delete__"),
				huh.NewOption("← 返回", "__back__"),
			).
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
```

- [ ] **Step 3：新增 editNamedAgent**

紧接追加（`errors`、`lipgloss` 包均已在 app.go 顶部导入，无需新增 import）：

> **注：** `editNamedAgent` 支持 Primary + Fallback，而 `__add__` 分支目前只收集 Primary（与现有行为一致）。这是已知的不对称，后续版本可补全。

```go
// editNamedAgent 编辑已有命名 Agent（Agent ID 只读）
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, error) {
	primary := agent.Model.Primary
	fallback := agent.Model.Fallback

	form := huh.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("Agent ID").
			Description(agent.ID),
		huh.NewSelect[string]().
			Title("使用模型 (Primary)").
			Options(allOptsWithSame...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("备用模型 (Fallback)").
			Description("可选，留空表示不配置").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.NamedAgentConfig{}, err
	}
	return config.NamedAgentConfig{
		ID:    agent.ID,
		Model: config.AgentModelConfig{Primary: primary, Fallback: fallback},
	}, nil
}
```

- [ ] **Step 4：重写 runStep4NamedAgents**

将整个 `runStep4NamedAgents` 函数（第 433-528 行）替换为：

```go
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
	allOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allOpts...,
	)

	for {
		action, err := pickNamedAgentAction(fullCfg.NamedAgents)
		if err != nil {
			return err
		}

		switch action {
		case "__continue__":
			return nil

		case "__add__":
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
			id := strings.TrimSpace(agentID)
			upserted := false
			for i, na := range fullCfg.NamedAgents {
				if na.ID == id {
					fullCfg.NamedAgents[i] = config.NamedAgentConfig{
						ID:    id,
						Model: config.AgentModelConfig{Primary: modelPrimary},
					}
					upserted = true
					break
				}
			}
			if !upserted {
				fullCfg.NamedAgents = append(fullCfg.NamedAgents, config.NamedAgentConfig{
					ID:    id,
					Model: config.AgentModelConfig{Primary: modelPrimary},
				})
			}

		default:
			// 选中已有 Agent → 二级菜单
			subAction, err := pickNamedAgentItemAction(action)
			if err != nil {
				return err
			}
			switch subAction {
			case "__edit__":
				for i, na := range fullCfg.NamedAgents {
					if na.ID == action {
						updated, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return err
						}
						fullCfg.NamedAgents[i] = updated
						break
					}
				}
			case "__delete__":
				newAgents := fullCfg.NamedAgents[:0]
				for _, na := range fullCfg.NamedAgents {
					if na.ID != action {
						newAgents = append(newAgents, na)
					}
				}
				fullCfg.NamedAgents = newAgents
			}
			// "__back__" 继续外层 for
		}
	}
}
```

- [ ] **Step 5：构建确认编译通过**

```bash
go build ./...
```

预期：无报错

- [ ] **Step 6：提交**

```bash
git add app.go
git commit -m "feat: Step 4 命名 Agent 管理重构为管理环，支持编辑和删除"
```

---

## Task 7：集成验证

**Files:** 无新增

- [ ] **Step 1：运行全量测试**

```bash
go test ./... -v
```

预期：全部 PASS，无报错

- [ ] **Step 2：手动验证核心流程（smoke test）**

```bash
go run .
```

验证以下场景：

| 场景 | 预期行为 |
|------|----------|
| Step 1 选中 Provider → 看到编辑/删除/返回二级菜单 | ✓ |
| Step 1 删除 Provider，Step 2 下拉列表不含该 Provider 模型 | ✓ |
| Step 1 删除被命名 Agent 引用的 Provider → 显示黄色警告 → 确认后删除 | ✓ |
| Step 4 看到已有命名 Agent 列表（不再是前置 Confirm） | ✓ |
| Step 4 选中 Agent → 看到编辑/删除/返回 | ✓ |
| Step 4 编辑 Agent，ID 只读，可修改 Primary 和 Fallback | ✓ |
| Step 4 删除 Agent，下次进入 Step 4 该 Agent 消失 | ✓ |
| 保存后删除的 Agent 不在磁盘配置中（依赖 Task 1） | ✓ |
| 保存后删除的 Provider API Key 不在 auth-profiles.json 中（依赖 Task 2） | ✓ |

- [ ] **Step 3：最终提交**

```bash
git add -A
git commit -m "feat: 完成 Provider 和命名 Agent 删除功能 — 二级菜单 + 持久化修复"
```
