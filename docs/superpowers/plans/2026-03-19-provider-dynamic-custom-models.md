# Provider 动态自定义模型 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `editProvider()` 中的自定义模型在输入完成后立即注册进动态多选列表、默认选中，最终保存只取当前勾选结果，并把该列表局部改成 `[ ]` / `[✓]` 样式。

**Architecture:** 将 provider 模型编辑状态拆成 `selectedModels`（唯一保存来源）、`customModelRegistry`（当前会话中的自定义模型注册池）和 `customModelInput`（一次性新增入口）。`MultiSelect` 改为 `OptionsFunc(...)` 动态渲染，底部输入框改为一个很薄的字段包装器：在识别到 `enter/tab` 完成输入时调用注册逻辑、清空可见值、并返回上一字段，从而避免最后一个字段直接提交整个表单。样式通过字段级 `WithTheme(...)` 覆盖，仅影响 provider 模型列表这一处。

**Tech Stack:** Go 1.23, `github.com/charmbracelet/huh v0.6.0`, Bubble Tea, Lipgloss

---

## 实现前必读

### 1. 当前代码入口

本次实现只改两个文件：

- `app.go`
  - `splitProviderModelsForEdit()` / `parseCustomModelInput()` / `mergeProviderModels()` / `appendUniqueStrings()` 位于 `app.go:402-458`
  - `editProvider()` 位于 `app.go:460-562`
  - 统一表单入口 `newForm()` / `runForm()` 位于 `app.go:368-392`
- `app_test.go`
  - 现有 provider 模型相关测试位于 `app_test.go:71-106`

### 2. `huh v0.6.0` 约束（本计划基于已核实的源码）

- `MultiSelect.OptionsFunc(...)` 支持动态 options，适合“预设模型 + 自定义注册池”这一列表。
- `MultiSelect.WithTheme(...)` 是字段级覆盖；可以只改 provider 模型列表，不影响别的页面。
- `theme.go` 里 `SelectedPrefix` / `UnselectedPrefix` 决定多选项前缀，局部主题覆盖就能实现 `[✓] ` / `[ ] `。
- `Input.Update()` 在每次按键后都会 `accessor.Set(i.textinput.Value())`；所以**不能**把 `Accessor.Set(...)` 当成“仅在失焦时触发”的钩子。
- `Group.nextField()` 在最后一个字段上会触发 `nextGroup`，也就是默认提交；因此必须在自定义输入字段内部拦截已处理的 `enter/tab`，把导航从 `NextField` 改成 `PrevField`。
- `Input.Accessor(...)` 会立刻把 `accessor.Get()` 同步进可见输入框；这可以用来在注册成功后同时清空“绑定值”和“可见文本”。

### 3. 代码边界

不改动：

- `internal/config/manager.go`
- `internal/config/types.go`
- `internal/models/presets.go`
- Step 2/3/4 的模型选择逻辑
- `formModel` / `runForm()` 的通用帮助栏机制

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `app.go` | 修改 | 重构 provider 模型 helper、追加 provider 专用 theme helper、增加自定义输入包装字段、改写 `editProvider()` 的状态与保存流 |
| `app_test.go` | 修改 | 替换旧的 provider helper 测试，新增动态 options / 注册逻辑 / 局部样式 / 输入包装器行为测试 |

---

## Task 1: 重构 provider 模型状态 helper，并把列表切到动态 options

**Files:**
- Modify: `app.go:402-562`
- Modify: `app_test.go:71-106`

### 目标

先把 `editProvider()` 的数据模型从“预设选择 + 自定义输入直存”改成“动态列表 + 选中结果单一来源”，但此时先不实现最后一个输入框的拦截回跳；只把底层 helper、局部主题、动态 `MultiSelect` 和最终保存流打通。

### 设计落点

1. `splitProviderModelsForEdit()` 改成返回：
   - `selectedModels []string`
   - `customModelRegistry []string`
2. 新增 `buildProviderModelOptions(selectedModels, customModelRegistry []string) []huh.Option[string]`
   - 预设模型始终在前
   - 自定义模型始终在后
   - 与预设重名的自定义项不得重复追加
3. 新增 `finalProviderModels(selectedModels []string) []string`
   - 只从 `selectedModels` 生成最终保存值
   - 去空串、去重、保序
