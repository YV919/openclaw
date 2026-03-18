# 设计文档：表单导航全面修复

**日期**：2026-03-18
**状态**：设计完成，待实现（源代码当前未包含此改动）

---

## 背景

openclaw-config TUI 存在三个导航缺陷：

1. **表单内字段无法后退**：`editProvider` 等多字段表单中，`chineseKeyMap()` 缺少 `Input.Prev`，Shift+Tab 在输入框内无效。
2. **步骤间无法返回**：Step 1→2→3→4 是顺序独立的 `form.Run()` 调用，一旦提交即无法回退。
3. **`enter next` 无中文注释**：`chineseKeyMap()` 未设置 `km.Form.NextGroup`，底部显示默认英文 `enter next`。

---

## 目标

1. 所有字段类型（Input / Confirm / Note / Select / MultiSelect）的导航键全部显示中文提示。
2. 在 Step 2、3、4 的入口选单加入"← 返回上一步"选项，允许用户回到前一步。
3. `editProvider` 内部字段可用 Shift+Tab 向上逐字段后退。

---

## 不做的事

- 不合并 Step 1-4 为单一 huh.Form
- 不改动 `internal/config/` 任何文件
- Ctrl+C 在任何步骤仍直接 `os.Exit(0)`（保持现有行为，不改为"返回上一步"）
- Step 1（Provider 管理）自身已是循环管理屏，不加跨步返回入口

---

## 架构

### 修改 1：`chineseKeyMap()`（`app.go`）

在现有 Select / MultiSelect 键绑定基础上，追加：

```go
km.Input.Next     = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
km.Input.Prev     = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Confirm.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
km.Confirm.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Note.Next      = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
km.Form.NextGroup = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "下一步"))
km.Form.PrevGroup = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一步"))
```

huh v0.6.0 中，`km.Input.Prev` 绑定的 `shift+tab` 在多字段 Group 内触发"跳转到前一字段"，不会在 Input 内部向左移动光标（光标移动由方向键控制）。

### 修改 2：`Run()` 改为步骤导航循环（`app.go`）

**现状**（顺序调用）：

```go
runStep1Providers(fullCfg)
runStep2MainAgent(fullCfg, ...)
runStep3SubAgent(fullCfg, ...)
runStep4NamedAgents(fullCfg, ...)
```

**改后**（步骤循环）：

```go
step := 1
for step >= 1 && step <= 4 {
    var back bool
    var err error

    // 每次进入 Step 2 前重建模型选项列表（Step 1 可能修改了 Provider）
    if step == 2 {
        allModelOpts = buildAllModelOpts(fullCfg.Providers)
        allModelOptsWithNone = append(
            []huh.Option[string]{huh.NewOption("（不配置）", "")},
            allModelOpts...,
        )
    }

    switch step {
    case 1:
        err = a.runStep1Providers(fullCfg)
        // Step 1 无 back 返回值：[继续→] 退出内部循环即代表前进
    case 2:
        back, err = a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone)
    case 3:
        back, err = a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone)
    case 4:
        back, err = a.runStep4NamedAgents(fullCfg, allModelOpts)
    }

    if err != nil {
        return err
    }
    if back {
        step--
    } else {
        step++
    }
}
```

**关键点**：`allModelOpts` 的重建只需在进入 Step 2 前执行。无论用户路径是 1→2、2→1→2 还是 3→2→1→2，只要 step 变为 2，都会重建。这覆盖了所有 Provider 修改场景。

**Provider 为空时的处理**：若 `allModelOpts` 为空（用户删完了所有 Provider），Step 2/3 的 Select 将无选项，huh 会触发空列表行为（不崩溃，用户只能 Ctrl+C 退出或返回 Step 1 补充 Provider）。这与现有行为一致，无需额外处理。

### 修改 3：`runStep2MainAgent` 签名改为 `(bool, error)`

```go
func (a *App) runStep2MainAgent(...) (bool, error)
```

