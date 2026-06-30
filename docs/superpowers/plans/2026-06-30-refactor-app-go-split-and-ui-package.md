# openclaw_config 重构计划：拆分 app.go + 抽取 UI 组件层

**日期**：2026-06-30
**目标**：将 1757 行的 app.go 按职责拆分，并将通用 UI 组件抽取到 internal/ui/ 包

---

## 现状分析

app.go (1757行) 承担了以下职责：
1. **快速配置流程** (~200行)：runQuickSetup, runQuickStep1Provider, runQuickStep2PrimaryModel
2. **高级配置流程** (~250行)：runAdvancedSetup, runStep1Providers, runStep2MainAgent, runStep3SubAgent, runStep4NamedAgents
3. **Provider 编辑器** (~250行)：editProvider, deleteProvider, pickProviderAction, pickProviderItemAction
4. **命名 Agent 编辑器** (~150行)：pickNamedAgentAction, pickNamedAgentItemAction, editNamedAgent
5. **自定义 UI 组件** (~400行)：
   - successDismissField (自定义 huh.Field)
   - providerModelListField (自定义 huh.Field，自适应高度 MultiSelect)
   - providerCustomModelInput (自定义 huh.Field，带 onSubmit 回调的 Input)
6. **通用 UI 工具** (~200行)：newForm, runForm, formModel, chineseKeyMap, helpFooter, renderHelpFooter
7. **辅助/工具函数** (~150行)：buildAllModelOpts, containsOptValue, appendUniqueStrings, cloneProviders, detectFormatFromModels, 等
8. **Banner/Success 输出** (~100行)：printBanner, printSuccess
9. **类型定义** (~30行)：App, setupMode, quickSetupSnapshot, providerCustomRegistration, providerModelListPresentation

---

## 重构方案

### 方案 A（推荐）：同包拆文件 + 新建 internal/ui/ 包

**策略**：
1. 在 main 包内拆成多个文件（快速配置、高级配置、Provider 编辑器、命名 Agent 编辑器、辅助函数、Banner/Success）
2. 将通用 UI 组件和工具抽取到 internal/ui/ 包

**上游侵入面**：

| 文件 | 改动类型 | 预估行数 | 是否上游已有 |
|---|---|---|---|
| `app.go` | 拆分删除 | -1757（内容分散到新文件） | 是 |
| `app_test.go` | 调整 import + 函数/类型引用 | ~30行改动 | 是 |
| `internal/ui/form.go` | 新增 | ~120 | 否 |
| `internal/ui/keymap.go` | 新增 | ~40 | 否 |
| `internal/ui/components.go` | 新增 | ~200 | 否 |
| `internal/ui/styles.go` | 新增 | ~40 | 否 |
| `quick_setup.go` | 新增 | ~200 | 否 |
| `advanced_setup.go` | 新增 | ~250 | 否 |
| `provider_editor.go` | 新增 | ~350 | 否 |
| `named_agent_editor.go` | 新增 | ~150 | 否 |
| `helpers.go` | 新增 | ~100 | 否 |
| `output.go` | 新增 | ~120 | 否 |

**合并冲突风险**：低。所有改动都在同一仓库内部，不涉及外部依赖变更。

### 方案 B：仅拆文件，不抽取 ui 包

**缺点**：自定义 Field 实现（每个 50-100 行样板代码）仍留在 main 包，main 包仍会膨胀。后续新增 UI 组件时没有清晰的复用层。

**结论**：采用方案 A。

---

## 目标文件结构

```
openclaw_config/
├── main.go                    # 入口（不变）
├── app.go                     # App 结构体 + Run() 入口 + 模式选择 (~100行)
├── quick_setup.go             # 快速配置流程 (~200行)
├── advanced_setup.go          # 高级配置流程 (~250行)
├── provider_editor.go         # Provider 编辑/删除/列表管理 (~350行)
├── named_agent_editor.go      # 命名 Agent 编辑/删除/列表管理 (~150行)
├── helpers.go                 # 辅助/工具函数 (~100行)
├── output.go                  # Banner、Success 输出 (~120行)
├── update.go                  # 版本检查（不变）
├── app_test.go                # 测试（调整引用，测试内容不变）
├── internal/
│   ├── config/                # 不变
│   │   ├── manager.go
│   │   ├── types.go
│   │   └── migration.go
│   ├── models/                # 不变
│   │   └── presets.go
│   └── ui/                    # 新建
│       ├── form.go            # FormModel, NewForm, RunForm, RunFormWithFooter, PrepareFormForRun
│       ├── keymap.go          # ChineseKeyMap
│       ├── components.go      # SuccessDismissField, ProviderModelListField, ProviderCustomModelInput + 相关类型
│       └── styles.go          # RenderHelpFooter, DefaultHelpFooter, ProviderModelListTheme 等
```