4. 新增 `providerModelListTheme() *huh.Theme`
   - 基于 `huh.ThemeCharm()` 克隆后，仅覆盖 provider 模型列表使用到的前缀
   - `Focused.SelectedPrefix = "[✓] "`
   - `Focused.UnselectedPrefix = "[ ] "`
   - `Blurred.SelectedPrefix = "[✓] "`
   - `Blurred.UnselectedPrefix = "[ ] "`
5. `editProvider()` 中：
   - `customModelInput` 初始值改为空字符串，不再用历史自定义模型回填输入框
   - `MultiSelect` 改为 `OptionsFunc(...)`
   - “至少选择一个模型”的验证从输入框迁到 `MultiSelect.Validate(...)`
   - 最终保存改用 `finalProviderModels(selectedModels)`，不再调用 `mergeProviderModels(...)`

- [ ] **Step 1: 在 `app_test.go` 写失败测试，锁定新的 helper 契约**

保留现有 `TestParseCustomModelInputSupportsMultipleEntries`（它仍负责验证逗号、中文逗号、分号、换行与去重行为），把其余旧的 provider helper 测试改写成下面 5 条（可直接接在 `TestPrepareFormForRunSetsQuitCommands` 之后）：

```go
func TestProviderModelSplitSeparatesPresetAndCustomRegistry(t *testing.T) {
	selected, customRegistry := splitProviderModelsForEdit([]string{
		"claude-opus-4-6",
		"custom-alpha",
		"gpt-5.2",
		"custom-beta",
	})

	if !reflect.DeepEqual(selected, []string{"claude-opus-4-6", "gpt-5.2"}) {
		t.Fatalf("selected = %v, want %v", selected, []string{"claude-opus-4-6", "gpt-5.2"})
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha", "custom-beta"}) {
		t.Fatalf("customRegistry = %v, want %v", customRegistry, []string{"custom-alpha", "custom-beta"})
	}
}

func TestProviderModelOptionsOrderPresetBeforeCustom(t *testing.T) {
	opts := buildProviderModelOptions(
		[]string{"claude-opus-4-6", "custom-beta"},
		[]string{"custom-alpha", "custom-beta"},
	)

	values := optionValues(opts)
	if len(values) < 2 {
		t.Fatalf("option count = %d, want at least 2", len(values))
	}
	gotTail := values[len(values)-2:]
	wantTail := []string{"custom-alpha", "custom-beta"}
	if !reflect.DeepEqual(gotTail, wantTail) {
		t.Fatalf("tail values = %v, want %v", gotTail, wantTail)
	}
}

func TestProviderModelOptionsSkipPresetNamedCustoms(t *testing.T) {
	opts := buildProviderModelOptions(
		[]string{"gpt-5.2"},
		[]string{"gpt-5.2", "custom-alpha"},
	)

	values := optionValues(opts)
	count := 0
	for _, value := range values {
		if value == "gpt-5.2" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("gpt-5.2 count = %d, want 1", count)
	}
}

func TestProviderModelFinalSelectionIsOnlySaveSource(t *testing.T) {
	got := finalProviderModels([]string{"claude-opus-4-6", "custom-alpha", "custom-alpha", ""})
	want := []string{"claude-opus-4-6", "custom-alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("finalProviderModels() = %v, want %v", got, want)
	}
}

func TestProviderModelListThemeUsesAsciiCheckboxPrefixes(t *testing.T) {
	theme := providerModelListTheme()
	if theme.Focused.SelectedPrefix.String() != "[✓] " {
		t.Fatalf("focused selected prefix = %q, want %q", theme.Focused.SelectedPrefix.String(), "[✓] ")
	}
	if theme.Focused.UnselectedPrefix.String() != "[ ] " {
		t.Fatalf("focused unselected prefix = %q, want %q", theme.Focused.UnselectedPrefix.String(), "[ ] ")
	}
	if theme.Blurred.SelectedPrefix.String() != "[✓] " {
		t.Fatalf("blurred selected prefix = %q, want %q", theme.Blurred.SelectedPrefix.String(), "[✓] ")
	}
	if theme.Blurred.UnselectedPrefix.String() != "[ ] " {
		t.Fatalf("blurred unselected prefix = %q, want %q", theme.Blurred.UnselectedPrefix.String(), "[ ] ")
	}
}

func optionValues(opts []huh.Option[string]) []string {
	values := make([]string, 0, len(opts))
	for _, opt := range opts {
		values = append(values, opt.Value)
	}
	return values
}
```

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'TestProviderModel(Split|Options|FinalSelection|ListTheme)'
```

Expected:

- 编译失败或测试失败，至少包含下面这些信号之一：
  - `cannot use customInput (type string) as type []string`
  - `undefined: buildProviderModelOptions`
  - `undefined: finalProviderModels`
  - `undefined: providerModelListTheme`

- [ ] **Step 3: 在 `app.go` 实现新的 helper**

把 `app.go:402-458` 这一段重组为下面的职责：

```go
func splitProviderModelsForEdit(providerModels []string) ([]string, []string) {
	presetSet := presetModelSet()
	selectedModels := make([]string, 0, len(providerModels))
	customRegistry := make([]string, 0, len(providerModels))

	for _, modelID := range providerModels {
		trimmed := strings.TrimSpace(modelID)
		if trimmed == "" {
			continue
		}
		if presetSet[trimmed] {
			selectedModels = appendUniqueStrings(selectedModels, trimmed)
			continue
		}
		customRegistry = appendUniqueStrings(customRegistry, trimmed)
		selectedModels = appendUniqueStrings(selectedModels, trimmed)
	}

	return selectedModels, customRegistry
}

