# 设计文档：表单导航全面修复

**日期**：2026-03-18
**状态**：修订版 v3（最终稿）

---

## 背景

openclaw-config TUI 存在三类导航缺陷：

1. **`enter next` 无中文注释**：`chineseKeyMap()` 已覆盖 `Input.Next`（非末字段），但 huh v0.6.0 对末字段单独使用 `Input.Submit` binding（`Submit` 启用，`Next` 禁用），而 `Submit` 未被覆盖，导致帮助栏显示默认英文 "next"/"submit"。`Select`、`MultiSelect`、`Note`、`Confirm` 同理。
2. **步骤间无法返回**：Step 1→2→3→4 为顺序独立的 `form.Run()` 调用，提交后无法回退。（Step 2-4 已实现，本次无需修改）
3. **`editProvider` / `editNamedAgent` 无法取消返回上级菜单**：Ctrl+C 时直接 `os.Exit(0)`，无法回到前一选单。另外帮助栏未显示 `shift+tab 上一字段` 提示，用户不知道可以后退。

---

## 目标

1. 覆盖所有字段类型的 `Submit` 中文绑定，彻底消除英文帮助栏文本。
2. `editProvider` / `editNamedAgent` 支持 Ctrl+C 取消返回上级菜单（不退出程序）。
3. `editProvider` 第一个字段的 Description 追加 `shift+tab` 和 `Ctrl+C` 使用提示。

---

## 不做的事

- 不合并 Step 1-4 为单一 huh.Form
- 不改动 `internal/config/` 任何文件
- Step 1（Provider 管理）自身已是循环管理屏，不加跨步返回入口
- 不把 `editProvider` 拆分为多屏（保持五字段单表单）
- `Pick*` 选单（`pickProviderAction` 等）是流程入口，Ctrl+C 仍直接 `os.Exit(0)`

---

## 实现状态

| 修改项 | 状态 |
|--------|------|
| `km.Input/Confirm/Note/Select/MultiSelect.Next/Prev` 中文 KeyMap | ✅ 已实现 |
| `Run()` 步骤导航循环 | ✅ 已实现 |
| Step 2/3/4 "← 返回上一步" | ✅ 已实现 |
| 各字段类型的 `Submit` 中文绑定 | ❌ 未实现（本次修改 1） |
| `editProvider` 取消返回上级 | ❌ 未实现（本次修改 2） |
| `editNamedAgent` 取消返回上级 | ❌ 未实现（本次修改 3） |
| `editProvider` shift+tab/Ctrl+C 提示文字 | ❌ 未实现（本次修改 4） |

---

## 架构

### 修改 1：`chineseKeyMap()` 追加各字段的 `Submit` 中文绑定（`app.go`）

在已有 `Next`/`Prev` 键绑定末尾追加：

```go
km.Input.Submit       = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "提交"))
km.Select.Submit      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
km.MultiSelect.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
km.Note.Submit        = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "继续"))
km.Confirm.Submit     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
```

**原因**：huh v0.6.0 区分 `Next`（非末字段）和 `Submit`（末字段）：

```go
// field_input.go:484-486（field_select.go / field_multiselect.go 同理）
i.keymap.Next.SetEnabled(!p.IsLast())    // 末字段：Next 禁用
i.keymap.Submit.SetEnabled(p.IsLast())   // 末字段：Submit 启用
```

huh v0.6.0 的 `KeyMap` 结构体中**不存在** `Form` 级子结构，`km.Form.NextGroup` 等字段编译报错。

---

### 修改 2：`editProvider` 改为 `(config.ProviderConfig, bool, error)`（`app.go`）

**现状签名**：
```go
func editProvider(p config.ProviderConfig) (config.ProviderConfig, error)
```

**改后签名**：
```go
func editProvider(p config.ProviderConfig) (config.ProviderConfig, bool, error)
```

- `huh.ErrUserAborted`（Ctrl+C）→ 返回 `(config.ProviderConfig{}, true, nil)`，**不**调用 `os.Exit(0)`
- 表单正常提交 → 返回 `(result, false, nil)`
- 其他错误 → 返回 `(config.ProviderConfig{}, false, err)`

调用方 `runStep1Providers` 两处均检查 `cancelled`：

```go
// __add__ 分支
p, cancelled, err := editProvider(config.ProviderConfig{})
if err != nil { return err }
if !cancelled {
    fullCfg.Providers = append(fullCfg.Providers, p)
}

// __edit__ 分支
updated, cancelled, err := editProvider(p)
if err != nil { return err }
if !cancelled {
    fullCfg.Providers[i] = updated
}
```