---

## 拆分细节

### 1. internal/ui/form.go（~120行）

从 app.go 迁移：
- `FormModel` struct（**导出**，原 formModel）+ Init/Update/View
  - 字段 `Form *huh.Form`（导出）、`HelpFooterView func() string`（导出）
- `PrepareFormForRun()` (导出)
- `RunForm()` (导出)
- `RunFormWithFooter()` (导出)
- `NewForm()` (导出，原 newForm)

### 2. internal/ui/keymap.go（~40行）

从 app.go 迁移：
- `ChineseKeyMap()` (导出)

### 3. internal/ui/styles.go（~40行）

从 app.go 迁移：
- `RenderHelpFooter()` (导出)
- `DefaultHelpFooter` (导出变量)
- `HelpFooter` (导出变量)
- `ProviderModelListTheme()` (导出)
- 常量：DefaultHelpFooterText, ProviderModelListFocusedHelpFooterText, ProviderFieldGapLines

### 4. internal/ui/components.go（~200行）

从 app.go 迁移三个自定义 huh.Field 实现：

**4a. SuccessDismissField**
- `SuccessDismissField` struct + 全部 huh.Field 方法
- `NewSuccessDismissField()` (导出)

**4b. ProviderModelListField**
- `ProviderModelListField` struct + 全部 huh.Field 方法 + syncPresentation
- `NewProviderModelListField()` (导出)
- **字段导出**：`lastWindowHeight` → `LastWindowHeight`（测试中直接赋值）

**4c. ProviderCustomModelInput**
- `ProviderCustomModelInput` struct + 全部 huh.Field 方法
- `NewProviderCustomModelInput()` (导出)

**4d. 导出类型 + 字段重命名**
- `ProviderCustomRegistration` struct
  - `handled` → `Handled`
  - `added` → `Added`
- `ProviderModelListPresentation` struct
  - `fieldHeight` → `FieldHeight`
  - `visibleRows` → `VisibleRows`
  - `hiddenCount` → `HiddenCount`
  - `showOverflowHint` → `ShowOverflowHint`

**4e. 导出函数**
- `ComputeProviderModelListPresentation()` (导出)
- `ProviderModelListOverflowHint()` (导出)
- `ProviderModelListRemainingBelow()` (导出)
- `ProviderModelListAvailableFieldHeight()` (导出)

**4f. 导出常量**
- `ProviderModelListTitleLines`
- `ProviderModelListBaseDescriptionLines`
- `ProviderModelListOverflowLines`
- `ProviderModelListBaseDescription`

### 5. app.go（保留 ~100行）

保留：
- `App` struct
- `NewApp()`
- `(*App).Run()` —— 主入口，加载配置、检查更新、打印 banner、选择模式、分流、保存
- `setupMode` 类型 + 常量
- `pickSetupMode()`
- `setupModeDescription()`

### 6. quick_setup.go（~200行）

迁移：
- `quickSetupSnapshot` struct
- `(*App).runQuickSetup()`
- `(*App).runQuickStep1Provider()`
- `(*App).runQuickStep2PrimaryModel()`
- `pickQuickSetupAction()`
- `quickSetupSummaryDescription()`
- `prepareQuickSetup()`
- `restoreQuickSetupSnapshot()`
- `applyQuickProviderSelection()`
- `applyQuickPrimaryModel()`
- `buildQuickPrimaryModelOptions()`
- `hasAdvancedConfig()`

### 7. advanced_setup.go（~250行）

迁移：
- `(*App).runAdvancedSetup()`
- `(*App).runStep1Providers()` (Provider 管理循环)
- `(*App).runStep2MainAgent()`
- `(*App).runStep3SubAgent()`
- `(*App).runStep4NamedAgents()`

### 8. provider_editor.go（~350行）

迁移：
- `editProvider()` —— 核心 Provider 编辑表单
- `deleteProvider()`
- `pickProviderAction()`
- `pickProviderItemAction()`
- `providerManagementDescription()`
- `savedConfigNoticeDescription()`
- `buildAllModelOpts()`
- `presetModelSet()` ← **计划验证发现的遗漏**
- `splitProviderModelsForEdit()`
- `buildProviderModelOptions()`
- `finalProviderModels()`
- `parseCustomModelInput()`
- `registerProviderCustomModels()`
- `detectFormatFromModels()`
- `checkProviderDeps()`
- `providerEditorHelpFooter()`