func buildProviderModelOptions(selectedModels []string, customModelRegistry []string) []huh.Option[string] {
	selectedSet := make(map[string]bool, len(selectedModels))
	for _, modelID := range selectedModels {
		selectedSet[modelID] = true
	}

	opts := make([]huh.Option[string], 0, len(models.PresetModels)+len(customModelRegistry))
	presetSet := presetModelSet()
	for _, modelID := range models.PresetModels {
		opt := huh.NewOption(modelID, modelID)
		if selectedSet[modelID] {
			opt = opt.Selected(true)
		}
		opts = append(opts, opt)
	}
	for _, modelID := range customModelRegistry {
		if presetSet[modelID] {
			continue
		}
		opt := huh.NewOption(modelID, modelID)
		if selectedSet[modelID] {
			opt = opt.Selected(true)
		}
		opts = append(opts, opt)
	}
	return opts
}

func finalProviderModels(selectedModels []string) []string {
	finalModels := make([]string, 0, len(selectedModels))
	for _, modelID := range selectedModels {
		if trimmed := strings.TrimSpace(modelID); trimmed != "" {
			finalModels = appendUniqueStrings(finalModels, trimmed)
		}
	}
	return finalModels
}

func providerModelListTheme() *huh.Theme {
	theme := huh.ThemeCharm()
	theme.Focused.SelectedPrefix = theme.Focused.SelectedPrefix.SetString("[✓] ")
	theme.Focused.UnselectedPrefix = theme.Focused.UnselectedPrefix.SetString("[ ] ")
	theme.Blurred.SelectedPrefix = theme.Blurred.SelectedPrefix.SetString("[✓] ")
	theme.Blurred.UnselectedPrefix = theme.Blurred.UnselectedPrefix.SetString("[ ] ")
	return theme
}
```

注意：

- `splitProviderModelsForEdit()` 现在返回自定义注册池，而不是输入框字符串。
- 历史自定义模型应同时进入 `selectedModels`，因为编辑已有 provider 时这些模型本来就是已保存模型。
- 删除旧的 `mergeProviderModels(...)` 实现与对应旧测试；后续 `editProvider()` 不再需要它。

- [ ] **Step 4: 改写 `editProvider()`，让模型列表变成动态字段**

把 `editProvider()` 中模型相关的局部状态改成：

```go
selectedModels, customModelRegistry := splitProviderModelsForEdit(p.Models)
customModelInput := ""
modelOptionsVersion := 0
```

然后把当前静态 `MultiSelect` 和底部输入框改成下面这种结构：

```go
modelListField := huh.NewMultiSelect[string]().
	Title("模型列表").
	Description("空格/x 切换选中，↑↓ 移动，enter 确认\n选择此 provider 支持的模型；如需新增自定义模型，请在下方输入框输入后按 enter/tab 注册").
	OptionsFunc(func() []huh.Option[string] {
		return buildProviderModelOptions(selectedModels, customModelRegistry)
	}, &modelOptionsVersion).
	Validate(func(values []string) error {
		if len(finalProviderModels(values)) == 0 {
			return fmt.Errorf("请至少选择一个模型")
		}
		return nil
	}).
	Value(&selectedModels)
