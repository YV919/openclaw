# 设计文档：表单导航全面修复

**日期**：2026-03-18
**状态**：已批准

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

- 不合并 Step 1-4 为单一 huh.Form（避免 WithHide 复杂度）
- 不改动 `internal/config/` 任何文件
- Step 1（Provider 管理）自身已是循环管理屏，不加跨步返回

---

## 架构

### 修改 1：`chineseKeyMap()`（`app.go`）

在现有 Select / MultiSelect 键绑定基础上，追加：

```go
km.Input.Next    = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
km.Input.Prev    = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Confirm.Next  = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
km.Confirm.Prev  = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Note.Next     = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
km.Form.NextGroup = key.NewBinding(key.WithKeys("enter"),       key.WithHelp("enter", "下一步"))
km.Form.PrevGroup = key.NewBinding(key.WithKeys("shift+tab"),   key.WithHelp("shift+tab", "上一步"))
```

### 修改 2：`Run()` 改为步骤导航循环（`app.go`）

**现状**：

```go
runStep1Providers(fullCfg)
runStep2MainAgent(fullCfg, ...)
runStep3SubAgent(fullCfg, ...)
runStep4NamedAgents(fullCfg, ...)
```

**改后**：

```go
step := 1
for step >= 1 && step <= 4 {
    var back bool
    var err error
    switch step {
    case 1:
        err = a.runStep1Providers(fullCfg)           // 无 back 返回值
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

注意：`allModelOpts` / `allModelOptsWithNone` 需在进入循环前构建（已在 `Run()` 中）；但 Step 1 可能修改 Providers，所以 Step 1 完成后要重新构建这两个选项列表，再进入 Step 2。

改为：

```go
step := 1
for step >= 1 && step <= 4 {
    // Step 1 结束后（或每次从 Step 2+ 返回 Step 1 后）重建模型选项
    if step == 2 {
        allModelOpts = buildAllModelOpts(fullCfg.Providers)
        allModelOptsWithNone = append(
            []huh.Option[string]{huh.NewOption("（不配置）", "")},
            allModelOpts...,
        )
    }
    ...
}
```

### 修改 3：`runStep2MainAgent` 签名改为返回 `(bool, error)`

在选项末尾加：

```go
opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
```

`huh.NewSelect` 的 `Value` 检测：若 `primary == "__back__"`，返回 `(true, nil)`。

### 修改 4：`runStep3SubAgent` 签名改为返回 `(bool, error)`

在第一个 Select（"同主 / 单独指定"）末尾加 `← 返回上一步`，检测后返回 `(true, nil)`。

### 修改 5：`runStep4NamedAgents` 签名改为返回 `(bool, error)`

在 `pickNamedAgentAction()` 的选单里加 `← 返回上一步`（值为 `"__back__"`），`runStep4NamedAgents` 收到该值后返回 `(true, nil)`。

---

## 数据流

```
Run()
  step=1 → runStep1Providers (内部循环，[继续→] 退出)
           ↓ 完成后重建 allModelOpts
  step=2 → runStep2MainAgent → 用户选模型 → [← 返回] → step=1
                                          → [确认]   → step=3
  step=3 → runStep3SubAgent  → 用户选子Agent → [← 返回] → step=2
                                             → [确认]   → step=4
  step=4 → runStep4NamedAgents → [← 返回] → step=3
                               → [继续→]   → step=5 (退出循环)
  → SaveFullConfig → printSuccess
```

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| 用户在 Step 2 返回，修改 Provider 新增模型 | step 回到 1，Step 1 退出后重建 allModelOpts，Step 2 拿到新选项 |
| 用户在 Step 3 返回后再返回 Step 1 | step-- 连续两次，allModelOpts 在 step==2 时重建 |
| Step 2 Select 中 primary 被设为 `__back__` | 不写入 fullCfg.MainAgent，直接返回 (true, nil) |
| Ctrl+C 中断 | `huh.ErrUserAborted` 仍由各步函数内部处理，`os.Exit(0)` |

---

## 改动文件清单

| 文件 | 改动内容 |
|------|----------|
| `app.go` | `chineseKeyMap()` 追加 7 个键绑定；`Run()` 改为步骤循环；`runStep2-4` 签名加 `bool` 返回值；各选单加"← 返回上一步"选项 |

无其他文件改动。

---

## 测试策略

- `go run .` 手动验证：
  - editProvider 内 Shift+Tab 可在字段间后退
  - Step 2/3/4 底部显示中文"enter 下一步"
  - Step 2/3/4 选"← 返回上一步"可回到前一步
  - Provider 修改后返回 Step 2，模型列表正确更新
- 现有单元测试（`app_test.go`, `manager_test.go`）不受影响，直接 `go test ./...` 通过