### 9. named_agent_editor.go（~150行）

迁移：
- `pickNamedAgentAction()`
- `pickNamedAgentItemAction()`
- `editNamedAgent()`

### 10. helpers.go（~100行）

迁移：
- `cloneProviders()`
- `cloneNamedAgents()`
- `containsOptValue()`
- `appendUniqueStrings()`

### 11. output.go（~120行）

迁移：
- `printBanner()`
- `printSuccess()`

---

## 实施顺序

1. **创建 internal/ui/ 包**：先迁移通用 UI 组件（form.go → keymap.go → styles.go → components.go）
2. **拆分 app.go**：按文件创建 quick_setup.go → advanced_setup.go → provider_editor.go → named_agent_editor.go → helpers.go → output.go
3. **精简 app.go**：只保留 App struct + Run() + 模式选择
4. **更新 app_test.go**：调整函数/类型/字段引用（详见测试策略）
5. **验证**：go build + go test ./...

---

## 测试策略

app_test.go 中的测试函数不改名、不改逻辑。需要调整的引用分为三类：

### A. 函数/变量引用变更（迁移到 ui 包）

| 旧引用 | 新引用 |
|---|---|
| `newForm(...)` | `ui.NewForm(...)` |
| `chineseKeyMap()` | `ui.ChineseKeyMap()` |
| `helpFooter` | `ui.HelpFooter` |
| `renderHelpFooter(...)` | `ui.RenderHelpFooter(...)` |
| `providerModelListTheme()` | `ui.ProviderModelListTheme()` |
| `providerModelListBaseDescription` | `ui.ProviderModelListBaseDescription` |
| `newProviderModelListField(...)` | `ui.NewProviderModelListField(...)` |
| `newProviderCustomModelInput(...)` | `ui.NewProviderCustomModelInput(...)` |
| `newSuccessDismissField(...)` | `ui.NewSuccessDismissField(...)` |
| `computeProviderModelListPresentation(...)` | `ui.ComputeProviderModelListPresentation(...)` |
| `providerModelListOverflowHint(...)` | `ui.ProviderModelListOverflowHint(...)` |
| `providerModelListAvailableFieldHeight(...)` | `ui.ProviderModelListAvailableFieldHeight(...)` |
| `prepareFormForRun(...)` | `ui.PrepareFormForRun(...)` |
| `providerModelListTitleLines` | `ui.ProviderModelListTitleLines` |
| `providerModelListBaseDescriptionLines` | `ui.ProviderModelListBaseDescriptionLines` |
| `providerModelListOverflowLines` | `ui.ProviderModelListOverflowLines` |

### B. 类型引用变更（迁移到 ui 包，需导出）

| 旧引用 | 新引用 |
|---|---|
| `providerCustomRegistration{...}` | `ui.ProviderCustomRegistration{...}` |
| `.handled` | `.Handled` |
| `.added` | `.Added` |
| `providerModelListPresentation{...}` | `ui.ProviderModelListPresentation{...}` |
| `.fieldHeight` | `.FieldHeight` |
| `.visibleRows` | `.VisibleRows` |
| `.hiddenCount` | `.HiddenCount` |
| `.showOverflowHint` | `.ShowOverflowHint` |

### C. 结构体直接构造变更（迁移到 ui 包，需导出类型和字段）

| 旧引用 | 新引用 |
|---|---|
| `&formModel{form: ..., helpFooterView: ...}` | `&ui.FormModel{Form: ..., HelpFooterView: ...}` |
| `field.lastWindowHeight = 8` | `field.LastWindowHeight = 8` |
| `model.(*providerModelListField)` | `model.(*ui.ProviderModelListField)` |

### D. 不需要调整的（仍在 main 包）

`detectFormatFromModels`, `prepareQuickSetup`, `restoreQuickSetupSnapshot`, `hasAdvancedConfig`, `applyQuickProviderSelection`, `applyQuickPrimaryModel`, `buildQuickPrimaryModelOptions`, `splitProviderModelsForEdit`, `buildProviderModelOptions`, `finalProviderModels`, `parseCustomModelInput`, `registerProviderCustomModels`, `containsOptValue`, `appendUniqueStrings`, `setupModeDescription`, `quickSetupSummaryDescription`, `providerManagementDescription`, `savedConfigNoticeDescription`, `providerEditorHelpFooter`, `presetModelSet`

---

## 验证步骤

1. `go build ./...` —— 编译通过
2. `go test ./...` —— 所有测试通过
3. `go vet ./...` —— 无警告
4. 手动运行 `go run . --version` —— 版本号正确输出