modelListField.WithTheme(providerModelListTheme())

customModelField := huh.NewInput().
	Title("自定义模型名称（可选，多个可用逗号/换行分隔）").
	Placeholder("my-custom-model-a, my-custom-model-b").
	Value(&customModelInput)
```

此时先保持底部输入框还是普通 `Input`，下一任务再把它换成包装字段。

同时把函数末尾：

```go
finalModels := mergeProviderModels(selectedModels, customModel)
```

改成：

```go
finalModels := finalProviderModels(selectedModels)
```

并删除“输入框文本也参与最终保存”的逻辑。

- [ ] **Step 5: 运行定向测试，确认 helper 与局部样式都已通过**

Run:

```bash
go test ./... -run 'TestProviderModel(Split|Options|FinalSelection|ListTheme)'
```

Expected:

- 5 条新测试全部 PASS。

- [ ] **Step 6: 运行全量测试**

Run:

```bash
go test ./...
```

Expected:

- 全部测试通过。

- [ ] **Step 7: 提交**

```bash
git add app.go app_test.go
git commit -m "refactor: move provider model editing to dynamic registry-backed state"
```

---

## Task 2: 实现自定义模型注册 helper，锁定“新增 / 重复 / 预设重名”语义

**Files:**
- Modify: `app.go:402-458`
- Modify: `app_test.go` provider 模型测试区

### 目标

把“输入框文本如何变成注册行为”从 UI 控件里剥出来，先沉到一个纯 helper 里。这样最复杂的边界条件可以先用单元测试锁死，最后 UI 包装层只负责在合适的按键时调用它。

### 设计落点

新增一个很小的结果结构体：

```go
type providerCustomRegistration struct {
	handled bool
	added   bool
}
```

以及：

```go
func registerProviderCustomModels(
	input string,
	selectedModels *[]string,
	customModelRegistry *[]string,
) providerCustomRegistration
```

行为规则：

- 输入为空 / 解析后没有有效模型名：`handled=false, added=false`
- 输入全是已存在模型名：`handled=true, added=false`
- 输入里有至少一个全新自定义模型：`handled=true, added=true`
- 与预设同名时：
  - 不追加到 `customModelRegistry`
  - 但要确保该预设模型进入 `selectedModels`
- 全新自定义模型：
  - 追加到 `customModelRegistry`
  - 追加到 `selectedModels`
- 所有写入都要保序去重

- [ ] **Step 1: 在 `app_test.go` 写失败测试，锁定注册语义**

在 Task 1 的测试之后继续追加下面 4 条：

```go
func TestProviderModelRegisterAddsNewCustomModelsAndSelectsThem(t *testing.T) {
	selectedModels := []string{"claude-opus-4-6"}
	customRegistry := []string{"custom-alpha"}

	result := registerProviderCustomModels(
		"custom-beta, custom-gamma",
		&selectedModels,
		&customRegistry,
	)

	if !result.handled || !result.added {
		t.Fatalf("result = %+v, want handled=true added=true", result)
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha", "custom-beta", "custom-gamma"}) {
		t.Fatalf("customRegistry = %v", customRegistry)
	}
	if !reflect.DeepEqual(selectedModels, []string{"claude-opus-4-6", "custom-beta", "custom-gamma"}) {
		t.Fatalf("selectedModels = %v", selectedModels)
	}
}

func TestProviderModelRegisterDeduplicatesInputAndKeepsExistingCustoms(t *testing.T) {
	selectedModels := []string{"custom-alpha"}
	customRegistry := []string{"custom-alpha"}

	result := registerProviderCustomModels(
		"custom-alpha, custom-alpha",
		&selectedModels,
		&customRegistry,
	)

	if !result.handled || result.added {
		t.Fatalf("result = %+v, want handled=true added=false", result)
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha"}) {
		t.Fatalf("customRegistry = %v", customRegistry)
	}
	if !reflect.DeepEqual(selectedModels, []string{"custom-alpha"}) {
		t.Fatalf("selectedModels = %v", selectedModels)
	}
}

