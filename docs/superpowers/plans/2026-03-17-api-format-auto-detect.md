# API 格式自动检测 Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从 `editProvider` 表单移除手动"API 格式"选择器，改为提交后根据模型列表自动推断。

**Architecture:** 在 `app.go` 新增私有函数 `detectFormatFromModels`，复用已有的 `config.DetectAPIFormat`。移除表单中的 `huh.NewSelect` 字段和相关死代码，调整 `NormalizeBaseURL` 调用顺序至 `apiFormat` 确定之后。

**Tech Stack:** Go 1.23，charmbracelet/huh v0.6.0，charmbracelet/lipgloss（黄色警告样式）

**Spec:** `docs/superpowers/specs/2026-03-17-api-format-auto-detect-design.md`

---

## 文件变更清单

| 操作 | 文件 | 说明 |
|------|------|------|
| Modify | `app.go` | 移除 `apiFormatOpts`、API 格式 Select 字段、死代码；新增 `detectFormatFromModels`；调整调用顺序 |
| Create | `app_test.go` | `detectFormatFromModels` 单元测试 |

---

### Task 1: 新增 `detectFormatFromModels` 并写测试

**Files:**
- Create: `app_test.go`
- Modify: `app.go`（仅追加函数，暂不改表单）

- [ ] **Step 1: 创建测试文件，写失败测试**

创建 `/Users/yesongyun/代码/openclaw_config/app_test.go`，内容如下：

```go
package main

import (
	"io"
	"os"
	"testing"
)

// suppressStdout 将 os.Stdout 重定向到 /dev/null，用于抑制测试中的警告输出
func suppressStdout(f func()) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	io.ReadAll(r) //nolint:errcheck
}

func TestDetectFormatFromModels(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{"claude 前缀", []string{"claude-sonnet-4-6"}, "anthropic-messages"},
		{"-cc 后缀", []string{"MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"gemini 前缀", []string{"gemini-3.1-pro-preview"}, "google-generative-ai"},
		{"gpt-5 前缀", []string{"gpt-5.2"}, "openai-responses"},
		{"gpt-5 带后缀变体", []string{"gpt-5.3-codex"}, "openai-responses"},
		{"o1 裸前缀", []string{"o1"}, "openai-responses"},
		{"o3-mini 带连字符", []string{"o3-mini"}, "openai-responses"},
		{"默认 openai-completions", []string{"qwen-turbo"}, "openai-completions"},
		{"单模型无冲突路径", []string{"claude-opus-4-6"}, "anthropic-messages"},
		{"多模型一致格式", []string{"claude-sonnet-4-6", "MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"冲突时取第一个", []string{"claude-sonnet-4-6", "gpt-5.2"}, "anthropic-messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			suppressStdout(func() {
				got = detectFormatFromModels(tt.models)
			})
			if got != tt.expected {
				t.Errorf("detectFormatFromModels(%v) = %q, want %q", tt.models, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试，确认失败（函数未定义）**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test -run TestDetectFormatFromModels -v ./...
```

预期输出：编译错误 `undefined: detectFormatFromModels`

- [ ] **Step 3: 在 `app.go` 末尾追加 `detectFormatFromModels` 函数**

在 `app.go` 的 `printBanner` 函数之后追加：

```go
// detectFormatFromModels 根据模型列表自动推断 API 格式。
// 若所有模型格式一致（含单模型情形），直接返回该格式。
// 若存在冲突，打印黄色警告并返回第一个模型的格式。
// 调用方保证 models 非空。
func detectFormatFromModels(models []string) string {
	seen := make(map[string]bool, len(models))
	for _, m := range models {
		seen[config.DetectAPIFormat(m)] = true
	}
	if len(seen) == 1 {
		for f := range seen {
			return f
		}
	}
	// 存在冲突：打印警告，返回第一个模型的格式
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	fmt.Println(yellow.Render("  ⚠ 所选模型包含不同 API 格式，将使用第一个模型的格式："))
	for _, m := range models {
		fmt.Println(dim.Render(fmt.Sprintf("    · %s → %s", m, config.DetectAPIFormat(m))))
	}
	fmt.Println(dim.Render("  建议：将不同格式的模型拆分为独立 provider"))
	fmt.Println()
	return config.DetectAPIFormat(models[0])
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test -run TestDetectFormatFromModels -v ./...
```

