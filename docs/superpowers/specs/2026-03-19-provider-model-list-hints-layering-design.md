# Provider 模型列表提示分层设计文档

**日期**：2026-03-19
**状态**：待实施
**模块**：`app.go`

---

## 背景与问题

当前 `editProvider()` 中的“模型列表”字段同时承担了三类信息：

1. **字段说明**：解释这里要选什么。
2. **字段状态**：当列表未完全显示时，提示还有多少项未显示。
3. **操作帮助**：提示如何切换选中、上下移动、确认。

但现有实现把第 2、3 类信息都放进了 `MultiSelect.Description()`：

- `app.go` 中当前 `providerModelListBaseDescription` 同时包含：
  - `空格/x 切换选中，↑↓ 移动，enter 确认`
  - `选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个`
- 当列表被截断时，还会继续在 `Description` 末尾追加：
  - `当前仅显示前 N 项，还有 M 项可继续向下查看`
- 与此同时，底部又有统一帮助栏：
  - `ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认`

这导致两个已确认的问题：

### 1. 信息层级混乱

用户反馈中，多选操作提示视觉上应该属于**底部统一帮助栏**，而不是“模型列表”的说明文字。当前把它放在 `Description` 中，会让人感觉它属于字段内容本身，而不是当前控件的操作方式。

### 2. 状态提示挂载位置不对

用户确认的目标不是把“当前仅显示前 N 项...”放到标题下面，而是：

- `Description` **只保留说明文案**
- 如果列表被截断，就在**选项列表下面**额外显示一条更醒目的状态提示，例如：

```text
↓ 更多模型（还有 3 项，继续向下查看）
```

也就是说，这次要把“说明”“状态”“操作”重新分层：

- **Description = 说明文案**
- **列表下方 = 当前可见性状态**
- **底部 footer = 当前字段的操作帮助**

---

## 已核实的实现事实

基于当前代码与 `huh v0.6.0` 实际接口，以下事实已经确认：

1. `providerModelListField` 已经不是裸 `huh.MultiSelect`，而是当前项目中的自定义包装层，负责：
   - 根据窗口高度计算可见区域
   - 设置 `Description`
   - 设置 `Height`
   - 自身实现 `View()/Focus()/Blur()/KeyBinds()` 等 `huh.Field` 接口

2. 当前统一帮助栏由 `formModel.View()` 在表单底部追加，因此 footer 逻辑并不受 `huh` 原生帮助栏约束，可以继续在项目包装层中扩展。

3. `huh.Form.KeyBinds()` 会返回当前聚焦字段的 `KeyBinds()`；而 `MultiSelect.KeyBinds()` 中确实包含：
   - Toggle
   - Up / Down
   - Prev / Submit / Next
   - SelectAll / SelectNone

4. `providerModelListField` 已有 `Focus()` / `Blur()` 包装点，因此可以在该字段聚焦时维护一份“当前处于模型列表字段”的状态，供 footer 渲染层读取。

5. 当前“列表可见高度”功能已经稳定工作，本次不推翻该能力，只调整：
   - 描述文本的职责
   - 溢出提示的挂载位置
   - 底部帮助栏在模型列表聚焦时的扩展内容

---

## 目标

在不改变 `editProvider()` 页面字段顺序、模型选择逻辑、自定义模型动态注册逻辑的前提下，实现以下行为：

1. `模型列表` 的 `Description` 只保留说明文案：

```text
选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个
```

2. 当模型列表没有被截断时：
   - 不显示“更多模型”提示
   - 列表保持当前可见高度策略

3. 当模型列表被截断时：
   - 在**选项列表下面**额外显示一条醒目的状态提示，例如：

```text
↓ 更多模型（还有 3 项，继续向下查看）
```

4. 底部统一帮助栏默认仍显示：

```text
ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认
```

5. 当焦点位于 `模型列表` 这个 `MultiSelect` 字段时，底部帮助栏扩展为：

```text
ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认
```