func TestProviderModelRegisterSelectsPresetWithoutAddingDuplicateCustom(t *testing.T) {
	selectedModels := []string{"claude-opus-4-6"}
	customRegistry := []string{"custom-alpha"}

	result := registerProviderCustomModels(
		"gpt-5.2",
		&selectedModels,
		&customRegistry,
	)

	if !result.handled || result.added {
		t.Fatalf("result = %+v, want handled=true added=false", result)
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha"}) {
		t.Fatalf("customRegistry = %v", customRegistry)
	}
	if !reflect.DeepEqual(selectedModels, []string{"claude-opus-4-6", "gpt-5.2"}) {
		t.Fatalf("selectedModels = %v", selectedModels)
	}
}

func TestProviderModelRegisterIgnoresEmptyInput(t *testing.T) {
	selectedModels := []string{"claude-opus-4-6"}
	customRegistry := []string{"custom-alpha"}

	result := registerProviderCustomModels("  ,  \n", &selectedModels, &customRegistry)
	if result.handled || result.added {
		t.Fatalf("result = %+v, want handled=false added=false", result)
	}
}
```

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'TestProviderModelRegister'
```

Expected:

- 编译失败：`undefined: registerProviderCustomModels`
- 或测试失败：返回值 / 列表变更与预期不一致

- [ ] **Step 3: 在 `app.go` 实现注册 helper**

把下面这段直接放在 `parseCustomModelInput()` 后、`finalProviderModels()` 前后都可以，但要和 provider 模型 helper 放在同一区域：

```go
type providerCustomRegistration struct {
	handled bool
	added   bool
}

func registerProviderCustomModels(
	input string,
	selectedModels *[]string,
	customModelRegistry *[]string,
) providerCustomRegistration {
	parsed := parseCustomModelInput(input)
	if len(parsed) == 0 {
		return providerCustomRegistration{}
	}

	presetSet := presetModelSet()
	result := providerCustomRegistration{handled: true}

	for _, modelID := range parsed {
		if presetSet[modelID] {
			*selectedModels = appendUniqueStrings(*selectedModels, modelID)
			continue
		}

		before := len(*customModelRegistry)
		*customModelRegistry = appendUniqueStrings(*customModelRegistry, modelID)
		if len(*customModelRegistry) != before {
			result.added = true
		}
		*selectedModels = appendUniqueStrings(*selectedModels, modelID)
	}

	return result
}
```

注意：

- `handled` 只表示“这次输入被识别并消费了”，不等于“一定新增了列表项”。
- `added` 只在真的新增了自定义列表项时才为 `true`。
- 这里不要操作输入框文本；清空输入框是下一任务的 UI 责任。

- [ ] **Step 4: 运行定向测试，确认注册语义全部通过**

Run:

```bash
go test ./... -run 'TestProviderModelRegister'
```

Expected:

- 4 条注册测试全部 PASS。

- [ ] **Step 5: 再跑一次全量测试**

Run:

```bash
go test ./...
```

Expected:

- 全部测试通过。

- [ ] **Step 6: 提交**

```bash
git add app.go app_test.go
git commit -m "feat: add provider custom model registration helper"
```

---

## Task 3: 用输入包装字段拦截最后一个输入框，并完成“注册后回跳列表”

**Files:**
- Modify: `app.go:460-562`（`editProvider()`）
- Modify: `app.go` provider helper 区附近（新增包装字段类型）
- Modify: `app_test.go`

### 目标

让最后一个输入框在按 `enter/tab` 且输入被识别时：

- 先调用注册 helper
- 若有全新模型，立即进入动态 `MultiSelect`
- 输入框可见值被清空
- 焦点返回上一字段（模型列表）
- 不直接提交整个 provider 表单

### 设计落点

新增一个只服务 `editProvider()` 的薄包装字段，示意结构如下：

```go
type providerCustomModelInput struct {
	input    *huh.Input
	accessor *huh.PointerAccessor[string]
	keymap   *huh.KeyMap
	onSubmit func(raw string) providerCustomRegistration
}
```

关键实现点：

1. 该类型自己实现 `huh.Field`，其余方法都直接委托给内部 `*huh.Input`
2. `WithKeyMap(...)` 里把 `*huh.KeyMap` 存到 `field.keymap`，这样 `Update(...)` 里就能和项目当前 keymap 保持一致
3. `Update(msg tea.Msg)` 中：
   - 若收到 `tea.KeyMsg` 且匹配 `keymap.Input.Next` / `keymap.Input.Submit`
   - 先读取 `raw := accessor.Get()`
   - 调用 `result := onSubmit(raw)`
   - 若 `result.handled`：
     - `accessor.Set("")`
     - `input.Accessor(accessor)`，强制把空字符串同步回可见文本框
     - 返回 `f, huh.PrevField`
   - 否则走 `input.Update(msg)` 原始逻辑
