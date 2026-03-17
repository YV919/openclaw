# API 格式自动检测设计文档

**日期**: 2026-03-17
**状态**: 已批准
**范围**: `app.go`

---

## 背景

`editProvider` 表单目前向用户展示一个手动的"API 格式"下拉选择器（`openai-completions` / `anthropic-messages` / `openai-responses` / `google-generative-ai`）。`DetectAPIFormat` 函数已存在于 `internal/config/manager.go`，但被绕过：`apiFormat` 在表单前被初始化为 `"openai-completions"`，导致提交后的检测逻辑成为死代码。

目标：移除手动选择器，改为根据用户选择的模型列表自动推断格式。

---

## 检测规则

| 模型名称匹配                | API 格式                  |
|-----------------------------|---------------------------|
| `claude` 开头（不区分大小写）| `anthropic-messages`      |
| `-cc` 结尾（不区分大小写）  | `anthropic-messages`      |
| `gemini` 开头（不区分大小写）| `google-generative-ai`    |
| `gpt-5` 开头（不区分大小写）| `openai-responses`        |
| `o1-` / `o3-` / `o4-` 开头 | `openai-responses`        |
| 其他                         | `openai-completions`      |

这些规则已实现于 `config.DetectAPIFormat`，无需修改。

---

## 架构

### 唯一变更文件：`app.go`

#### 移除内容

1. `apiFormat` 变量及其默认值初始化（`app.go:326-328`）
2. `huh.NewSelect[string]` for "API 格式" 字段（`app.go:416-420`）
3. 提交后的死代码检测分支（`app.go:449-451`）

#### 新增内容

**`detectFormatFromModels(models []string) string`** — 私有辅助函数，放在 `app.go`：

```
输入：已整理的最终模型列表（非空）
逻辑：
  1. 对每个模型调用 config.DetectAPIFormat
  2. 收集所有唯一格式
  3. 若只有一种格式 → 直接返回
  4. 若存在冲突 → 打印黄色警告（列出各模型的检测结果），建议用户拆分 provider，返回第一个模型的格式
输出：string（API 格式标识符）
```

在 `editProvider` 的 `form.Run()` 之后调用：
```go
apiFormat := detectFormatFromModels(finalModels)
```

---

## 数据流

```
用户填写表单（名称/URL/Key/模型）
    ↓ form.Run()
整理 finalModels（过滤 __custom__，追加自定义）
    ↓
detectFormatFromModels(finalModels)
    ↓ 对每个 model 调用 DetectAPIFormat
    → 全部一致 → 直接返回格式
    → 存在冲突 → 打印警告，返回 finalModels[0] 的格式
    ↓
构建 ProviderConfig{ApiFormat: apiFormat, ...}
```

---

## 错误处理

- `finalModels` 为空时：由现有逻辑（`app.go:444-447`）已返回 error，`detectFormatFromModels` 不会被调用到空列表
- 冲突时：**不阻断**，输出黄色警告并继续，用户可在下次编辑时通过拆分 provider 解决

---

## 测试要点

- `claude-sonnet-4-6` → `anthropic-messages`
- `MiniMax-M2.5-cc` → `anthropic-messages`（`-cc` 后缀）
- `gemini-3.1-pro-preview` → `google-generative-ai`
- `gpt-5.2` → `openai-responses`
- `qwen-turbo` → `openai-completions`（默认）
- 混合模型列表（如 `["claude-sonnet-4-6", "gpt-5.2"]`）→ 输出警告，返回 `anthropic-messages`

---

## 不在范围内

- `DetectAPIFormat` 逻辑修改
- `internal/config/manager.go` 的任何改动
- `internal/models/presets.go` 的任何改动
- 编辑已有 provider 时的迁移（已有 `ApiFormat` 字段的 provider 在加载时保留原值，下次编辑时重新检测）