6. 离开 `模型列表` 字段后，footer 恢复为默认版本。

---

## 方案比较

### 方案 A（推荐）：Description 只做说明 + 列表下状态提示 + footer 动态扩展

将三类信息拆开：

- `Description` 只显示说明文案
- “更多模型”提示在 `providerModelListField.View()` 中追加到列表下方
- footer 在模型列表聚焦时扩展为多选专用帮助

**优点**：

- 完全符合用户已确认的交互要求
- 信息层级清晰，视觉归属正确
- 改动仍然集中在 `app.go` 包装层
- 后续其他字段若需要局部状态提示，也可复用同样思路

**缺点**：

- 需要同时调整列表包装层和 footer 渲染层

### 方案 B：Description 只做说明 + 溢出提示仍放 Description 末尾 + footer 动态扩展

**优点**：

- 改动较小

**缺点**：

- 不符合用户确认的“状态提示应在列表下面”要求
- 状态提示仍然会显得像字段说明的一部分

### 方案 C：把“更多模型”和多选帮助都放 footer

**优点**：

- footer 信息更集中

**缺点**：

- footer 承担过多职责
- “还有多少项未显示”是字段局部状态，不应跑到全局帮助栏
- 不符合用户已确认的信息分层

**结论**：采用方案 A。

---

## 设计

### 1. Description 调整

当前 `providerModelListBaseDescription` 需要收敛为单一职责，只保留说明文案：

```text
选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个
```

不再包含：

- `空格/x 切换选中，↑↓ 移动，enter 确认`
- `当前仅显示前 N 项，还有 M 项可继续向下查看`

### 2. 列表下方状态提示

#### 2.1 显示条件

当 `providerModelListPresentation.showOverflowHint == true` 且 `hiddenCount > 0` 时，在模型选项列表下方追加一行状态提示。

#### 2.2 文案

最终采用用户接受的风格：

```text
↓ 更多模型（还有 N 项，继续向下查看）
```

其中：

- `N = hiddenCount`

#### 2.3 挂载位置

这条提示不进入 `Description()`，而是在 `providerModelListField.View()` 中拼接到 `f.field.View()` 的返回值之后。

换句话说：

```text
标题
说明文案
[✓] ...
[ ] ...
[ ] ...
↓ 更多模型（还有 N 项，继续向下查看）
```

#### 2.4 为什么放在 `View()` 而不是 `Description()`

因为这是“当前渲染状态”，不是“字段说明”。

放在 `View()` 层有两个直接好处：

1. 不会再和字段说明混在一起。
2. 可以单独控制样式，让它比普通说明更显眼。

### 3. 高度策略兼容

当前项目已有 provider 模型列表的动态高度计算逻辑，本次不重写策略，只调整“溢出提示”占位方式。

为了保持高度计算稳定：

- 仍然保留“溢出提示最多占 1 行”的预算概念
- 但这 1 行不再计入 `Description`，而是作为 `providerModelListField` 整体 block 的附加渲染

也就是说，模型列表这一整块区域的总高度仍然需要为以下内容留空间：

1. 标题
2. Description（仅说明文案）
3. 可见选项列表
4. 如有溢出时的 1 行“更多模型”提示

这样可以避免出现新的循环依赖：

```text
提示在 Description 中变长
-> Description 高度变大
-> 列表变短
-> hiddenCount 变化
-> 提示再次变化
```

把溢出提示从 `Description` 拆出去后，逻辑会更稳定。

### 4. footer 动态扩展

#### 4.1 默认 footer

默认保持当前统一帮助栏：

```text
ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认
```

#### 4.2 模型列表聚焦时的 footer

当当前焦点位于 `providerModelListField` 时，footer 切换为：

```text
ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认
```

#### 4.3 焦点来源

推荐在 `providerModelListField.Focus()` / `Blur()` 中维护显式的 `focused` 状态，由 `runForm()` / `formModel` 的 footer 渲染逻辑读取。

