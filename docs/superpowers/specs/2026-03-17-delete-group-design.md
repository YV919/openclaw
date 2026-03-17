# 设计文档：Provider 与命名 Agent 的删除功能

**日期：** 2026-03-17
**状态：** 已批准（第三次修订）
**范围：** `app.go` + `internal/config/manager.go`

---

## 背景

当前工具支持添加和编辑 Provider，支持添加命名 Agent（upsert），但均无删除功能。用户需要能够删除不再需要的 Provider 和命名 Agent。

---

## 目标

1. Provider 管理（Step 1）：增加删除功能，删除前检查依赖并警告
2. 命名 Agent 管理（Step 4）：增加编辑和删除功能，重构为与 Step 1 一致的管理环
3. 确保删除操作真实落盘：修复 `manager.go` 中 `SaveFullConfig` 的持久化逻辑

---

## 方案选择

采用**二级操作菜单**模式：

- 第一级：选择要操作的对象（已有 Provider/Agent 列表 + 添加 + 继续）
- 第二级：选择操作（编辑 / 删除 / 返回）

---

## 详细设计

### 一、app.go — Step 1：Provider 管理

**`pickProviderAction` 主菜单标签修改：**

现有标签格式 `[编辑] name (url)` 在引入二级菜单后语义不准确，需改为 `name  (url)`（去掉 `[编辑]` 前缀），Title/Description 改为"Provider 管理 — 选择要操作的 Provider，或添加新的"。

**`Run()` 中 allModelOpts 重建：**

Step 1（`runStep1Providers`）结束后、进入 Step 2/3/4 之前，必须重新调用 `buildAllModelOpts` 重建 `allModelOpts` 和 `allModelOptsWithNone`，确保删除 Provider 后下拉列表不包含已删除 Provider 的模型。原来在 Step 1 和 Step 2 之间一次性构建的代码（`app.go` 第 62-66 行）需移到 `runStep1Providers` 返回之后。

**变更后的 `runStep1Providers` 主循环伪代码：**

```
for {
    action = pickProviderAction(fullCfg.Providers)  // 返回 name / __add__ / __continue__
    if action == "__continue__" { break }
    if action == "__add__" {
        p = editProvider(空 ProviderConfig)
        fullCfg.Providers = append(..., p)
    } else {
        // 选中已有 provider → 弹二级菜单
        subAction = pickProviderItemAction(action)   // 返回 __edit__ / __delete__ / __back__
        if subAction == "__edit__" {
            updated = editProvider(当前 provider)
            fullCfg.Providers[i] = updated
        } else if subAction == "__delete__" {
            deleteProvider(fullCfg, action)
        }
        // __back__ 直接回到外层 for 继续
    }
}
```

**新增函数：**

`pickProviderItemAction(name string) (string, error)`
- 弹出 Select，选项：`[编辑 name]` / `[删除 name]` / `[← 返回]`
- 返回 `"__edit__"` / `"__delete__"` / `"__back__"`

`deleteProvider(fullCfg *config.FullConfig, name string) error`
- 调用 `checkProviderDeps(name, fullCfg)` 获取 `deps []string`
- 若 `len(deps) > 0`：用黄色文字逐行列出受影响项
- 弹出 `huh.NewConfirm`（"确认删除 provider？"）
- 确认后：
  1. 从 `fullCfg.Providers` 中移除 `name`
  2. 遍历 `fullCfg.NamedAgents`：若 `Model.Primary` 或 `Model.Fallback` 以 `name+"/"` 开头，则将其**同时清空**为 `""`
  3. 注：`MainAgent`/`SubAgent` 的悬空引用由 Step 2/3 的 `containsOptValue` 检查自动重置，无需在此处理

`checkProviderDeps(providerName string, cfg *config.FullConfig) []string`
- 检查 `cfg.MainAgent.Primary/Fallback`（前缀 `providerName+"/"`）
- 检查 `cfg.SubAgent.Primary/Fallback`
- 遍历 `cfg.NamedAgents`，检查 `Model.Primary/Fallback`
- 返回描述列表，如 `["主 Agent (primary)", "named:my-coder (primary)"]`

