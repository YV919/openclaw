# Provider 模型列表提示分层 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `editProvider()` 的 provider 模型列表把“说明文案”“更多模型状态提示”“底部操作帮助”拆到正确层级：Description 只保留说明、列表截断时在选项下方显示 `↓ 更多模型（还有 N 项，继续向下查看）`、模型列表聚焦时 footer 扩展显示多选帮助。

**Architecture:** 保持改动严格收敛在 `app.go` 的 provider 编辑页包装层和对应测试中，不触碰 `internal/config/*`、`internal/models/*`。先用纯 helper 锁定新的文案与高度预算规则，再让 `providerModelListField` 负责列表下方的 overflow hint 与焦点状态，最后通过一个很薄的 `runFormWithFooter(...)` 包装，把 provider 编辑页接成 focus-aware footer，而不把动态 footer 机制扩散到整个项目。

**Tech Stack:** Go 1.23, `github.com/charmbracelet/huh v0.6.0`, Bubble Tea, Lipgloss

---

## 输入文档

- Spec: `docs/superpowers/specs/2026-03-19-provider-model-list-hints-layering-design.md`
- 当前实现：`app.go`
- 当前测试：`app_test.go`

---

## 文件结构与职责

| 文件 | 类型 | 职责 |
|------|------|------|
| `app.go` | 修改 | 调整 provider 模型列表的说明/状态/帮助分层；新增 overflow hint helper、focus-aware footer helper、`runFormWithFooter(...)` 包装，并在 `editProvider()` 中接线 |
| `app_test.go` | 修改 | 锁定新的纯 helper 规则、overflow hint 文案、focus-aware footer 文案，以及 `providerModelListField` 的关键渲染/焦点行为 |

### 本次不改动的边界

- 不改 `internal/config/manager.go`
- 不改 `internal/config/types.go`
- 不改 `internal/models/presets.go`
- 不把动态 footer 扩散到 Step 2 / Step 3 / 命名 Agent 的其他字段
- 不新增分页器、页码、进度条等额外交互

---

## 关键实现约束（开始前先读）

1. 当前 `providerModelListField` 已经是自定义包装器，最佳挂载点如下：
   - `Description`：只放说明文案
   - `providerModelListField.View()`：在 `f.field.View()` 下方拼接 overflow hint
   - `formModel.View()`：追加 footer

2. 当前 `helpFooter` 是静态字符串；本次应把它升级为：
   - 默认 footer 文案不变
   - provider 编辑页可选择传入一个动态 footer 渲染函数
   - 非 provider 页面继续走原 `runForm(form)`，行为不变

3. **最容易错的点是高度预算**：
   - overflow hint 从 `Description` 挪到 `View()` 后，`MultiSelect.Height(...)` 不能再把那 1 行算进字段高度，否则总 block 会比预算多出 1 行。
   - 这意味着 `computeProviderModelListPresentation(...)` 在溢出分支下要返回：
     - `fieldHeight = title + description + visibleRows`
     - 而不是 `fieldHeight = 整个 block 高度`

4. 当前 `providerModelListBaseDescription` 从两行显式文案变成一行显式文案后，`providerModelListBaseDescriptionLines` 的常量预期也要同步改成 `1`，相关测试预期会整体变化。

---

## Task 1: 锁定新的文案与高度预算 helper

**Files:**
- Modify: `app_test.go:155-341`
- Modify: `app.go:337-340`
- Modify: `app.go:474-537`

### 目标

先把“Description 只保留说明文案”“overflow hint 单独生成”“footer 文案切换”“溢出时高度预算要为列表下方提示留出 1 行”都收敛成纯 helper。这样后面的 TUI 接线只是在调用这些 helper，不会把字符串和公式散落到 `Update()`/`View()` 里。

- [ ] **Step 1: 先写失败测试，锁定新的 helper 契约**

在 `app_test.go` 中把现有 provider 模型列表相关测试更新/扩展为下面这组：

