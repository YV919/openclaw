# 统一帮助栏 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 bubbletea 包装器在所有表单页面底部统一显示 `ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认`，替代当前 `editProvider` Description 中的手动文字补丁。

**Architecture:** 新增 `formModel` 类型（实现 `tea.Model` 接口）包裹 `huh.Form`，在 `View()` 末尾追加 footer；新增 `runForm()` 函数替代所有 `form.Run()` 调用；`newForm()` 追加 `.WithShowHelp(false)` 关闭 huh 原生帮助栏。

**Tech Stack:** Go 1.23，charmbracelet/huh v0.6.0，charmbracelet/bubbletea v1.1.0（已在 go.mod 中），charmbracelet/lipgloss v0.13.0

**Spec:** `docs/superpowers/specs/2026-03-18-unified-help-bar-design.md`

---

## 文件改动地图

| 文件 | 操作 | 内容 |
|------|------|------|
| `app.go` | 修改 | 新增 `helpFooter`、`formModel`、`runForm()`；修改 `newForm()`；替换 11 处 `form.Run()`；删除 Description 中的导航提示文字 |
| `go.mod` / `go.sum` | 修改 | bubbletea 从 indirect 升为 direct（由 `go mod tidy` 自动完成，两个文件均需提交） |

---

## Task 1：新增 `formModel` + `runForm()`

**Files:**
- Modify: `app.go`（在 `newForm()` 函数上方新增代码）

- [ ] **Step 1：在 `app.go` 中，找到 `newForm()` 函数（约第 337 行），在其正上方插入以下代码**

  ```go
  // helpFooter 是统一显示在所有表单底部的导航提示栏
  var helpFooter = lipgloss.NewStyle().
  	Foreground(lipgloss.Color("241")).
  	Render("  ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认")

  // formModel 将 huh.Form 包裹为 tea.Model，在 View() 底部追加固定帮助栏。
  // 替代 form.Run()，配合 runForm() 使用。
  type formModel struct {
  	form *huh.Form
  }

  func (m *formModel) Init() tea.Cmd {
  	return m.form.Init()
  }

  func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  	// huh.Form.Update 返回新的 tea.Model，必须类型断言并回写 m.form，
  	// 否则表单状态永远不会推进。bubbletea 事件循环为单线程，无竞态风险。
  	form, cmd := m.form.Update(msg)
  	m.form = form.(*huh.Form)
  	return m, cmd
  }

  func (m *formModel) View() string {
  	v := m.form.View()
  	if v == "" {
  		return "" // form.go:610：quitting 时 View() 确定返回空字符串
  	}
  	return v + "\n" + helpFooter
  }

  // runForm 替代 form.Run()，通过 bubbletea 程序展示带固定帮助栏的表单。
  func runForm(form *huh.Form) error {
  	form.WithShowHelp(false)
  	m := &formModel{form: form}
  	result, err := tea.NewProgram(m).Run()
  	if err != nil {
  		return err
  	}
  	if result.(*formModel).form.State == huh.StateAborted {
  		return huh.ErrUserAborted
  	}
  	return nil
  }
  ```

  需要在 `app.go` 顶部 import 中添加 `tea "github.com/charmbracelet/bubbletea"`。完整 import 块（按 goimports 字母序，`bubbles/key` 在 `bubbletea` 之前）改为：

  ```go
  import (
  	"errors"
  	"fmt"
  	"net/url"
  	"os"
  	"strings"

  	"github.com/charmbracelet/bubbles/key"
  	tea "github.com/charmbracelet/bubbletea"
  	"github.com/charmbracelet/huh"
  	"github.com/charmbracelet/lipgloss"
  	"openclaw_config/internal/config"
  	"openclaw_config/internal/models"
  )
  ```

- [ ] **Step 5：修改 `newForm()` 函数，追加 `.WithShowHelp(false)`**

  找到（约第 337 行）：
  ```go
  func newForm(groups ...*huh.Group) *huh.Form {
  	return huh.NewForm(groups...).WithKeyMap(chineseKeyMap())
  }
  ```

  改为：
  ```go
  func newForm(groups ...*huh.Group) *huh.Form {
  	return huh.NewForm(groups...).WithKeyMap(chineseKeyMap()).WithShowHelp(false)
  }
  ```

- [ ] **Step 6：运行 `go build` 确认编译通过**

  ```bash
  go build ./...
  ```

  期望：无错误输出

- [ ] **Step 7：运行 `go mod tidy`，将 bubbletea 从 indirect 升为 direct**

  ```bash
  go mod tidy
  ```

  验证 `go.mod` 中 `github.com/charmbracelet/bubbletea` 行已不含 `// indirect`