---

### 修改 3：`editNamedAgent` 改为 `(config.NamedAgentConfig, bool, error)`（`app.go`）

场景与 `editProvider` 相同——用户在编辑命名 Agent 模型时可能想取消返回列表。

**现状签名**：
```go
func editNamedAgent(agent config.NamedAgentConfig, ...) (config.NamedAgentConfig, error)
```

**改后签名**：
```go
func editNamedAgent(agent config.NamedAgentConfig, ...) (config.NamedAgentConfig, bool, error)
```

- `huh.ErrUserAborted` → 返回 `(config.NamedAgentConfig{}, true, nil)`，不 `os.Exit(0)`
- 正常提交 → 返回 `(result, false, nil)`

调用方 `runStep4NamedAgents` 的 `__edit__` 分支检查 `cancelled`：

```go
updated, cancelled, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
if err != nil { return false, err }
if !cancelled {
    fullCfg.NamedAgents[i] = updated
}
```

---

### 修改 4：`editProvider` 第一个字段 Description 追加提示（`app.go`）

"Provider 标识名" Input 字段的 Description 追加操作提示：

```go
Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）\n· shift+tab 返回上一字段  · Ctrl+C 取消编辑")
```

---

## 数据流

```
Run()
  step=1 → runStep1Providers（内部循环，[继续→] 退出）
    ├─ __add__  → editProvider(空)   → cancelled=true  → 跳过追加，继续循环
    │                                → cancelled=false → append，继续循环
    └─ __edit__ → editProvider(现有) → cancelled=true  → 跳过更新，继续循环
                                     → cancelled=false → 更新 Providers[i]，继续循环
  step=4 → runStep4NamedAgents（内部循环）
    └─ __edit__ → editNamedAgent(现有) → cancelled=true  → 跳过更新，继续循环
                                        → cancelled=false → 更新 NamedAgents[i]，继续循环
```

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| 用户在 `editProvider` 里按 Ctrl+C | 返回 `cancelled=true`，跳过保存，回到 Provider 列表循环 |
| 用户在 `editNamedAgent` 里按 Ctrl+C | 返回 `cancelled=true`，跳过保存，回到命名 Agent 列表循环 |
| Ctrl+C 在 `Pick*` 选单或 Step 2/3/4 | `huh.ErrUserAborted` → `os.Exit(0)`（保持现有行为） |
| Ctrl+C 在 `deleteProvider` 确认框 | `deleteProvider` 已对 ErrUserAborted 做 `return nil`，无需修改 |
| `primary == "__back__"` 写入风险 | 在写入 `fullCfg.MainAgent` 前检测，提前返回 `(true, nil)` |
| editProvider 多字段 Shift+Tab | huh v0.6.0 在单 Group 内 `km.Input.Prev` 触发跳转到前一字段 |
| Provider 列表为空（全部删除） | allModelOpts=[]，Step 2/3 Select 无选项，用户只能 Ctrl+C 或返回 Step 1 |

---

## 改动文件清单

| 文件 | 改动内容 |
|------|----------|
| `app.go` | `chineseKeyMap()` 追加 5 个 `Submit` 中文绑定；`editProvider` 签名改为 `(ProviderConfig, bool, error)`；`editNamedAgent` 签名改为 `(NamedAgentConfig, bool, error)`；`runStep1Providers` 两处调用检查 `cancelled`；`runStep4NamedAgents` 的 `__edit__` 分支检查 `cancelled`；"Provider 标识名" Description 追加操作提示 |

无其他文件改动。

---

## 测试策略

手动运行 `go run .` 验证：
- 所有表单底部显示中文键提示（无英文 "enter next" / "enter submit"）
- `editProvider` 按 Ctrl+C 后回到 Provider 列表，不退出程序
- `editNamedAgent` 按 Ctrl+C 后回到命名 Agent 列表，不退出程序
- `editProvider` 内 Shift+Tab 可在字段间后退
- "Provider 标识名" 字段 Description 显示 shift+tab / Ctrl+C 提示
- Step 2/3/4 选"← 返回上一步"正确回退
- Ctrl+C 在 Step 2/3/4 仍然直接退出程序

现有单元测试 `go test ./...` 不受影响（不测试 TUI 交互）。