```go
func TestComputeProviderModelListPresentationShowsAllOptionsWhenSpaceAllows(t *testing.T) {
	got := computeProviderModelListPresentation(12, 4)
	want := providerModelListPresentation{
		fieldHeight:      6,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestComputeProviderModelListPresentationReservesOverflowHintLine(t *testing.T) {
	got := computeProviderModelListPresentation(8, 10)
	want := providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      5,
		hiddenCount:      5,
		showOverflowHint: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestProviderModelListDescriptionAlwaysUsesBaseText(t *testing.T) {
	got := providerModelListDescription(providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      5,
		hiddenCount:      5,
		showOverflowHint: true,
	})
	want := providerModelListBaseDescription
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestProviderModelListOverflowHintWithOverflowShowsRemainingCount(t *testing.T) {
	got := providerModelListOverflowHint(providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      5,
		hiddenCount:      5,
		showOverflowHint: true,
	})
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if got != want {
		t.Fatalf("overflow hint = %q, want %q", got, want)
	}
}

func TestProviderModelListOverflowHintWithoutOverflowReturnsEmpty(t *testing.T) {
	got := providerModelListOverflowHint(providerModelListPresentation{
		fieldHeight:      6,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	})
	if got != "" {
		t.Fatalf("overflow hint = %q, want empty", got)
	}
}

func TestProviderEditorHelpFooterUsesDefaultTextWhenNotFocused(t *testing.T) {
	got := providerEditorHelpFooter(false)
	want := renderHelpFooter(defaultHelpFooterText)
	if got != want {
		t.Fatalf("footer = %q, want %q", got, want)
	}
}

func TestProviderEditorHelpFooterUsesExpandedTextWhenModelListFocused(t *testing.T) {
	got := providerEditorHelpFooter(true)
	want := renderHelpFooter(providerModelListFocusedHelpFooterText)
	if got != want {
		t.Fatalf("footer = %q, want %q", got, want)
	}
}

func TestProviderModelListAvailableFieldHeightClampsToMinimum(t *testing.T) {
	got := providerModelListAvailableFieldHeight(6, "a", "b", "c", "d")
	want := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	if got != want {
		t.Fatalf("available height = %d, want %d", got, want)
	}
}
```

说明：

- 这里要**改掉**当前 `TestProviderModelListDescriptionWithOverflowAppendsHiddenCount` 的预期，因为新设计里 overflow hint 不再挂在 `Description`。
- `fieldHeight: 6/7` 这组预期是本次实现的关键：
  - 基础说明文案 1 行
  - overflow hint 1 行单独放到 block 外层
  - 所以溢出时 `fieldHeight` 比总 block 高度少 1

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription|ProviderModelListOverflowHint|ProviderEditorHelpFooter|ProviderModelListAvailableFieldHeight)'
```

Expected:

- 至少有若干测试失败，原因包括但不限于：
  - `undefined: providerModelListOverflowHint`
  - `undefined: providerEditorHelpFooter`
  - `undefined: renderHelpFooter`
  - `description` 仍然带着旧的 overflow 文案
  - `fieldHeight` / `visibleRows` 仍然沿用旧预算

- [ ] **Step 3: 在 `app.go` 写最小实现，让 helper 契约成立**

在 `app.go` 中做以下最小修改：

1. 把 footer 文案拆成文本 + 渲染 helper：

```go
const defaultHelpFooterText = "  ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认"

const providerModelListFocusedHelpFooterText = "  ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认"

func renderHelpFooter(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(text)
}

var helpFooter = renderHelpFooter(defaultHelpFooterText)

func providerEditorHelpFooter(modelListFocused bool) string {
	if modelListFocused {
		return renderHelpFooter(providerModelListFocusedHelpFooterText)
	}
	return helpFooter
}
```

2. 把基础描述改成只保留说明文案，并同步更新行数常量：

```go
const providerModelListBaseDescription = "选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个"

