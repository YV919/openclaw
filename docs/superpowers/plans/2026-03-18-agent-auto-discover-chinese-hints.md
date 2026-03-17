# Agent 自动发现 + 导航提示中文化 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移除命名 Agent 手动创建入口，改为自动发现磁盘 + agents.list 双源合并；同时将所有 huh 表单导航提示统一为中文。

**Architecture:** 在 `config/manager.go` 新增 `ListAgentIDsFromDisk()` 扫描 `~/.openclaw/agents/` 子目录，`LoadFullConfig()` 合并两源；`app.go` 新增 `newForm()` 辅助函数统一应用中文 KeyMap，并移除命名 Agent 的"添加"分支。

**Tech Stack:** Go 1.23, charmbracelet/huh v0.6.0

---

## 文件映射

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/config/manager.go` | 修改 | 新增 `ListAgentIDsFromDisk()`，修改 `extractNamedAgents()`，修改 `LoadFullConfig()` |
| `internal/config/manager_test.go` | 修改 | 新增对应单元测试 |
| `app.go` | 修改 | 新增 `newForm()`，替换所有 `huh.NewForm(`，移除 `__add__` 分支 |

---

## Task 1：`ListAgentIDsFromDisk` + `extractNamedAgents` 过滤 main

**Files:**
- Modify: `internal/config/manager.go`
- Modify: `internal/config/manager_test.go`

- [ ] **Step 1: 写失败测试 — `ListAgentIDsFromDisk`**

在 `manager_test.go` 末尾追加：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestListAgentIDsFromDisk -v
```

期望：`FAIL — undefined: cm.ListAgentIDsFromDisk` 或编译错误

- [ ] **Step 3: 实现 `ListAgentIDsFromDisk`**

在 `internal/config/manager.go` 末尾追加：

```go
// ListAgentIDsFromDisk 扫描 ~/.openclaw/agents/ 目录，返回所有子目录名。
// 排除 "main"（默认 agent，由 Step 2/3 管理）。
// 目录不存在时静默返回空切片。
func (cm *ConfigManager) ListAgentIDsFromDisk() []string {
	agentsDir := filepath.Join(cm.homeDir, OpenClawDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "main" {
			continue
		}
		ids = append(ids, e.Name())
	}
	return ids
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestListAgentIDsFromDisk -v
```

期望：`PASS`

- [ ] **Step 5: 写失败测试 — `extractNamedAgents` 过滤 main**

在 `manager_test.go` 追加：

```go
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
```

- [ ] **Step 6: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestExtractNamedAgentsFiltersMain -v
```

期望：`FAIL`（main 未被过滤）

- [ ] **Step 7: 修改 `extractNamedAgents` 过滤 `main`**

在 `manager.go` 的 `extractNamedAgents` 函数中，找到：

```go
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
```

改为：

```go
		id, _ := m["id"].(string)
		if id == "" || id == "main" {
			continue
		}
```

- [ ] **Step 8: 运行全部 config 测试确认通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -v
```

期望：全部 PASS

- [ ] **Step 9: Commit**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat: add ListAgentIDsFromDisk and filter main from extractNamedAgents"
```

---

## Task 2：`LoadFullConfig` 合并磁盘来源

**Files:**
- Modify: `internal/config/manager.go`
- Modify: `internal/config/manager_test.go`

- [ ] **Step 1: 写失败测试 — 磁盘来源合并**

在 `manager_test.go` 追加（使用项目已有的 `setupTestHome` + `writeJSON` 辅助函数）：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -run TestLoadFullConfigMergesDiskAgents -v
```

期望：`FAIL`（feishu 未被合并）

- [ ] **Step 3: 修改 `LoadFullConfig` 合并磁盘来源**

在 `manager.go` 的 `LoadFullConfig` 函数中，找到：

```go
	cfg.NamedAgents = extractNamedAgents(raw)

	return cfg, logs, nil
```

改为：

```go
	cfg.NamedAgents = extractNamedAgents(raw)

	// 合并磁盘来源：将磁盘中存在但 agents.list 没有的 agent ID 追加为空模型条目
	existingIDs := make(map[string]bool, len(cfg.NamedAgents))
	for _, na := range cfg.NamedAgents {
		existingIDs[na.ID] = true
	}
	for _, id := range cm.ListAgentIDsFromDisk() {
		if !existingIDs[id] {
			cfg.NamedAgents = append(cfg.NamedAgents, NamedAgentConfig{ID: id})
		}
	}

	return cfg, logs, nil
```

- [ ] **Step 4: 运行全量测试确认通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./internal/config/... -v
```

期望：全部 PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add internal/config/manager.go internal/config/manager_test.go
git commit -m "feat: merge disk-discovered agents into LoadFullConfig"
```

---

## Task 3：`newForm` 辅助函数 + 替换所有 `huh.NewForm` 调用

**Files:**
- Modify: `app.go`

- [ ] **Step 1: 在 `app.go` 新增 `newForm` 辅助函数**

在 `chineseKeyMap()` 函数的结束 `}` 之后（即 `// chineseKeyMap` 注释块的最后一行，紧接 `func editProvider` 之前）插入：

```go
// newForm 创建带中文 KeyMap 的 huh.Form（统一入口，避免重复调用 WithKeyMap）
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithKeyMap(chineseKeyMap())
}
```

插入后文件结构为：
```
func chineseKeyMap() *huh.KeyMap { ... }   // 原有函数，不改
                                           // ← 插入 newForm 在此
func editProvider(...) { ... }             // 原有函数，不改
```

- [ ] **Step 2: 替换 `app.go` 中所有 `huh.NewForm(` 为 `newForm(`**

共 11 处（不含将在 Task 4 删除的 `__add__` 分支内的第 12 处）：

1. `pickProviderAction`：`huh.NewForm(huh.NewGroup(` → `newForm(huh.NewGroup(`
2. `pickProviderItemAction`：同上
3. `deleteProvider`：同上
4. `editProvider`：
   - 将 `huh.NewForm(huh.NewGroup(` → `newForm(huh.NewGroup(`
   - **同时删除**该 form 末尾的 `.WithKeyMap(chineseKeyMap())`，避免重复应用
   - 修改前：`)).WithKeyMap(chineseKeyMap())`
   - 修改后：`))`
5. `runStep2MainAgent`：`huh.NewForm(huh.NewGroup(` → `newForm(huh.NewGroup(`
6. `runStep3SubAgent` 中 form1：同上
7. `runStep3SubAgent` 中 form2：同上
8. `pickNamedAgentAction`：同上
9. `pickNamedAgentItemAction`：同上
10. `editNamedAgent`：同上
11. `printSuccess` 末尾 Note 表单：同上

- [ ] **Step 3: 编译确认无报错**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

期望：无输出（编译通过）

- [ ] **Step 4: 运行全量测试**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./...
```

期望：全部 PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add app.go
git commit -m "feat: add newForm helper and unify Chinese keymap across all forms"
```

---

## Task 4：移除命名 Agent 手动创建入口

**Files:**
- Modify: `app.go`

- [ ] **Step 1: 修改 `pickNamedAgentAction` — 移除"添加"选项，始终显示"继续"**

找到 `pickNamedAgentAction` 函数中的：

```go
	opts = append(opts, huh.NewOption("[+ 添加新命名 Agent]", "__add__"))
	if len(agents) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
	}
```

替换为：

```go
	opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
```

- [ ] **Step 2: 修改 `runStep4NamedAgents` — 移除 `__add__` case**

找到并删除整个 `case "__add__":` 分支（从 `case "__add__":` 到对应的 `}` 结束）。

删除后 `switch action` 只剩两个分支：
```go
switch action {
case "__continue__":
    return nil
default:
    // 选中已有 Agent → 二级菜单
    ...
}
```

- [ ] **Step 3: 编译确认无报错**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

期望：无输出

- [ ] **Step 4: 运行全量测试**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./...
```

期望：全部 PASS

- [ ] **Step 5: 手动验证 TUI**

```bash
cd /Users/yesongyun/代码/openclaw_config && go run .
```

验证以下几点：
- Step 4 "命名 Agent 管理"界面：不再有"添加新命名 Agent"选项
- 如有磁盘 agent（`~/.openclaw/agents/` 下子目录），自动出现在列表中
- 所有界面底部导航提示显示中文（如 `↑ 向上 • ↓ 向下 • / 过滤 • enter 确认`）

- [ ] **Step 6: Commit**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add app.go
git commit -m "feat: remove manual named agent creation, show auto-discovered agents only"
```
