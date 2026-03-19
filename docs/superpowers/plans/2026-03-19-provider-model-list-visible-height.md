# Provider 模型列表可见高度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `editProvider()` 里的 provider 模型列表在窗口足够大时尽量一次显示全部模型，在窗口较小时动态显示尽可能多的可见项，并明确提示还有多少模型未显示，同时保持现有自定义模型注册与保存行为不变。

**Architecture:** 保持本次改动严格局限在 `app.go` 的 provider 编辑页面。先把“可见行数/剩余提示”的规则提炼成纯计算 helper 并用单元测试锁定，再增加一个很薄的 `providerModelListField` 包装器：包装 `*huh.MultiSelect[string]`，在 `tea.WindowSizeMsg` 和后续更新中按当前窗口高度、其他字段渲染高度、固定帮助栏高度计算列表总高度，并同步更新 Description。`editProvider()` 继续负责组装字段和保存最终模型列表，不把这套行为扩散成全局长列表框架。

**Tech Stack:** Go 1.23, `github.com/charmbracelet/huh v0.6.0`, Bubble Tea, Lipgloss

---

## 实现前必读

### 当前代码位置

- `app.go:342-391`
  - `formModel` / `prepareFormForRun()` / `runForm()` / `newForm()`
  - 当前统一帮助栏入口
- `app.go:402-518`
  - provider 模型相关 helper：`splitProviderModelsForEdit()`、`buildProviderModelOptions()`、`finalProviderModels()`、`parseCustomModelInput()`、`registerProviderCustomModels()`
- `app.go:520-599`
  - `providerCustomModelInput` 字段包装器
- `app.go:609-725`
  - `editProvider()` 当前 provider 编辑表单
- `app_test.go:71-275`
  - provider 模型相关现有测试

### 已确认的 `huh v0.6.0` 约束

- `MultiSelect.Limit(...)` 限制的是“最多可选多少项”，**不是**可见行数。
- `MultiSelect.Height(...)` / `WithHeight(...)` 才能控制该字段的总高度。
- `field_multiselect.go:456-467`：MultiSelect 视口高度 = `height - titleHeight - descriptionHeight`。
- `form.go:510-527` + `group.go:270-272`：窗口缩小时，Form/Group 会按窗口高度压缩整体内容；所以 provider 模型列表需要自己的高度策略，否则就会只显示部分项且没有剩余提示。
- `group.go:263-264`：每次 Group 更新末尾都会给所有字段再发一次 `updateFieldMsg{}`，因此一个字段包装器只要在每次 `Update` 时根据最近窗口高度重新同步，就能吃到“窗口变化”和“自定义模型新增后 options 变化”这两类更新，不需要额外把 Description 做成 `DescriptionFunc(...)`。

### 不改动的边界

- 不改 `internal/config/manager.go`
- 不改 `internal/config/types.go`
- 不改 `internal/models/presets.go`
- 不把这套逻辑扩散到 Step 2 / Step 3 / 命名 Agent 的其他 Select/MultiSelect
- 不新增分页器、页码、进度条等额外交互

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `app.go` | 修改 | 新增 provider 模型列表展示 helper、可用高度计算 helper、`providerModelListField` 包装器，并把 `editProvider()` 改为显式持有字段变量后再组装表单 |
| `app_test.go` | 修改 | 为纯计算 helper 增加单元测试，锁定“全显/截断/剩余提示/可用高度预留”规则 |

---

## Task 1: 提炼 provider 模型列表展示规则为纯 helper

**Files:**
- Modify: `app_test.go:71-275`
- Modify: `app.go:402-492`

### 目标

把“窗口给模型列表多少总高度、当前能看到几项、是否要显示剩余提示、提示文案长什么样”从 TUI 代码里拆出来，变成纯函数。这样后续 TUI 包装器只负责取实时尺寸并调用这些 helper，不在 `Update()` 里手写公式。