推荐这种方式，而不是在 footer 层通过字符串解析 `KeyBinds()` 猜测字段类型，原因是：

- 可读性更强
- 行为边界更清晰
- 只影响当前 provider 模型列表字段，不扩大为全局“自动推断字段类型”逻辑

#### 4.4 实现边界

本次只要求 `editProvider()` 页面中的模型列表支持动态 footer 扩展，不要求把这套机制泛化到所有 `MultiSelect` / `Select` / `Input` 字段。

如果未来有类似需求，可以在这次实现基础上再抽象。

### 5. 推荐的实现结构

为了让变更尽量小且测试清晰，推荐引入或调整以下辅助函数：

1. `providerModelListBaseDescription`：仅保留说明文案
2. `providerModelListOverflowHint(...) string`：专门生成“↓ 更多模型（还有 N 项，继续向下查看）”
3. `providerModelListField.View()`：负责在 `f.field.View()` 之后附加 overflow hint
4. `providerEditorHelpFooter(modelListFocused bool) string`：按当前焦点决定 footer 文案
5. `runForm(...)` / `formModel`：允许 provider 编辑页为 footer 提供一个动态渲染来源

---

## 改动范围

### 修改文件

- `app.go`
- `app_test.go`

### 不改动

- `internal/config/manager.go`
- `internal/config/types.go`
- `internal/models/presets.go`
- 其他步骤（主 Agent / 子 Agent / 命名 Agent）的字段布局

---

## 测试策略

本次按 TDD 执行，至少覆盖以下行为：

### 1. Description 只保留说明文案

更新现有测试，使 `providerModelListDescription(...)` 在无溢出和有溢出两种情况下都只返回基础说明文案，不再拼接操作提示和剩余数量。

### 2. overflow hint 单独生成

新增测试验证当 `hiddenCount > 0` 时，专用函数返回：

```text
↓ 更多模型（还有 N 项，继续向下查看）
```

当没有溢出时，不返回该提示。

### 3. footer 文案切换

新增测试验证：

- 普通状态返回默认 footer
- 模型列表聚焦时返回扩展 footer

### 4. 现有高度相关测试保持语义正确

现有“可见行数 / hiddenCount / 可用高度”测试继续保留，但预期值可能需要根据“Description 只剩说明文案、overflow hint 脱离 Description”做相应调整。

---

## 验收标准

- [ ] `模型列表` 的 `Description` 只显示说明文案
- [ ] 操作提示 `空格/x 切换选中，↑↓ 移动，enter 确认` 不再出现在 `Description` 中
- [ ] 列表有溢出时，模型选项下方显示 `↓ 更多模型（还有 N 项，继续向下查看）`
- [ ] 列表无溢出时，不显示“更多模型”提示
- [ ] 底部统一帮助栏默认保持基础版本
- [ ] 焦点位于模型列表字段时，底部帮助栏扩展显示多选操作提示
- [ ] 离开模型列表字段后，底部帮助栏恢复默认版本
- [ ] 自定义模型动态注册、默认选中、ASCII 复选框样式等既有行为保持不变
- [ ] `go test ./...` 全部通过

---

## 风险与边界

1. **高度预算风险**
   - 由于“更多模型”提示从 `Description` 挪到列表 block 外层，必须确保总高度预算仍然为这 1 行留位。
   - 否则可能出现提示被挤掉，或下方 `自定义模型` 输入框位置异常。

2. **footer 动态性边界**
   - 本次 footer 动态扩展仅针对 provider 模型列表字段，不泛化为整个项目的“所有字段都动态帮助”。
   - 这样可以降低回归风险。

3. **样式边界**
   - “更多模型”提示应比普通说明更显眼，但仍与当前 TUI 风格保持一致，不引入过强对比色或破坏现有主题。

---

## 实施后续

设计确认后，下一步应进入实现计划与 TDD：

1. 先补测试，让新行为预期失败
2. 再修改 `app.go`
3. 运行 `go test ./...` 验证
