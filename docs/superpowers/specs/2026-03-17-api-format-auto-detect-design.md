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

| 模型名称匹配                              | API 格式                  |
|-------------------------------------------|---------------------------|
| `claude` 开头（不区分大小写）             | `anthropic-messages`      |
| `-cc` 结尾（不区分大小写）               | `anthropic-messages`      |
| `gemini` 开头（不区分大小写）             | `google-generative-ai`    |
| `gpt-5` 开头（不区分大小写）             | `openai-responses`        |
| 恰好等于 `o1`/`o3`/`o4`，或以 `o1-`/`o3-`/`o4-` 开头 | `openai-responses` |
| 其他                                      | `openai-completions`      |

这些规则已实现于 `config.DetectAPIFormat`，无需修改。

---

## 架构

### 唯一变更文件：`app.go`

#### 移除内容

以下代码块全部位于 `editProvider` 函数内，按从上到下顺序：

1. **`apiFormat` 相关变量**：包括 `apiFormat := p.ApiFormat` 的读取行及其后的 `if apiFormat == ""` 默认值覆盖块（共约 4 行）
2. **`huh.NewSelect[string]` for "API 格式"**：表单中整个 API 格式选择字段（约 5 行）
3. **死代码检测分支**：`form.Run()` 之后的 `if apiFormat == "" { apiFormat = config.DetectAPIFormat(...) }` 块（约 4 行）

此外还需移除：
4. **`apiFormatOpts` 包级变量**（`app.go` 顶部，约 5 行）— 移除 Select 字段后无任何引用，需同步删除

#### 新增内容

**`detectFormatFromModels(models []string) string`** — 私有辅助函数，放在 `app.go`：

```
输入：已整理的最终模型列表（类型 []string，调用方保证非空）
逻辑：
  1. 对每个模型调用 config.DetectAPIFormat，收集所有唯一格式
  2. 若只有一种格式（包括列表只有一个元素的情形）→ 直接返回
  3. 若存在冲突 → 打印黄色警告到 stdout（与兼容性修复日志风格一致），
     列出各模型的检测结果，建议用户拆分 provider，返回 models[0] 的格式
输出：string（API 格式标识符之一）
```

#### 调用顺序（重要）

当前代码中，`NormalizeBaseURL` 在 `finalModels` 整理之前调用（约 `form.Run()` 后第 2 行），而 `finalModels` 整理在后（约第 10-20 行）。

实施后需调整顺序为：

```go
// 1. 整理 finalModels（过滤 __custom__，追加自定义）
finalModels := ...

// 2. 先确定 apiFormat（后续 NormalizeBaseURL 依赖此值）
apiFormat := detectFormatFromModels(finalModels)

// 3. 再规范化 URL（google-generative-ai 不追加 /v1，其他格式追加）
baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)

// 4. 构建 ProviderConfig（此行无需修改）
return config.ProviderConfig{..., ApiFormat: apiFormat, ...}
```

`NormalizeBaseURL` 对 `google-generative-ai` 不追加 `/v1`，对其他三种格式追加。若使用错误的 `apiFormat` 值（如 `"openai-completions"`）规范化 Gemini URL，会错误追加 `/v1`，因此调用顺序不可颠倒。

---

## 数据流

```
用户填写表单（名称/URL/Key/模型）
    ↓ form.Run()
整理 finalModels（过滤 __custom__，追加自定义）
    ↓
detectFormatFromModels(finalModels)       ← apiFormat 在此确定
    ↓ 对每个 model 调用 DetectAPIFormat
    → 只有一种格式（含单模型情形）→ 直接返回
    → 存在冲突 → 打印警告，返回 models[0] 的格式
    ↓
NormalizeBaseURL(baseUrl, apiFormat)      ← 必须在 apiFormat 确定后执行
    ↓
return ProviderConfig{ApiFormat: apiFormat, ...}
```

---

## 错误处理

- `finalModels` 为空时：由现有逻辑已返回 error，`detectFormatFromModels` 不会被调用到空列表
- 冲突时：**不阻断**，输出黄色警告到 stdout 并继续，用户可在下次编辑时通过拆分 provider 解决

---

## 编辑已有 Provider 的行为（预期行为，非副作用）

每次通过 `editProvider` 编辑 provider 时，无论是否更改模型列表，都会对最终模型列表重新运行 `detectFormatFromModels`，其结果将覆盖之前保存的 `ApiFormat`。这是**预期行为**：自动检测取代手动设置，用户不再需要记忆或手动维护格式字段。

---

## 测试要点

- `claude-sonnet-4-6` → `anthropic-messages`（`claude` 前缀）
- `MiniMax-M2.5-cc` → `anthropic-messages`（`-cc` 后缀）
- `gemini-3.1-pro-preview` → `google-generative-ai`
- `gpt-5.2` → `openai-responses`（`gpt-5` 前缀）
- `gpt-5.3-codex` → `openai-responses`（`gpt-5` 前缀，带后缀变体）
- `o1` → `openai-responses`（裸前缀，无连字符）
- `o3-mini` → `openai-responses`（`o3-` 前缀）
- `qwen-turbo` → `openai-completions`（默认）
- 单模型列表 `["claude-sonnet-4-6"]` → `anthropic-messages`（无冲突路径）
- 混合模型列表 `["claude-sonnet-4-6", "gpt-5.2"]` → 输出黄色警告，返回 `anthropic-messages`（取 models[0] 的格式）

---

## 不在范围内

- `DetectAPIFormat` 逻辑修改
- `internal/config/manager.go` 的任何改动
- `internal/models/presets.go` 的任何改动
