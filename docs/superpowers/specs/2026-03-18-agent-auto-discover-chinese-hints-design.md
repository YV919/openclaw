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

---

## 架构

### 新增：`internal/config/manager.go`

```go
// ListAgentIDsFromDisk 扫描 ~/.openclaw/agents/ 目录，返回所有子目录名。
// 排除 "main"（默认 agent，由 Step 2/3 管理）。
// 目录不存在时静默返回空切片。
func (cm *ConfigManager) ListAgentIDsFromDisk() []string
```

- 只返回子目录名，不递归
- 排除 `main`

### 修改：`extractNamedAgents`（`manager.go`）

新增对 `id == "main"` 的过滤，与 `ListAgentIDsFromDisk` 保持一致：两个来源都不包含 `main`。

### 修改：`LoadFullConfig()`

在 `extractNamedAgents(raw)` 之后，调用 `ListAgentIDsFromDisk()`，合并两个来源：

```
agents.list → extractNamedAgents（过滤 main）→ []NamedAgentConfig
                                                       ↓ 合并
~/.openclaw/agents/ → ListAgentIDsFromDisk → 过滤已存在 ID → 追加空模型条目
                                                       ↓
                                          cfg.NamedAgents（最终）
```

合并规则：磁盘有但 agents.list 没有的 ID，追加 `NamedAgentConfig{ID: id, Model: {}}` 条目。

**SaveFullConfig 行为说明**：对于磁盘发现但 `agents.list` 中原本不存在的 agent，若用户不配置模型（Primary 为空），SaveFullConfig 会在 agents.list 中写入一个最小条目 `{"id": "..."}` 而不含 `model` 字段。这与 openclaw 的"外部管理"约定一致（无 model 字段的条目被视为外部管理，openclaw 自身可追加配置），不会破坏现有行为。若用户为其配置了模型，则写入完整的 `{"id": "...", "model": {...}}` 条目。

### 新增：`app.go`

```go
// newForm 创建带中文 KeyMap 的 huh.Form（统一入口）
func newForm(groups ...*huh.Group) *huh.Form {
    return huh.NewForm(groups...).WithKeyMap(chineseKeyMap())
}
```

将 `app.go` 中**所有** `huh.NewForm(` 调用替换为 `newForm(`，包括：
- `pickProviderAction`
- `pickProviderItemAction`
- `deleteProvider`（确认对话框）
- `editProvider`（已有 `.WithKeyMap`，替换后去掉重复调用）
- `runStep2MainAgent`
- `runStep3SubAgent`（两处）
- `pickNamedAgentAction`
- `pickNamedAgentItemAction`
- `editNamedAgent`
- `printSuccess` 末尾的 Note

共 11 处（含 `deleteProvider`）。

### 修改：`app.go` - `pickNamedAgentAction`

- 删除 `huh.NewOption("[+ 添加新命名 Agent]", "__add__")` 选项
- **始终显示** `[继续 →]`，去除 `if len(agents) > 0` 的限制
  - 无 agent 时：列表只有 `[继续 →]` 一项，huh.Select 正常工作，用户可直接确认跳过
  - 有 agent 时：列表展示所有 agent + `[继续 →]`

### 修改：`app.go` - `runStep4NamedAgents`

- 移除 `case "__add__":` 分支及其所有逻辑

---

## 数据流

### 读取阶段（LoadFullConfig）

```
openclaw.json agents.list → extractNamedAgents（过滤 main）
~/.openclaw/agents/ 目录   → ListAgentIDsFromDisk（排除 main）
                                      ↓
                              合并去重（以 ID 为 key）
                                      ↓
                          cfg.NamedAgents（含空模型条目）
```

### 保存阶段（SaveFullConfig）

SaveFullConfig 无需修改。现有逻辑：
- 已在 agents.list 的条目：upsert model 字段（有则写入，无则删除 model 字段保留条目）
- 仅在磁盘发现（不在 agents.list）的条目：创建最小条目 `{"id": "..."}` 或 `{"id": "...", "model": {...}}`
- 保留外部管理条目（无 model 字段）不动

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| `~/.openclaw/agents/` 不存在 | 静默跳过，不报错 |
| 磁盘 agent 已在 agents.list | 不重复添加 |
| 无任何命名 agent | Step 4 只显示 `[继续 →]`，huh.Select 单选项正常工作 |
| 用户删除某 agent | 仅从 `fullCfg.NamedAgents` 移除，不删磁盘目录（openclaw 管理） |
| 用户删除磁盘来源 agent 后重启工具 | 该 agent 目录仍在磁盘，下次运行会再次出现在列表中——这是预期行为，用户只能通过 openclaw 本身删除 agent |
| agents.list 中有 id=="main" 的条目 | extractNamedAgents 过滤掉，不出现在命名 Agent 列表 |

---

## 测试策略

- `ListAgentIDsFromDisk` 单元测试：目录不存在、空目录、正常目录、过滤 `main`
- `extractNamedAgents` 单元测试（新增）：验证过滤 `id == "main"` 的行为
- `LoadFullConfig` 集成测试：磁盘来源与 agents.list 合并去重
- 导航提示：手动运行 `go run .` 验证（huh TUI 难以自动化测试）
