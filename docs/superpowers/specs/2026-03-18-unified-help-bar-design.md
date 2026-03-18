# 统一帮助栏设计文档

**日期**：2026-03-18
**状态**：待实施
**模块**：`app.go`

---

## 背景与问题

openclaw-config 是一个基于 `charmbracelet/huh v0.6.0` 的终端 TUI 配置工具。当前帮助栏存在两个问题：

1. **信息割裂**：`shift+tab 返回上一字段 · Ctrl+C 取消编辑` 被手动嵌入 `editProvider` 第一个 Input 字段的 `Description` 文本中，而 `enter 下一项` 由 huh 渲染在底部帮助栏，两者位置不一致。
2. **覆盖不全**：其余页面（Step2-4 的 Select、editNamedAgent 等）没有任何导航提示。

### huh 底层约束（已验证于 v0.6.0 源码）

- `group.go:345`：帮助栏内容 = `g.help.ShortHelpView(field.KeyBinds())`，仅含**字段级**按键绑定
- `field_input.go:484`：`Prev.SetEnabled(!p.IsFirst())` — 第一个字段的 `shift+tab` 被禁用，从帮助栏消失
- `keymap.go:109`：`Quit`（`ctrl+c`）属于**表单级**，不在 `KeyBinds()` 中，永远不会出现在帮助栏
- `form.go:610`：`if f.quitting { return "" }` — 表单退出时 `View()` **确定返回空字符串**（已验证）
- `form.go:72`：`State FormState` 为**公开字段**；`StateAborted` / `StateCompleted` 为公开常量

因此，无法通过修改 KeyMap 实现帮助栏统一，需要 bubbletea 包装方案。

---

## 目标

所有表单页面在底部展示统一的固定帮助栏：

```
  ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认
```

替代当前 Description 中的手动文字补丁，且所有页面一致。

---

## 方案：bubbletea 包装器

### 架构

```
┌─────────────────────────────────┐
│   tea.NewProgram(formModel)     │
│   ┌─────────────────────────┐   │
│   │   huh.Form.View()       │   │  ← huh 渲染区（关闭原生 help bar）
│   └─────────────────────────┘   │
│   ctrl+c 取消 · shift+tab ↑ · enter 确认  │  ← 固定 footer
└─────────────────────────────────┘
```

### 核心类型

```go
type formModel struct {
    form *huh.Form
}
```

注：不需要手动追踪 `aborted` 标志，直接使用 `form.State`（公开字段）判断退出原因。

完整方法实现：

```go
func (m *formModel) Init() tea.Cmd {
    return m.form.Init()
}

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // huh.Form.Update 返回新的 tea.Model，必须类型断言并回写 m.form，
    // 否则表单状态永远不会推进。
    // bubbletea 事件循环为单线程，无并发竞态风险。
    form, cmd := m.form.Update(msg)
    m.form = form.(*huh.Form)
    return m, cmd
}

func (m *formModel) View() string {
    v := m.form.View()
    if v == "" {
        return "" // form.go:610 确认：quitting 时 View() 返回空字符串
    }
    return v + "\n" + helpFooter
}
```

### footer 样式

```go
var helpFooter = lipgloss.NewStyle().
    Foreground(lipgloss.Color("241")).
    Render("  ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认")
```

颜色 `241`（dim）与 huh 原生帮助栏视觉一致。

### runForm 函数

```go
func runForm(form *huh.Form) error
```

实现逻辑：

```go
func runForm(form *huh.Form) error {
    form.WithShowHelp(false)
    m := &formModel{form: form}
    result, err := tea.NewProgram(m).Run()
    if err != nil {
        return err  // 透传 tea/terminal 层错误
    }
    fm := result.(*formModel)
    if fm.form.State == huh.StateAborted {
        return huh.ErrUserAborted
    }
    return nil
}
```

**错误路径**：

| 路径 | 触发 | `form.State` | `runForm` 返回 |
|------|------|------------|---------------|
| 正常提交 | 最后字段 `enter` | `StateCompleted` | `nil` |
| 用户取消 | `ctrl+c` | `StateAborted` | `huh.ErrUserAborted` |
| I/O 或 terminal 错误 | 系统错误 | 不适用 | 透传原始 `error` |