4. `onSubmit(...)` 闭包里：
   - 调用 `registerProviderCustomModels(...)`
   - 若 `result.added`，则 `modelOptionsVersion++`
   - 返回 `result`
5. 不要改 `formModel.Update()` 或 `runForm()`；这次行为只限定在 provider 自定义模型输入字段里

- [ ] **Step 1: 在 `app_test.go` 写失败测试，锁定包装字段行为**

新增下面两条测试：

```go
func TestProviderCustomInputHandledEnterClearsValueAndMutatesSelection(t *testing.T) {
	selectedModels := []string{"claude-opus-4-6"}
	customRegistry := []string{"custom-alpha"}
	customInput := "custom-beta"
	optionsVersion := 0

	field := newProviderCustomModelInput(
		&customInput,
		func(raw string) providerCustomRegistration {
			result := registerProviderCustomModels(raw, &selectedModels, &customRegistry)
			if result.added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(chineseKeyMap())

	_, cmd := field.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected navigation command")
	}
	if customInput != "" {
		t.Fatalf("customInput = %q, want empty", customInput)
	}
	if optionsVersion != 1 {
		t.Fatalf("optionsVersion = %d, want 1", optionsVersion)
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha", "custom-beta"}) {
		t.Fatalf("customRegistry = %v", customRegistry)
	}
	if !reflect.DeepEqual(selectedModels, []string{"claude-opus-4-6", "custom-beta"}) {
		t.Fatalf("selectedModels = %v", selectedModels)
	}
}

func TestProviderCustomInputHandledDuplicateStillClearsValue(t *testing.T) {
	selectedModels := []string{"custom-alpha"}
	customRegistry := []string{"custom-alpha"}
	customInput := "custom-alpha"
	optionsVersion := 0

	field := newProviderCustomModelInput(
		&customInput,
		func(raw string) providerCustomRegistration {
			result := registerProviderCustomModels(raw, &selectedModels, &customRegistry)
			if result.added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(chineseKeyMap())

	_, cmd := field.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected navigation command")
	}
	if customInput != "" {
		t.Fatalf("customInput = %q, want empty", customInput)
	}
	if optionsVersion != 0 {
		t.Fatalf("optionsVersion = %d, want 0", optionsVersion)
	}
}
```

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'TestProviderCustomInput'
```

Expected:

- 编译失败：`undefined: newProviderCustomModelInput`
- 或测试失败：输入值未清空 / 未返回命令 / 注册结果未写入

- [ ] **Step 3: 在 `app.go` 实现包装字段**

在 provider helper 区后面、`editProvider()` 前面新增这组代码：

```go
type providerCustomModelInput struct {
	input    *huh.Input
	accessor *huh.PointerAccessor[string]
	keymap   *huh.KeyMap
	onSubmit func(string) providerCustomRegistration
}

func newProviderCustomModelInput(
	value *string,
	onSubmit func(string) providerCustomRegistration,
) *providerCustomModelInput {
	accessor := huh.NewPointerAccessor(value)
	return &providerCustomModelInput{
		input: huh.NewInput().
			Title("自定义模型名称（可选，多个可用逗号/换行分隔）").
			Placeholder("my-custom-model-a, my-custom-model-b").
			Accessor(accessor),
		accessor: accessor,
		onSubmit: onSubmit,
	}
}