- [ ] **Step 1: 先写失败测试，锁定展示 helper 的契约**

在 `app_test.go` 现有 provider 测试后面追加下面 4 个测试：

```go
func TestComputeProviderModelListPresentationShowsAllOptionsWhenSpaceAllows(t *testing.T) {
	got := computeProviderModelListPresentation(12, 4)
	want := providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestComputeProviderModelListPresentationShowsOverflowWhenSpaceIsTight(t *testing.T) {
	got := computeProviderModelListPresentation(8, 10)
	want := providerModelListPresentation{
		fieldHeight:      8,
		visibleRows:      4,
		hiddenCount:      6,
		showOverflowHint: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestProviderModelListDescriptionWithoutOverflowUsesBaseText(t *testing.T) {
	got := providerModelListDescription(providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	})
	want := providerModelListBaseDescription
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestProviderModelListDescriptionWithOverflowAppendsHiddenCount(t *testing.T) {
	got := providerModelListDescription(providerModelListPresentation{
		fieldHeight:      8,
		visibleRows:      4,
		hiddenCount:      6,
		showOverflowHint: true,
	})
	want := providerModelListBaseDescription + "\n当前仅显示前 4 项，还有 6 项可继续向下查看"
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription)'
```

Expected:

- 编译失败或测试失败，至少出现以下一种：
  - `undefined: computeProviderModelListPresentation`
  - `undefined: providerModelListPresentation`
  - `undefined: providerModelListDescription`
  - `undefined: providerModelListBaseDescription`

- [ ] **Step 3: 在 `app.go` 实现最小展示 helper**

在 `app.go` 的 provider 模型 helper 区域（`splitProviderModelsForEdit()` 附近）新增下面这组结构和函数：

```go
const providerModelListBaseDescription = "空格/x 切换选中，↑↓ 移动，enter 确认\n选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个"

const (
	providerModelListTitleLines           = 1
	providerModelListBaseDescriptionLines = 2
	providerModelListOverflowLines        = 1
)

type providerModelListPresentation struct {
	fieldHeight      int
	visibleRows      int
	hiddenCount      int
	showOverflowHint bool
}

func computeProviderModelListPresentation(availableFieldHeight int, optionCount int) providerModelListPresentation {
	if optionCount <= 0 {
		return providerModelListPresentation{
			fieldHeight:      providerModelListTitleLines + providerModelListBaseDescriptionLines + 1,
			visibleRows:      1,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	fullFieldHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + optionCount
	if availableFieldHeight >= fullFieldHeight {
		return providerModelListPresentation{
			fieldHeight:      fullFieldHeight,
			visibleRows:      optionCount,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	visibleRows := max(1, availableFieldHeight-providerModelListTitleLines-providerModelListBaseDescriptionLines-providerModelListOverflowLines)
	return providerModelListPresentation{
		fieldHeight:      max(providerModelListTitleLines+providerModelListBaseDescriptionLines+providerModelListOverflowLines+1, availableFieldHeight),
		visibleRows:      visibleRows,
		hiddenCount:      max(0, optionCount-visibleRows),
		showOverflowHint: true,
	}
}

func providerModelListDescription(p providerModelListPresentation) string {
	if !p.showOverflowHint || p.hiddenCount <= 0 {
		return providerModelListBaseDescription
	}
	return providerModelListBaseDescription + fmt.Sprintf("\n当前仅显示前 %d 项，还有 %d 项可继续向下查看", p.visibleRows, p.hiddenCount)
}
```

说明：

- `fieldHeight` 是传给 `MultiSelect.Height(...)` 的“总字段高度”，不是裸 option 行数。
- “截断分支”必须按**包含额外提示行**的描述高度计算，否则会出现 description 增一行后又把 `visibleRows` 挤少一行的循环依赖。
- 保持这组 helper 为纯函数，不依赖 `tea.Msg`、字段实例或全局状态。