---

### 二、app.go — Step 4：命名 Agent 管理

**变更后的 `runStep4NamedAgents` 主循环伪代码：**

```
for {
    action = pickNamedAgentAction(fullCfg.NamedAgents, allOpts)
    // 返回 agent ID / __add__ / __continue__

    if action == "__continue__" { break }

    if action == "__add__" {
        id, model = addNamedAgentForm(allOptsWithSame)
        upsert id+model 到 fullCfg.NamedAgents
    } else {
        // 选中已有 Agent → 弹二级菜单
        subAction = pickNamedAgentItemAction(action)
        if subAction == "__edit__" {
            updated = editNamedAgent(当前 agent, allOptsWithSame)
            fullCfg.NamedAgents[i] = updated
        } else if subAction == "__delete__" {
            从 fullCfg.NamedAgents 中移除 action
        }
        // __back__ 直接继续外层 for
    }
}
```

**注：** `[继续 →]` 选项在每次循环重新调用 `pickNamedAgentAction` 时，根据当时 `fullCfg.NamedAgents` 是否非空来决定是否加入选项列表（动态刷新）。

**新增函数：**

`pickNamedAgentAction(agents []config.NamedAgentConfig, allOpts []huh.Option[string]) (string, error)`
- 为每个 agent 生成选项：`[选择] agentID  (model.primary 或 "同主 Agent")`
- 附加 `[+ 添加新命名 Agent]`（`__add__`）
- 若 `len(agents) > 0` 则附加 `[继续 →]`（`__continue__`）

`pickNamedAgentItemAction(id string) (string, error)`
- 弹出 Select，选项：`[编辑 id]` / `[删除 id]` / `[← 返回]`
- 返回 `"__edit__"` / `"__delete__"` / `"__back__"`

`editNamedAgent(agent config.NamedAgentConfig, allOptsWithSame []huh.Option[string], allOptsWithNone []huh.Option[string]) (config.NamedAgentConfig, error)`
- `huh.NewNote` 展示 Agent ID（只读）
- `huh.NewSelect` 选择 Primary 模型（复用 `allOptsWithSame`，含"同主 Agent"选项）
- `huh.NewSelect` 选择 Fallback 模型（复用 `allOptsWithNone`，含"（不配置）"选项，与 Step 2/3 对称）
- 返回更新后的 `NamedAgentConfig`（ID 不变，更新 `Model.Primary` 和 `Model.Fallback`）
- 支持 Fallback，保持与 Step 2/3 的字段对称性；也使 deleteProvider 清空的 Fallback 可以在此恢复

---

### 三、manager.go — SaveFullConfig 持久化修复

> **注：** `manager.go` 原标注"勿改动"，以下两处是实现删除功能的必要前提，需要修改。

#### 3.1 agents.list 改为先过滤删除再 upsert

**当前代码（第 651-698 行）问题：**
- `if len(cfg.NamedAgents) > 0` 条件守卫：当所有命名 Agent 均被删除时，`existingList` 不会被更新
- 只做 upsert，不删除已从 `cfg.NamedAgents` 移除的条目

**修改后的伪代码（替换现有 agents.list 写入块）：**

```
// 1. 构建本轮管理的 ID 集合
managedIDs = set of na.ID for na in cfg.NamedAgents

// 2. 过滤 existingList：
//    保留条目 = "不含 model 字段的条目"（非本工具管理，原样保留）
//           OR "ID 在 managedIDs 中的含 model 字段条目"（后续 upsert 更新）
//    删除条目 = "含 model 字段" AND "ID 不在 managedIDs 中"（本工具管理且已被用户删除）
filteredList = []
for item in existingList:
    _, hasModel = item["model"]
    if !hasModel {
        filteredList = append(filteredList, item)  // 非本工具管理，保留
    } else if managedIDs 包含 item["id"] {
        filteredList = append(filteredList, item)  // 本工具管理且仍在列表中，保留
    }
    // 否则（含 model 且 ID 不在 managedIDs）：丢弃

// 3. 重建 indexByID（基于 filteredList）

// 4. 对 cfg.NamedAgents 执行 upsert（逻辑与原代码相同）

// 5. agents["list"] = filteredList（始终写入，不受 if len > 0 守卫）
```

