# 设计文档：命名 Agent 自动发现 + 导航提示中文化

**日期**：2026-03-18
**状态**：已批准

---

## 背景

当前 openclaw-config TUI 存在两个问题：

1. 命名 Agent 管理界面提供了 `[+ 添加新命名 Agent]` 按钮，但 openclaw 本身会在集成（如飞书）初始化时自动创建 agent 条目，用户不应手动创建 agent。
2. 所有 `huh.Select` / `huh.MultiSelect` 表单的底部导航提示为英文（如 `↑ up • ↓ down • / filter • enter submit`），与界面整体中文风格不符。

---

## 目标

1. 移除命名 Agent 的手动创建入口，改为自动发现并展示已有 agent（来自 `agents.list` 和磁盘目录两个来源）。
2. 将所有 huh 表单的导航提示统一为中文。

---

## 不做的事

- 不删除磁盘上的 agent 目录（agent 生命周期由 openclaw 管理）
- 不新增 agent（只读+编辑模型）
- 不改动 `internal/config/manager.go` 中与 Provider 无关的现有逻辑

---

## 架构

### 新增：`internal/config/manager.go`

```go
// ListAgentIDsFromDisk 扫描 ~/.openclaw/agents/ 目录，返回所有子目录名（排除 "main"）
func (cm *ConfigManager) ListAgentIDsFromDisk() []string
```

- 目录不存在时静默返回空切片（不报错）
- 只返回子目录名，不递归
- 排除 `main`（由 Step 2/3 管理的默认 agent）

### 修改：`LoadFullConfig()`

在 `extractNamedAgents(raw)` 之后，调用 `ListAgentIDsFromDisk()`，合并两个来源：

```
agents.list → extractNamedAgents → []NamedAgentConfig
                                         ↓ 合并
~/.openclaw/agents/ → ListAgentIDsFromDisk → 过滤已存在 ID → 追加空模型条目
                                         ↓
                          去除 "main"，最终 cfg.NamedAgents
```

合并规则：磁盘有但 agents.list 没有的 ID，追加 `NamedAgentConfig{ID: id, Model: {}}` 条目。

### 新增：`app.go`

```go
// newForm 创建带中文 KeyMap 的 huh.Form（统一入口）
func newForm(groups ...*huh.Group) *huh.Form {
    return huh.NewForm(groups...).WithKeyMap(chineseKeyMap())
}
```

全文替换 `huh.NewForm(` → `newForm(`（约 10 处调用）。

### 修改：`app.go` - `pickNamedAgentAction`

- 删除 `huh.NewOption("[+ 添加新命名 Agent]", "__add__")` 选项
- 无 agent 时仍显示 `[继续 →]`（去除当前 `if len(agents) > 0` 判断）

### 修改：`app.go` - `runStep4NamedAgents`

- 移除 `case "__add__":` 分支及其所有逻辑

---

## 数据流

### 读取阶段（LoadFullConfig）

```
openclaw.json agents.list → extractNamedAgents
~/.openclaw/agents/ 目录    → ListAgentIDsFromDisk
                              ↓
                          合并去重（以 ID 为 key）
                              ↓
                          cfg.NamedAgents（含空模型条目）
```

### 保存阶段（SaveFullConfig）

无需修改。现有逻辑：
- 已在 agents.list 的条目：upsert model 字段
- 仅在磁盘发现（不在 agents.list）的条目：新增 `{"id": "...", "model": {...}}` 最小条目
- 外部管理条目（无 model 字段）：保留不动

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| `~/.openclaw/agents/` 不存在 | 静默跳过，不报错 |
| 磁盘 agent 已在 agents.list | 不重复添加 |
| 无任何命名 agent | Step 4 显示空列表 + `[继续 →]` |
| 用户删除某 agent | 仅从 `fullCfg.NamedAgents` 移除，不删磁盘目录 |

---

## 测试策略

- `ListAgentIDsFromDisk` 单元测试：目录不存在、空目录、正常目录、过滤 `main`
- `LoadFullConfig` 集成测试：磁盘来源与 agents.list 合并去重
- 导航提示：手动运行 `go run .` 验证（huh TUI 难以自动化测试）