在 Select 选项末尾追加返回选项（仅当 Step 能回退时，即始终追加，因为 Step 2 前面始终有 Step 1）：

```go
opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
```

`form.Run()` 后，**在写入 `fullCfg.MainAgent` 前**先检测特殊值：

```go
if primary == "__back__" {
    return true, nil  // 不写入配置
}
fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
return false, nil
```

### 修改 4：`runStep3SubAgent` 签名改为 `(bool, error)`

```go
func (a *App) runStep3SubAgent(...) (bool, error)
```

在第一个 Select（"同主 / 单独指定"）末尾追加：

```go
huh.NewOption("← 返回上一步", "__back__")
```

`form1.Run()` 后检测：

```go
if subChoice == "__back__" {
    return true, nil
}
```

后续第二个 Select（Primary + Fallback）不需要返回选项，因为用户可以 Shift+Tab 回到第一个 Select，再选"← 返回"。

### 修改 5：`runStep4NamedAgents` 签名改为 `(bool, error)`

```go
func (a *App) runStep4NamedAgents(...) (bool, error)
```

在 `pickNamedAgentAction()` 的选单里追加（放在 `[继续 →]` 之前）：

```go
opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
```

`runStep4NamedAgents` 收到 `"__back__"` 时返回 `(true, nil)`，收到 `"__continue__"` 时返回 `(false, nil)`。

---

## 数据流

```
Run()
  step=1 → runStep1Providers  (内部循环，[继续→] 退出)
           ↓ step++ → step=2
  step=2 → 重建 allModelOpts
         → runStep2MainAgent
           [← 返回] → back=true → step-- → step=1 → 重走 Step 1
           [确认]   → back=false → step++ → step=3
  step=3 → runStep3SubAgent
           [← 返回] → back=true → step-- → step=2 → 重建 allModelOpts → runStep2MainAgent
           [确认]   → back=false → step++ → step=4
  step=4 → runStep4NamedAgents
           [← 返回] → back=true → step-- → step=3 → runStep3SubAgent
           [继续→]  → back=false → step++ → step=5 (退出循环)
  → SaveFullConfig → printSuccess
```

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| 用户从 Step 2 返回 Step 1，修改 Provider 新增模型 | Step 1 退出后 step=2，重建 allModelOpts，Step 2 拿到新选项 |
| 用户连续返回：Step 4→3→2→1 | step 依次递减，每次进入 step=2 前重建 allModelOpts |
| Provider 列表为空（全部删除） | allModelOpts=[]，Step 2/3 Select 无选项，用户只能 Ctrl+C 或返回 Step 1 |
| Ctrl+C 在任何步骤 | `huh.ErrUserAborted` → `os.Exit(0)`，直接退出（保持现有行为） |
| `primary == "__back__"` 写入风险 | 在写入 `fullCfg.MainAgent` 前检测，提前返回 `(true, nil)` |
| editProvider 多字段 Shift+Tab | huh v0.6.0 在单 Group 内 `km.Input.Prev` 触发跳转到前一字段，不会越出 Group |

---

## 改动文件清单

| 文件 | 改动内容 |
|------|----------|
| `app.go` | `chineseKeyMap()` 追加 7 个键绑定；`Run()` 改为步骤循环（含 allModelOpts 重建）；`runStep2/3/4` 签名从 `error` 改为 `(bool, error)`；各选单追加"← 返回上一步"选项；写入配置前检测 `__back__` |

无其他文件改动。

---

## 测试策略

手动运行 `go run .` 验证：
- `editProvider` 内 Shift+Tab 可在字段间后退（Input → MultiSelect → Input）
- 所有表单底部显示中文键提示（无英文 `enter next`）
- Step 2/3/4 选"← 返回上一步"正确回退
- Step 1 修改 Provider 后进 Step 2，模型列表正确更新
- Ctrl+C 仍然直接退出

现有单元测试 `go test ./...` 不受影响（不测试 TUI 交互）。