#### 3.2 saveAllApiKeys 增加孤立条目清理

**修改后的伪代码（在写入新条目前插入清理步骤）：**

```
// 0. 构建当前 provider 名称集合
currentNames = set of p.Name for p in providers

// 1. 清理孤立条目（新增）
// 判断方式：以最后一个 ":" 分割 key，若后缀为 "default" 且前缀不在 currentNames 中则删除
for key in profiles:
    parts = strings.LastIndex(key, ":")
    if parts > 0 AND key[parts+1:] == "default" AND key[:parts] 不在 currentNames 中:
        delete(profiles, key)

// 2. 原有写入逻辑（不变）
for p in providers:
    profiles[p.Name+":default"] = {...}
```

---

## 边界情况

| 情况 | 处理方式 |
|------|----------|
| 删除 Provider 后，MainAgent/SubAgent 模型引用悬空 | Step 1 结束后重建 allModelOpts，Step 2/3 通过 `containsOptValue` 检查自动重置为第一个可用模型 |
| 删除 Provider 后，NamedAgent 模型引用悬空 | `deleteProvider` 时主动将相关 NamedAgent 的 `Model.Primary` 和 `Model.Fallback` 同时清空为 `""` |
| 删除最后一个 Provider | 允许删除；Step 2/3 将因 allOpts 为空而无法选模型（后续版本可加前置校验） |
| 删除所有命名 Agent 后调用 SaveFullConfig | `filteredList` 为空，仍会写入 `agents["list"] = []`，正确清空 |
| 命名 Agent 列表为空进入 Step 4 | `pickNamedAgentAction` 只显示 `[+ 添加]`，无 `[继续 →]` |
| 添加第一个 Agent 后继续循环 | 下次调用 `pickNamedAgentAction` 时 `len(agents) > 0`，`[继续 →]` 动态出现 |
| agents.list 中存在非本工具创建的条目（无 model 字段） | 不含 `model` 字段的条目原样保留，不被过滤 |
| 用户在任意表单按 Esc/Ctrl+C | 维持现有行为：打印"已取消"并 `os.Exit(0)` |

---

## 文件变更范围

### `app.go` — 函数变更

| 函数 | 变更类型 | 说明 |
|------|----------|------|
| `runStep1Providers` | 修改 | 添加二级菜单调用分支 |
| `runStep4NamedAgents` | 重写 | 移除前置 Confirm，改为管理环 |
| `Run()` | 修改 | Step 1 结束后重建 `allModelOpts` / `allModelOptsWithNone` |
| `pickProviderAction` | 修改 | 主菜单标签改为 `name (url)`，去掉 `[编辑]` 前缀 |
| `pickProviderItemAction(name string) (string, error)` | 新增 | Provider 二级操作菜单 |
| `deleteProvider(fullCfg *config.FullConfig, name string) error` | 新增 | 删除逻辑（含警告 + NamedAgent 引用清空） |
| `checkProviderDeps(name string, cfg *config.FullConfig) []string` | 新增 | 依赖检查 |
| `pickNamedAgentAction(agents []config.NamedAgentConfig, allOpts []huh.Option[string]) (string, error)` | 新增 | 命名 Agent 一级选单 |
| `pickNamedAgentItemAction(id string) (string, error)` | 新增 | 命名 Agent 二级操作菜单 |
| `editNamedAgent(agent config.NamedAgentConfig, allOptsWithSame []huh.Option[string], allOptsWithNone []huh.Option[string]) (config.NamedAgentConfig, error)` | 新增 | 编辑表单（ID 只读，支持 Primary + Fallback） |

### `internal/config/manager.go` — 函数变更

| 函数 | 变更类型 | 说明 |
|------|----------|------|
| `SaveFullConfig` | 修改 | agents.list 先过滤删除再 upsert，移除 if len > 0 守卫 |
| `saveAllApiKeys` | 修改 | 写入前清理孤立 profileKey |
