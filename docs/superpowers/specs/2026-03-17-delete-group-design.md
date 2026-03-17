# 设计文档：Provider 与命名 Agent 的删除功能

**日期：** 2026-03-17
**状态：** 已批准
**范围：** `app.go` — Step 1 Provider 管理 / Step 4 命名 Agent 管理

---

## 背景

当前工具支持添加和编辑 Provider，支持添加命名 Agent（upsert），但均无删除功能。用户需要能够删除不再需要的 Provider 和命名 Agent。

---

## 目标

1. Provider 管理（Step 1）：增加删除功能，删除前检查依赖并警告
2. 命名 Agent 管理（Step 4）：增加编辑和删除功能，重构为与 Step 1 一致的管理环

---

## 方案选择

采用**二级操作菜单**模式：

- 第一级：选择要操作的对象（已有 Provider/Agent 列表 + 添加 + 继续）
- 第二级：选择操作（编辑 / 删除 / 返回）

优于"列表内混排删除项"方案（方案 A），原因：
- 视觉上更清晰，不将编辑和删除混在同一层
- Provider 和命名 Agent 可复用同一 UX 模式
- 与删除前警告提示配合自然

---

## 详细设计

### Step 1：Provider 管理

**现有函数：**
- `runStep1Providers(fullCfg)` — 管理环，调用 `pickProviderAction` 和 `editProvider`
- `pickProviderAction(providers)` — 一级选单，返回 provider name 或 `__add__` / `__continue__`

**变更：**

1. `pickProviderAction` 保持不变（仍只负责选对象）
2. 在 `runStep1Providers` 中，当选中已有 provider 时，调用新函数 `pickProviderItemAction(name)` 弹出二级菜单，返回以下之一：
   - `"__edit__"` → 进入 `editProvider`（现有逻辑不变）
   - `"__delete__"` → 调用 `deleteProvider(fullCfg, name)`
   - `"__back__"` → 返回一级选单
3. 新增 `deleteProvider(fullCfg, name)` 函数：
   - 调用 `checkProviderDeps(name, fullCfg)` 获取依赖列表
   - 若有依赖，用黄色文字列出受影响的 Agent（主/子/命名）
   - 弹出 `huh.NewConfirm` 询问是否确认删除
   - 确认后从 `fullCfg.Providers` 中移除该 provider（不清除 Agent 的模型引用）
4. 新增 `checkProviderDeps(providerName string, cfg *config.FullConfig) []string`：
   - 检查 `MainAgent.Primary/Fallback` 是否以 `providerName+"/"`  开头
   - 检查 `SubAgent.Primary/Fallback` 是否引用
   - 遍历 `NamedAgents`，检查各 Agent 的 `Model.Primary/Fallback`
   - 返回受影响项的描述列表，如 `["主 Agent (primary)", "named:my-coder (primary)"]`

### Step 4：命名 Agent 管理

**现有实现问题：**
- 直接进入循环新增 Agent，无法查看/编辑/删除已有的
- 仅有 upsert，没有独立的编辑和删除流程

**重构为与 Step 1 一致的管理环：**

1. 新增 `pickNamedAgentAction(agents)` — 类似 `pickProviderAction`：
   - 列出已有命名 Agent（格式：`[选择] agentID  (model)`）
   - `[+ 添加新命名 Agent]`
   - `[继续 →]`（当已有 Agent 时才显示）
   - 返回 agent ID 或 `__add__` / `__continue__`
2. 当选中已有 Agent 时，调用 `pickNamedAgentItemAction(id)` 弹出二级菜单：
   - `[编辑]` / `[删除]` / `[返回]`
3. 编辑：打开 `editNamedAgent(agent, allOptsWithSame)` 表单（填充现有值）
4. 删除：命名 Agent 不被其他配置引用，直接从 `fullCfg.NamedAgents` 中移除，无需警告
5. 添加：复用现有的新建表单逻辑

---

## 边界情况

| 情况 | 处理方式 |
|------|----------|
| 删除 Provider 后，Agent 模型引用悬空 | 警告用户，但不阻止删除；用户在 Step 2/3/4 重新选择 |
| 删除最后一个 Provider | 允许删除，Step 2/3 会因无可用模型而进入空状态（后续可加验证） |
| 命名 Agent 列表为空时进入 Step 4 | 只显示 `[+ 添加]`，无 `[继续 →]`（与现有 Provider 行为一致） |
| 用户在任意表单按 Esc/Ctrl+C | 维持现有行为：打印"已取消"并 `os.Exit(0)` |

---

## 不涉及的修改

- `internal/config/` 下所有文件（`types.go`、`manager.go` 等）
- `main.go`
- `SaveFullConfig`、`LoadFullConfig`

---

## 文件变更范围

仅修改 `app.go`，新增以下函数：

| 函数 | 用途 |
|------|------|
| `pickProviderItemAction(name)` | Provider 二级操作菜单 |
| `deleteProvider(fullCfg, name)` | Provider 删除逻辑（含依赖检查） |
| `checkProviderDeps(name, cfg)` | 依赖检查，返回受影响项描述 |
| `pickNamedAgentAction(agents, allOpts)` | 命名 Agent 一级选单 |
| `pickNamedAgentItemAction(id)` | 命名 Agent 二级操作菜单 |
| `editNamedAgent(agent, allOpts)` | 命名 Agent 编辑表单 |

`runStep1Providers` 和 `runStep4NamedAgents` 将更新调用逻辑。