- [ ] **Step 4: 重新运行定向测试，确认 helper 契约通过**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription)'
```

Expected: PASS

- [ ] **Step 5: 提交这一小步**

```bash
git add app.go app_test.go
git commit -m "test: lock provider model list presentation rules"
```

---

## Task 2: 增加 provider 专用 MultiSelect 包装器并接入 `editProvider()`

**Files:**
- Modify: `app_test.go:71-275`
- Modify: `app.go:342-725`

### 目标

让 provider 模型列表在运行时能真正根据窗口高度和其他字段占用空间调整自身高度，并使用 Task 1 的 helper 输出剩余提示。改动只影响 provider 页面，不改变 `runForm()` 的通用帮助栏机制。

- [ ] **Step 1: 先写失败测试，锁定“可用高度预留”规则**

在 `app_test.go` 继续追加下面 2 个测试：

```go
func TestProviderModelListAvailableFieldHeightReservesOtherViewsGapsAndFooter(t *testing.T) {
	got := providerModelListAvailableFieldHeight(
		30,
		"Provider 标识名\n说明\n> demo",
		"Base URL\n说明\n> https://example.com",
		"API Key\n> sk-...",
		"自定义模型名称（可选，多个可用逗号/换行分隔）\n> custom-alpha",
	)

	// 其他字段高度 3 + 3 + 2 + 2 = 10
	// 字段间隔 4 * 2 = 8
	// 底部帮助栏 1
	// 可留给模型列表的总高度 = 30 - 10 - 8 - 1 = 11
	if got != 11 {
		t.Fatalf("available height = %d, want 11", got)
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

- [ ] **Step 2: 运行定向测试，确认现在失败**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription|ProviderModelListAvailableFieldHeight)'
```

Expected:

- 新增测试失败，至少出现：`undefined: providerModelListAvailableFieldHeight`

- [ ] **Step 3: 在 `app.go` 实现可用高度 helper 和薄包装器**

先实现一个只做空间预留计算的纯 helper：

```go
const providerFieldGapLines = 2

func providerModelListAvailableFieldHeight(windowHeight int, otherViews ...string) int {
	reserved := lipgloss.Height(helpFooter)
	for _, view := range otherViews {
		reserved += lipgloss.Height(view)
	}
	reserved += len(otherViews) * providerFieldGapLines

	minFieldHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	return max(minFieldHeight, windowHeight-reserved)
}
```

然后新增一个 provider 专用字段包装器，模式和当前 `providerCustomModelInput` 一样：

```go
type providerModelListField struct {
	field                *huh.MultiSelect[string]
	measureAvailable     func(windowHeight int) int
	optionCount          func() int
	lastWindowHeight     int
	presentation         providerModelListPresentation
}
```

关键方法要求：

1. `syncPresentation(windowHeight int)`：
   - `available := measureAvailable(windowHeight)`
   - `presentation = computeProviderModelListPresentation(available, optionCount())`
   - `field.Description(providerModelListDescription(presentation))`
   - `field.Height(presentation.fieldHeight)`
2. `Update(msg tea.Msg)`：
   - 如果收到 `tea.WindowSizeMsg`，缓存 `msg.Height`
   - 如果 `lastWindowHeight > 0`，**每次 Update 都先调用一次 `syncPresentation(lastWindowHeight)`**，这样窗口变化和 options 变化都能同步到 description/height
   - 再把消息委托给内部 `field.Update(msg)`，并回写 `f.field = m.(*huh.MultiSelect[string])`
3. 其余 `Init/Blur/Focus/Error/Run/Skip/Zoom/KeyBinds/WithTheme/WithAccessible/WithKeyMap/WithWidth/WithHeight/WithPosition/GetKey/GetValue` 全部薄委托给内部 `*huh.MultiSelect[string]`

不要把这个包装器做成通用组件。类型名和注释都明确写成 provider 专用，避免误导后续把它扩散到别的表单。

- [ ] **Step 4: 改写 `editProvider()`，显式持有字段变量后再组装表单**

当前 `editProvider()` 里大部分字段是直接链式写进 `newForm(huh.NewGroup(...))`。这一步要把 5 个字段先各自存入局部变量，再组装 `Group`，因为 provider 模型列表包装器需要读取其他 4 个字段的 `View()` 来估算剩余空间。

按下面结构改：

```go
nameField := huh.NewInput()....Value(&name)
baseURLField := huh.NewInput()....Value(&baseUrl)
apiKeyField := huh.NewInput()....Value(&apiKey)
customModelField := newProviderCustomModelInput(...)
modelListField := newProviderModelListField(
	huh.NewMultiSelect[string]().
		Title("模型列表").
		Description(providerModelListBaseDescription).
		OptionsFunc(func() []huh.Option[string] {
			return buildProviderModelOptions(selectedModels, customModelRegistry)
		}, &modelOptionsVersion).
		Validate(func(selected []string) error {
			if len(finalProviderModels(selected)) == 0 {
				return fmt.Errorf("请至少选择一个模型")
			}
			return nil
		}).
		Value(&selectedModels).
		WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(windowHeight int) int {
			return providerModelListAvailableFieldHeight(
				windowHeight,
				nameField.View(),
				baseURLField.View(),
				apiKeyField.View(),
				customModelField.View(),
			)
		},
		func() int {
			return len(buildProviderModelOptions(selectedModels, customModelRegistry))
		},
)

form := newForm(huh.NewGroup(
	nameField,
	baseURLField,
	apiKeyField,
	modelListField,
	customModelField,
))
```

注意事项：

- 继续保留现有的 `modelOptionsVersion++` 逻辑，它仍然负责让 `OptionsFunc(...)` 在新增自定义模型后刷新 options。
- `Description(...)` 的基础文案不要删掉；包装器第一次同步前也要有合理的默认显示。
- 不要改 `finalProviderModels(selectedModels)` 的保存路径。
- 不要碰 `providerCustomModelInput` 的 enter/tab 拦截逻辑，避免把两个需求缠在一起。

- [ ] **Step 5: 跑测试，先小范围再全量**

Run:

```bash
go test ./... -run 'Test(ComputeProviderModelListPresentation|ProviderModelListDescription|ProviderModelListAvailableFieldHeight|ProviderModel)'
go test ./...
```

Expected:

- 所有 provider 相关测试通过
- 全量 `go test ./...` 通过

- [ ] **Step 6: 做一次手工 smoke test，确认真实终端体验**

Run:

```bash
go run .
```

手动验证两个窗口尺寸：

1. **较大终端**
   - 打开 provider 编辑页
   - “模型列表”应能直接显示全部当前模型
   - 不出现“还有 N 项”提示
2. **较小终端**
   - 打开同一页
   - “模型列表”只显示部分项，但会出现：`当前仅显示前 N 项，还有 M 项可继续向下查看`
   - 仍然能上下滚动访问所有模型
3. **新增自定义模型后**
   - 在底部输入框加入 1-2 个自定义模型
   - 新模型仍然立即出现、默认选中
   - 若模型总数因此超过可见行数，剩余提示会随之更新

- [ ] **Step 7: 提交这一小步**

```bash
git add app.go app_test.go
git commit -m "feat: make provider model list height responsive"
```

---

## 最终验收清单

- [ ] provider 编辑页的模型列表优先尽量显示更多模型
- [ ] 窗口足够大时可全部显示，无剩余提示
- [ ] 窗口较小时显示剩余提示，格式为：`当前仅显示前 N 项，还有 M 项可继续向下查看`
- [ ] 模型列表仍可滚动访问全部项
- [ ] 自定义模型动态注册行为不变
- [ ] provider 模型列表的 ASCII 复选框样式不变
- [ ] `go test ./...` 通过
- [ ] 手工 `go run .` smoke test 通过