const (
	providerModelListTitleLines           = 1
	providerModelListBaseDescriptionLines = 1
	providerModelListOverflowLines        = 1
)
```

3. 更新 `computeProviderModelListPresentation(...)`，让溢出分支为列表下方 hint 预留 1 行，但返回给 `MultiSelect.Height(...)` 的 `fieldHeight` 不包含这 1 行：

```go
func computeProviderModelListPresentation(availableBlockHeight int, optionCount int) providerModelListPresentation {
	minBlockHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	if optionCount <= 0 {
		return providerModelListPresentation{
			fieldHeight:      providerModelListTitleLines + providerModelListBaseDescriptionLines + 1,
			visibleRows:      1,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	fullBlockHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + optionCount
	if availableBlockHeight >= fullBlockHeight {
		return providerModelListPresentation{
			fieldHeight:      fullBlockHeight,
			visibleRows:      optionCount,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	availableBlockHeight = max(minBlockHeight, availableBlockHeight)
	visibleRows := max(1, availableBlockHeight-providerModelListTitleLines-providerModelListBaseDescriptionLines-providerModelListOverflowLines)
	return providerModelListPresentation{
		fieldHeight:      providerModelListTitleLines + providerModelListBaseDescriptionLines + visibleRows,
		visibleRows:      visibleRows,
		hiddenCount:      max(0, optionCount-visibleRows),
		showOverflowHint: optionCount > visibleRows,
	}
}
```

4. 让 `providerModelListDescription(...)` 只返回基础说明文案，并新增 overflow hint helper：

```go
func providerModelListDescription(providerModelListPresentation) string {
	return providerModelListBaseDescription
}

func providerModelListOverflowHint(p providerModelListPresentation) string {
	if !p.showOverflowHint || p.hiddenCount <= 0 {
		return ""
	}
	return fmt.Sprintf("↓ 更多模型（还有 %d 项，继续向下查看）", p.hiddenCount)
}
```

- [ ] **Step 4: 重新运行定向测试，确认 helper 全绿**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription|ProviderModelListOverflowHint|ProviderEditorHelpFooter|ProviderModelListAvailableFieldHeight)'
```

Expected: PASS

- [ ] **Step 5: 提交这一小步**

```bash
git add app.go app_test.go
git commit -m "refactor: split provider model list hint helpers"
```

---

## Task 2: 把 overflow hint 和 focus-aware footer 接到 provider 编辑页

**Files:**
- Modify: `app_test.go:1-40`
- Modify: `app_test.go:155-341`
- Modify: `app.go:342-386`
- Modify: `app.go:540-629`
- Modify: `app.go:844-881`

### 目标

让 provider 编辑页真正表现出 spec 中的行为：

- 列表截断时，hint 出现在**选项列表下面**
- 进入模型列表字段时，底部帮助栏切成多选版本
- 离开模型列表字段后，底部帮助栏恢复默认版本
- 其他页面继续使用原 `runForm(form)`，不受影响

- [ ] **Step 1: 先写失败测试，锁定 `providerModelListField` 的关键行为**

在 `app_test.go` 增加 `strings` import，并追加下面 2 个测试：

```go
func TestProviderModelListFieldViewAppendsOverflowHintBelowOptions(t *testing.T) {
	var selected []string
	field := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
			Options(
				huh.NewOption("a", "a"),
				huh.NewOption("b", "b"),
				huh.NewOption("c", "c"),
				huh.NewOption("d", "d"),
				huh.NewOption("e", "e"),
				huh.NewOption("f", "f"),
				huh.NewOption("g", "g"),
				huh.NewOption("h", "h"),
				huh.NewOption("i", "i"),
				huh.NewOption("j", "j"),
			).
			Value(&selected).
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 8 },
		func() int { return 10 },
	)
	field.lastWindowHeight = 8

	got := field.View()
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if !strings.Contains(got, want) {
		t.Fatalf("view = %q, want substring %q", got, want)
	}
}

func TestProviderModelListFieldFocusStateTracksFocusAndBlur(t *testing.T) {
	var selected []string
	field := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
			Options(huh.NewOption("a", "a")).
			Value(&selected).
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 6 },
		func() int { return 1 },
	)

	if field.IsFocused() {
		t.Fatalf("expected unfocused field")
	}
	field.Focus()
	if !field.IsFocused() {
		t.Fatalf("expected focused field")
	}
	field.Blur()
	if field.IsFocused() {
		t.Fatalf("expected blurred field")
	}
}
```

- [ ] **Step 2: 运行定向测试，确认这些行为当前还没接上**

Run:

```bash
go test ./... -run 'TestProviderModelListField(ViewAppendsOverflowHintBelowOptions|FocusStateTracksFocusAndBlur)'
```

Expected:

- 编译失败或测试失败，至少出现以下一种：
  - `field.IsFocused undefined`
  - `View()` 输出里没有 `↓ 更多模型（还有 5 项，继续向下查看）`
  - `Focus()` / `Blur()` 没有维护包装层焦点状态

- [ ] **Step 3: 写最小实现，把动态渲染真正接起来**

在 `app.go` 做以下接线：

1. 给 `formModel` 增加可选的 footer 渲染函数：

```go
type formModel struct {
	form           *huh.Form
	helpFooterView func() string
}
```

2. 让 `formModel.View()` 优先使用动态 footer，否则回退到默认 `helpFooter`：

```go
func (m *formModel) View() string {
	v := m.form.View()
	if v == "" {
		return ""
	}
	footer := helpFooter
	if m.helpFooterView != nil {
		footer = m.helpFooterView()
	}
	return v + "\n" + footer
}
```

3. 新增 `runFormWithFooter(...)`，保持 `runForm(form)` 作为默认入口不变：

```go
func runFormWithFooter(form *huh.Form, helpFooterView func() string) error {
	form = prepareFormForRun(form)
	m := &formModel{form: form, helpFooterView: helpFooterView}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if result.(*formModel).form.State == huh.StateAborted {
		return huh.ErrUserAborted
	}
	return nil
}

func runForm(form *huh.Form) error {
	return runFormWithFooter(form, nil)
}
```

4. 给 `providerModelListField` 增加包装层焦点状态，并在 `View()` 中拼接 overflow hint：

```go
type providerModelListField struct {
	field            *huh.MultiSelect[string]
	measureAvailable func(int) int
	optionCount      func() int
	lastWindowHeight int
	presentation     providerModelListPresentation
	focused          bool
}

func (f *providerModelListField) View() string {
	if f.lastWindowHeight > 0 {
		f.syncPresentation(f.lastWindowHeight)
	}
	view := f.field.View()
	if hint := providerModelListOverflowHint(f.presentation); hint != "" {
		return view + "\n" + hint
	}
	return view
}

func (f *providerModelListField) Focus() tea.Cmd {
	f.focused = true
	return f.field.Focus()
}

func (f *providerModelListField) Blur() tea.Cmd {
	f.focused = false
	return f.field.Blur()
}

func (f *providerModelListField) IsFocused() bool {
	return f.focused
}
```

5. `syncPresentation(...)` 继续只设置：
   - `Description(providerModelListDescription(...))`
   - `Height(presentation.fieldHeight)`

   不要把 overflow hint 再塞回 `Description()`。

6. 在 `editProvider()` 中改为使用动态 footer：

```go
if err := runFormWithFooter(form, func() string {
	return providerEditorHelpFooter(modelListField.IsFocused())
}); err != nil {
	...
}
```

说明：

- 这里不要去改全局 `helpFooter` 变量，也不要让 `providerModelListField` 直接修改全局状态。
- 用闭包捕获 `modelListField` 指针，是这次最薄、回归面最小的接线方式。

- [ ] **Step 4: 重新运行定向测试，确认运行时接线通过**

Run:

```bash
go test ./... -run 'TestProviderModelListField|TestProviderEditorHelpFooter|TestProviderModelListOverflowHint'
```

Expected: PASS

- [ ] **Step 5: 运行全量测试，确认没有回归**

Run:

```bash
go test ./...
```

Expected: PASS

- [ ] **Step 6: 手动做一次 TUI 冒烟验证**

Run:

```bash
go run .
```

手动检查 provider 编辑页：

- 焦点不在模型列表时，底部为：`ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认`
- 焦点进入模型列表后，底部扩展为：`ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认`
- 模型列表说明文案只剩一句说明，不再包含多选操作提示
- 当窗口高度较小且选项未显示完时，“↓ 更多模型（还有 N 项，继续向下查看）”出现在**选项列表下面**，而不是标题下面或 Description 里

- [ ] **Step 7: 提交这一小步**

```bash
git add app.go app_test.go
git commit -m "feat: layer provider model list hints"
```

---

## Task 3: 最终回归与收尾检查

**Files:**
- Modify if needed: `app.go`
- Modify if needed: `app_test.go`

### 目标

在声称完成之前，再做一次最小但完整的回归核对，确保这次改动只改变用户要求的交互层级，不引入额外行为变化。

- [ ] **Step 1: 重新运行全量测试并确认输出干净**

Run:

```bash
go test ./...
```

Expected: PASS，且无新增 warning / panic / hanging tests

- [ ] **Step 2: 检查 diff，确认没有超范围修改**

Run:

```bash
git diff -- app.go app_test.go
```

Expected:

- 只出现与以下目标直接相关的变更：
  - provider 模型列表基础说明文案收敛
  - overflow hint 从 `Description` 拆到 `View()`
  - provider 页面动态 footer 接线
  - 对应测试更新

- [ ] **Step 3: 如冒烟检查后有小修正，先补对应测试再改代码**

如果手动验证中发现以下任何问题：

- footer 没有在离开模型列表后恢复
- overflow hint 出现在错误位置
- 列表高度被多挤掉 1 行

先补一个失败测试，再修代码；不要直接热修。

- [ ] **Step 4: 做最终提交**

```bash
git add app.go app_test.go
git commit -m "fix: polish provider model list help layering"
```

如果 Task 2 后没有新增修正，这一步可以跳过，保留 Task 2 的提交作为最终提交。

---

## 验收清单

- [ ] `模型列表` 的 `Description` 只显示说明文案
- [ ] 多选操作提示不再出现在 `Description` 中
- [ ] 列表被截断时，`↓ 更多模型（还有 N 项，继续向下查看）` 出现在选项列表下面
- [ ] 列表完整显示时，不出现 overflow hint
- [ ] 焦点进入模型列表字段时，底部帮助栏切换到多选版本
- [ ] 焦点离开模型列表字段后，底部帮助栏恢复默认版本
- [ ] 自定义模型动态注册与默认选中行为保持不变
- [ ] `go test ./...` 通过
- [ ] 手动 `go run .` 冒烟验证通过

---

## 失败时的回退思路

如果 Task 2 接线时发现 `runFormWithFooter(...)` 让通用表单包装变复杂，优先保持边界清晰：

1. 保留 `runForm(form)` 作为默认入口不动
2. 只为 `editProvider()` 引入一个专用入口，例如 `runProviderEditorForm(...)`
3. 不要因此把“动态 footer”做成全局字段类型推断系统

YAGNI：本次只解决 provider 编辑页这一处。