- [ ] **Step 8：提交（`go.sum` 也会被 `go mod tidy` 更新，务必一并提交）**

  ```bash
  git add app.go go.mod go.sum
  git commit -m "feat: 新增 formModel 包装器和 runForm()，统一帮助栏基础设施"
  ```

---

## Task 2：替换 11 处 `form.Run()` 调用

**Files:**
- Modify: `app.go`

按以下顺序逐一替换，每处只需将 `.Run()` 改为传入 `runForm()`：

- [ ] **Step 1：替换 `deleteProvider`（约第 172 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 2：替换 `pickProviderAction`（约第 273 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 3：替换 `pickProviderItemAction`（约第 296 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 4：替换 `editProvider`（约第 437 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 5：替换 `runStep2MainAgent`（约第 505 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 6：替换 `runStep3SubAgent` 第一处（`form1`，约第 542 行）**

  找到：
  ```go
  if err := form1.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form1); err != nil {
  ```

- [ ] **Step 7：替换 `runStep3SubAgent` 第二处（`form2`，约第 580 行）**

  找到：
  ```go
  if err := form2.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form2); err != nil {
  ```

- [ ] **Step 8：替换 `pickNamedAgentAction`（约第 613 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 9：替换 `pickNamedAgentItemAction`（约第 636 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 10：替换 `editNamedAgent`（约第 670 行）**

  找到：
  ```go
  if err := form.Run(); err != nil {
  ```
  改为：
  ```go
  if err := runForm(form); err != nil {
  ```

- [ ] **Step 11：替换 `printSuccess`（约第 772 行）— 内联调用，需拆为两行**

  找到：
  ```go
  newForm(huh.NewGroup( //nolint:errcheck
  	huh.NewNote().
  		Title("提示").
  		Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
  		Next(true).
  		NextLabel("按 Enter 退出"),
  )).Run() //nolint:errcheck
  ```

  改为：
  ```go
  f := newForm(huh.NewGroup(
  	huh.NewNote().
  		Title("提示").
  		Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
  		Next(true).
  		NextLabel("按 Enter 退出"),
  ))
  runForm(f) //nolint:errcheck
  ```

- [ ] **Step 12：`go build` 验证编译**

  ```bash
  go build ./...
  ```

  期望：无错误

- [ ] **Step 13：提交**

  ```bash
  git add app.go
  git commit -m "feat: 替换全部 11 处 form.Run() 为 runForm()，启用统一帮助栏"
  ```

---

## Task 3：清理 `editProvider` Description 中的手动导航提示

**Files:**
- Modify: `app.go`（约第 378 行）

- [ ] **Step 1：删除 `editProvider` 第一个 Input 字段 Description 末尾的导航提示**

  找到（约第 376-379 行）：
  ```go
  huh.NewInput().
  	Title("Provider 标识名").
  	Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）\n· shift+tab 返回上一字段  · Ctrl+C 取消编辑").
  	Placeholder("my-proxy").
  ```

  改为：
  ```go
  huh.NewInput().
  	Title("Provider 标识名").
  	Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）").
  	Placeholder("my-proxy").
  ```

- [ ] **Step 2：`go build` 验证编译**

  ```bash
  go build ./...
  ```

- [ ] **Step 3：运行全部测试**

  ```bash
  go test ./...
  ```

  期望：`ok  openclaw_config [no test files]`（或现有测试全部 PASS）

- [ ] **Step 4：提交**

  ```bash
  git add app.go
  git commit -m "fix: 删除 editProvider Description 中的手动导航提示，由统一帮助栏替代"
  ```

---

## Task 4：验收确认

- [ ] **Step 1：本地运行程序，目视检查所有页面**

  ```bash
  go run .
  ```

  逐页确认：
  - Provider 管理页（Select）→ 底部显示 `ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认`
  - 编辑 Provider 页（Input × 3 + MultiSelect + Input）→ 每个字段底部显示相同 footer
  - 主 Agent 页（Select）→ footer 存在
  - 子 Agent 页（两个 form 页）→ footer 存在
  - 命名 Agent 页 → footer 存在
  - 成功提示页（Note）→ footer 存在
  - 原生 huh 帮助栏（`enter 下一项`）**不再单独显示**

- [ ] **Step 2：确认 `ctrl+c` 取消行为正常**

  在任意页面按 `ctrl+c`，程序应正常退出（不崩溃，不输出堆栈）

- [ ] **Step 3：确认 `editProvider` 第一个字段 Description 中无导航提示文字**

  进入"编辑 Provider"页，Provider 标识名字段的说明文字应只有：
  > 唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）

  不应包含 `shift+tab` 或 `Ctrl+C`。