### 已知设计权衡

**单字段表单的 `shift+tab` 提示**：对于只有一个字段的表单（如 `pickProviderAction`、`pickProviderItemAction`），footer 中的 `shift+tab 上一项` 在语义上无效（该字段是第一也是最后一个字段）。此为**可接受的权衡**：提示文字提供的是整体导航上下文，即在多步骤流程中可用 `shift+tab` 返回上一步（通过返回菜单项实现）；保持 footer 一致性优于逐表单定制。

---

## 改动范围

### 修改文件：`app.go`

**新增代码**（`newForm()` 附近）：
- `helpFooter` 包级 `var`
- `formModel` 结构体及 `Init`、`Update`、`View` 方法
- `runForm(form *huh.Form) error` 函数

**修改 `newForm()`**：追加 `.WithShowHelp(false)` 关闭 huh 原生帮助栏。

**删除 Description 中的导航提示文字**：`editProvider` 第一个 Input 字段的 Description（当前第 378 行），将 `\n· shift+tab 返回上一字段  · Ctrl+C 取消编辑` 从末尾删除。

**替换 `form.Run()` → `runForm(form)` 的完整调用点列表**（`app.go` 共 11 处）：

| 函数 | 变量名 | 行（近似） |
|------|--------|-----------|
| `deleteProvider` | `form` | ~172 |
| `pickProviderAction` | `form` | ~273 |
| `pickProviderItemAction` | `form` | ~296 |
| `editProvider` | `form` | ~437 |
| `runStep2MainAgent` | `form` | ~505 |
| `runStep3SubAgent` | `form1` | ~542 |
| `runStep3SubAgent` | `form2` | ~580 |
| `pickNamedAgentAction` | `form` | ~613 |
| `pickNamedAgentItemAction` | `form` | ~636 |
| `editNamedAgent` | `form` | ~670 |
| `printSuccess` | （无变量，内联调用） | ~778 |

**`printSuccess` 特殊处理**：当前为 `newForm(...).Run()`（链式调用，无中间变量）。改为：
```go
f := newForm(...)
runForm(f) //nolint:errcheck  // 保留：成功提示页为纯展示，忽略退出错误符合原有意图
```

**ErrUserAborted 行为说明**：所有调用方（`pickProviderAction` 等）已有 `errors.Is(err, huh.ErrUserAborted)` 检查并调用 `os.Exit(0)`。`runForm()` 返回 `huh.ErrUserAborted` 的路径与原 `form.Run()` 完全一致，行为不变。

**formModel 测试说明**：`formModel` 是纯委托层（仅转发调用 + 拼接字符串），无独立业务逻辑，不需要单独单元测试。行为正确性由 `go test ./...`（集成级）覆盖。

### 依赖说明

- `github.com/charmbracelet/bubbletea v1.1.0`：当前在 `go.mod` 中为 `// indirect`。`app.go` 直接 import 后需执行 `go mod tidy` 将其提升为直接依赖（`go.sum` 和 `go.mod` 均需更新）。
- `github.com/charmbracelet/lipgloss v0.13.0`：已为直接依赖，无需变更。

### 不改动

- `chineseKeyMap()` 键绑定定义
- `internal/config/` 目录所有文件
- `internal/models/` 目录所有文件

---

## 验收标准

- [ ] 所有表单页面底部均显示：`ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认`
- [ ] `editProvider` 第一个字段的 Description 中不再包含导航提示文字
- [ ] 原生 huh 帮助栏（`enter 下一项`）不再单独显示
- [ ] `ctrl+c` 取消行为与现有逻辑兼容（上层 `errors.Is(err, huh.ErrUserAborted)` 无需修改）
- [ ] `runStep3SubAgent` 的 `form1` 和 `form2` 两处均已替换
- [ ] `printSuccess` 的内联 `Run()` 已改为两行形式
- [ ] `go mod tidy` 已执行，`go.mod` 中 bubbletea 为直接依赖
- [ ] 所有现有测试通过（`go test ./...`）