预期输出：所有 11 个子测试 PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add app.go app_test.go
git commit -m "feat: add detectFormatFromModels with tests"
```

---

### Task 2: 移除表单中的 API 格式选择器及相关死代码

**Files:**
- Modify: `app.go`（`editProvider` 函数内部改动）

**前置条件**：Task 1 已完成，`detectFormatFromModels` 已存在。

- [ ] **Step 1: 移除 `apiFormatOpts` 包级变量**

定位 `app.go:311-316`：

```go
var apiFormatOpts = []huh.Option[string]{
	huh.NewOption("openai-completions  (GPT / 通用兼容)", "openai-completions"),
	huh.NewOption("anthropic-messages  (Claude)", "anthropic-messages"),
	huh.NewOption("openai-responses    (GPT-5 / o 系列)", "openai-responses"),
	huh.NewOption("google-generative-ai (Gemini)", "google-generative-ai"),
}
```

整块删除（共 6 行）。

- [ ] **Step 2: 移除 `editProvider` 中的 `apiFormat` 初始化块**

定位 `editProvider` 函数内，找到以下两行（顺序相邻）：

```go
apiFormat := p.ApiFormat
if apiFormat == "" {
	apiFormat = "openai-completions"
}
```

整块删除（共 4 行）。

- [ ] **Step 3: 移除表单中的 API 格式 Select 字段**

定位 `editProvider` 表单中的 `huh.NewSelect[string]` API 格式字段：

```go
		huh.NewSelect[string]().
			Title("API 格式").
			Description("不确定时选 openai-completions。若同一 provider 含不同格式模型，请分拆为多个 provider").
			Options(apiFormatOpts...).
			Value(&apiFormat),
```

整块删除（含尾部逗号或紧跟的 `)`，共约 5 行）。

- [ ] **Step 4: 移除 `form.Run()` 之后的死代码检测分支**

定位 `editProvider` 中 `form.Run()` 之后的死代码块：

```go
	// 若 apiFormat 为空，根据第一个模型自动推断
	if apiFormat == "" {
		apiFormat = config.DetectAPIFormat(finalModels[0])
	}
```

整块删除（共 4 行）。

- [ ] **Step 5: 调整调用顺序 — 将 `NormalizeBaseURL` 移到 `finalModels` 整理完成后**

当前代码结构（`form.Run()` 之后）：

```go
// 格式感知规范化（当前在整理 finalModels 之前）
baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)

// 整理模型列表
finalModels := []string{}
for _, m := range selectedModels {
    if m != "__custom__" {
        finalModels = append(finalModels, m)
    }
}
if ct := strings.TrimSpace(customModel); ct != "" {
    finalModels = append(finalModels, ct)
}
// 若没有任何选择，返回错误
if len(finalModels) == 0 {
    return config.ProviderConfig{}, fmt.Errorf("provider %q 的模型列表不能为空", name)
}
```

改为：

```go
// 整理模型列表
finalModels := []string{}
for _, m := range selectedModels {
    if m != "__custom__" {
        finalModels = append(finalModels, m)
    }
}
if ct := strings.TrimSpace(customModel); ct != "" {
    finalModels = append(finalModels, ct)
}
// 若没有任何选择，返回错误
if len(finalModels) == 0 {
    return config.ProviderConfig{}, fmt.Errorf("provider %q 的模型列表不能为空", name)
}

// 先确定 apiFormat，再规范化 URL（NormalizeBaseURL 依赖 apiFormat）
apiFormat := detectFormatFromModels(finalModels)
baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)
```

- [ ] **Step 6: 验证编译通过**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

预期输出：无错误，无警告。

- [ ] **Step 7: 运行全部测试**

```bash
cd /Users/yesongyun/代码/openclaw_config && go test ./...
```

预期输出：所有测试 PASS，无 FAIL。

- [ ] **Step 8: 提交**

```bash
cd /Users/yesongyun/代码/openclaw_config
git add app.go
git commit -m "feat: 移除 API 格式手动选择器，改为根据模型列表自动检测"
```

---

## 验收标准

1. `go build ./...` 通过，无编译错误
2. `go test ./...` 全部通过，`TestDetectFormatFromModels` 11 个子测试全 PASS
3. 运行 `go run .`，添加/编辑 provider 时不再出现"API 格式"选择步骤
4. 选择 `gemini-*` 模型 → URL 不被追加 `/v1`；选择 `claude-*` 或 `-cc` 结尾模型 → URL 追加 `/v1`