func (f *providerCustomModelInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && f.keymap != nil {
		if key.Matches(keyMsg, f.keymap.Input.Next, f.keymap.Input.Submit) {
			result := f.onSubmit(f.accessor.Get())
			if result.handled {
				f.accessor.Set("")
				f.input.Accessor(f.accessor)
				return f, huh.PrevField
			}
		}
	}
	m, cmd := f.input.Update(msg)
	f.input = m.(*huh.Input)
	return f, cmd
}
```

再把 `Init/Blur/Focus/View/Error/Run/Skip/Zoom/KeyBinds/WithTheme/WithAccessible/WithKeyMap/WithWidth/WithHeight/WithPosition/GetKey/GetValue` 全部薄代理到 `f.input`，其中：

- `WithKeyMap(k *huh.KeyMap) Field` 里先 `f.keymap = k`，再 `f.input = f.input.WithKeyMap(k).(*huh.Input)`，最后 `return f`
- 其他 `With*` 方法也都要把 `f.input = ...(*huh.Input)` 回写，返回 `f`

这样 `huh` 在统一应用 theme / width / keymap / position 时，包装字段不会被替换成裸 `*huh.Input`。

- [ ] **Step 4: 用包装字段替换 `editProvider()` 里的底部输入框**

把 Task 1 里的普通 `customModelField := huh.NewInput()...` 改成：

```go
customModelField := newProviderCustomModelInput(
	&customModelInput,
	func(raw string) providerCustomRegistration {
		result := registerProviderCustomModels(raw, &selectedModels, &customModelRegistry)
		if result.added {
			modelOptionsVersion++
		}
		return result
	},
)
```

然后把 `form := newForm(huh.NewGroup(...))` 里的最后一个字段从 `huh.NewInput()` 改成 `customModelField`。

此时 `editProvider()` 的模型段应该满足：

- 新增模型 → `registerProviderCustomModels()` 写入 registry + selected
- `modelOptionsVersion++` 触发 `OptionsFunc(...)` 刷新
- `customModelInput` 清空
- 当前字段返回 `huh.PrevField`
- `Group.prevField()` 把焦点切回上一个字段（即模型列表）

- [ ] **Step 5: 运行定向测试，确认包装字段行为通过**

Run:

```bash
go test ./... -run 'TestProvider(CustomInput|ModelRegister|Model(Split|Options|FinalSelection|ListTheme))'
```

Expected:

- 新增的 `TestProviderCustomInput...` 两条测试 PASS。
- Task 1 / Task 2 的 provider 模型测试仍然 PASS。

- [ ] **Step 6: 运行全量测试**

Run:

```bash
go test ./...
```

Expected:

- 全部测试通过。

- [ ] **Step 7: 手动回归验证 `editProvider()` 全流程**

Run:

```bash
go run .
```

按下面顺序手测，每条都要真的走一遍：

1. 新建 provider，进入“模型列表”后直接看样式
   - 预期：未选中前缀是 `[ ] `，选中前缀是 `[✓] `
2. 在最后一个“自定义模型名称”字段输入 `custom-alpha`，按 `enter`
   - 预期：返回到模型列表，不提交整个 Provider 表单
   - 预期：`custom-alpha` 立刻出现在列表底部，并默认勾选
   - 预期：输入框再次进入时已为空
3. 在自定义输入框一次输入 `custom-beta, custom-gamma`，按 `tab`
   - 预期：两项都立刻出现，且都默认勾选
4. 输入一个已存在的自定义模型名，比如 `custom-alpha`
   - 预期：不新增重复项
   - 预期：输入框被清空
   - 预期：焦点返回模型列表
5. 输入一个预设模型名，比如 `gpt-5.2`
   - 预期：不新增新的自定义项
   - 预期：对应预设模型被勾选
6. 把某个自定义模型取消勾选后保存
   - 预期：该模型不进入最终保存结果
7. 重新编辑刚保存过的 provider
   - 预期：上次保存过的自定义模型初始就能在列表中看到

- [ ] **Step 8: 提交**

```bash
git add app.go app_test.go
git commit -m "feat: register provider custom models into dynamic multiselect"
```

---

## 最终验收命令

全部任务完成后，再统一执行一遍：

```bash
go test ./...
```

期望：

- 所有测试通过
- 没有残留对 `mergeProviderModels(selectedPresets, customInput string)` 旧语义的依赖

---

## 完成定义（DoD）

- `editProvider()` 中输入全新自定义模型后，离开输入框即可立即出现在上方列表中
- 新增自定义模型默认自动选中
- 在最后一个输入框按 `enter/tab` 处理有效输入后，不会直接提交 Provider 表单，而是返回模型列表
- 输入已存在模型名时不会生成重复项，但输入框仍会清空并返回模型列表
- 删除输入框文字不会移除已注册到列表中的自定义模型
- 最终保存结果仅由 `selectedModels` 决定
- provider 模型列表前缀仅在该字段局部显示为 `[ ]` / `[✓]`
- 编辑已有 provider 时，历史自定义模型会显示在列表中
- `go test ./...` 通过
