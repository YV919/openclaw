# 表单导航修复 v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 openclaw-config TUI 三处残留缺陷：末字段帮助栏仍显示英文（Submit 未覆盖）、editProvider/editNamedAgent 按 Ctrl+C 退出整个程序而非返回上级菜单。

**Architecture:** 仅修改 `app.go`，三个原子任务：(1) 在 chineseKeyMap() 追加 5 个 Submit 中文绑定；(2) editProvider 签名改为 (bool, error) 风格，支持取消返回；(3) editNamedAgent 同理。

**Tech Stack:** Go 1.23, `charmbracelet/huh v0.6.0`

---

## 背景知识（实现前必读）

huh v0.6.0 对每种字段类型区分两个不同 binding：
- `Next`：非末字段时启用，按 Enter 跳到下一字段
- `Submit`：末字段时启用，按 Enter 提交表单

`chineseKeyMap()` 已覆盖所有字段类型的 `Next`/`Prev`，但**未覆盖 `Submit`**，导致末字段帮助栏退回英文默认值。

huh v0.6.0 的 `KeyMap` 结构体**没有 `Form` 子结构**，不存在 `km.Form.NextGroup` 等字段，勿尝试设置。

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `app.go` | 修改 | 唯一改动文件：3 处修改 |
| `app_test.go` | 不改动 | 现有测试仍然通过 |

---

## Task 1：`chineseKeyMap()` 追加 Submit 中文绑定

**Files:**
- Modify: `app.go` — `chineseKeyMap()` 函数，`return km` 之前

### 背景

当前 `chineseKeyMap()` 末尾（app.go 第 318-324 行附近）已有：

```go
km.Input.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
km.Input.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Confirm.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
km.Note.Next    = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
km.Note.Prev    = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "返回"))
return km
```

需要在 `return km` 前插入 5 行 Submit 绑定。

- [ ] **Step 1: 在 `return km` 前插入 5 行**

在 `km.Note.Prev = ...` 行之后、`return km` 之前，插入：

```go
	km.Input.Submit       = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "提交"))
	km.Select.Submit      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.MultiSelect.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.Note.Submit        = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "继续"))
	km.Confirm.Submit     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
```

改后 `chineseKeyMap()` 函数末尾应如下（确认完整性）：

```go
	km.Input.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
	km.Input.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Note.Next    = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
	km.Note.Prev    = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "返回"))
	km.Input.Submit       = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "提交"))
	km.Select.Submit      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.MultiSelect.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.Note.Submit        = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "继续"))
	km.Confirm.Submit     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	return km
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/yesongyun/代码/openclaw_config && go build ./...
```

期望：无错误输出。

- [ ] **Step 3: 运行现有测试**

```bash
go test ./...
```

期望：所有测试通过。

- [ ] **Step 4: 提交**

```bash
git add app.go
git commit -m "feat: chineseKeyMap 追加各字段 Submit 中文绑定，修复末字段帮助栏英文问题"
```

---

## Task 2：`editProvider` 支持取消返回上级菜单

**Files:**
- Modify: `app.go` — `editProvider` 函数签名与内部错误处理
- Modify: `app.go` — `runStep1Providers` 两处调用点

### 背景

当前 `editProvider` 在 `huh.ErrUserAborted`（Ctrl+C）时调用 `os.Exit(0)`，导致整个程序退出。目标是改为返回 `cancelled=true`，让调用方（`runStep1Providers`）跳过保存并继续循环，回到 Provider 列表选单。

同时在 "Provider 标识名" 字段的 Description 追加操作提示。

- [ ] **Step 1: 修改 `editProvider` 函数签名**

将：
```go
func editProvider(p config.ProviderConfig) (config.ProviderConfig, error) {
```
改为：
```go
func editProvider(p config.ProviderConfig) (config.ProviderConfig, bool, error) {
```

- [ ] **Step 2: 修改 `editProvider` 内部的 `huh.ErrUserAborted` 处理**

找到：
```go
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.ProviderConfig{}, err
	}
```

改为：
```go
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return config.ProviderConfig{}, true, nil
		}
		return config.ProviderConfig{}, false, err
	}
```

- [ ] **Step 3: 修改 `editProvider` 末尾的 return 语句**

找到函数末尾的 return（返回 ProviderConfig 结果的那行）：
```go
	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		...
	}, nil
```

改为（追加 `false,`）：
```go
	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		BaseUrl:   strings.TrimSpace(baseUrl),
		ApiKey:    strings.TrimSpace(apiKey),
		Models:    finalModels,
		ApiFormat: apiFormat,
	}, false, nil
```

- [ ] **Step 4: 修改 "Provider 标识名" 字段的 Description**

在 `editProvider` 中找到：
```go
		Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）").
```

改为：
```go
		Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）\n· shift+tab 返回上一字段  · Ctrl+C 取消编辑").
```

- [ ] **Step 5: 修改 `runStep1Providers` 中 `__add__` 分支**

找到：
```go
		if action == "__add__" {
			p, err := editProvider(config.ProviderConfig{})
			if err != nil {
				return err
			}
			fullCfg.Providers = append(fullCfg.Providers, p)
```

改为：
```go
		if action == "__add__" {
			p, cancelled, err := editProvider(config.ProviderConfig{})
			if err != nil {
				return err
			}
			if !cancelled {
				fullCfg.Providers = append(fullCfg.Providers, p)
			}
```

- [ ] **Step 6: 修改 `runStep1Providers` 中 `__edit__` 分支**

找到：
```go
			case "__edit__":
				for i, p := range fullCfg.Providers {
					if p.Name == action {
						updated, err := editProvider(p)
						if err != nil {
							return err
						}
						fullCfg.Providers[i] = updated
						break
					}
				}
```

改为：
```go
			case "__edit__":
				for i, p := range fullCfg.Providers {
					if p.Name == action {
						updated, cancelled, err := editProvider(p)
						if err != nil {
							return err
						}
						if !cancelled {
							fullCfg.Providers[i] = updated
						}
						break
					}
				}
```

- [ ] **Step 7: 编译验证**

```bash
go build ./...
```

期望：无错误。

- [ ] **Step 8: 运行现有测试**

```bash
go test ./...
```

期望：所有测试通过。

- [ ] **Step 9: 提交**

```bash
git add app.go
git commit -m "feat: editProvider 支持 Ctrl+C 取消返回上级菜单，追加操作提示"
```

---

## Task 3：`editNamedAgent` 支持取消返回上级菜单

**Files:**
- Modify: `app.go` — `editNamedAgent` 函数签名与内部错误处理
- Modify: `app.go` — `runStep4NamedAgents` 中 `__edit__` 分支

### 背景

与 Task 2 场景相同，`editNamedAgent` 也有相同的 `os.Exit(0)` 问题。

- [ ] **Step 1: 修改 `editNamedAgent` 函数签名**

将：
```go
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, error) {
```

改为：
```go
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, bool, error) {
```

- [ ] **Step 2: 修改 `editNamedAgent` 内部的 `huh.ErrUserAborted` 处理**

找到：
```go
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.NamedAgentConfig{}, err
	}
```

改为：
```go
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return config.NamedAgentConfig{}, true, nil
		}
		return config.NamedAgentConfig{}, false, err
	}
```

- [ ] **Step 3: 修改 `editNamedAgent` 末尾的 return 语句**

找到：
```go
	return config.NamedAgentConfig{
		ID:    agent.ID,
		Model: config.AgentModelConfig{Primary: primary, Fallback: fallback},
	}, nil
```

改为：
```go
	return config.NamedAgentConfig{
		ID:    agent.ID,
		Model: config.AgentModelConfig{Primary: primary, Fallback: fallback},
	}, false, nil
```

- [ ] **Step 4: 修改 `runStep4NamedAgents` 中 `__edit__` 分支**

找到：
```go
			case "__edit__":
				for i, na := range fullCfg.NamedAgents {
					if na.ID == action {
						updated, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return false, err
						}
						fullCfg.NamedAgents[i] = updated
						break
					}
				}
```

改为：
```go
			case "__edit__":
				for i, na := range fullCfg.NamedAgents {
					if na.ID == action {
						updated, cancelled, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return false, err
						}
						if !cancelled {
							fullCfg.NamedAgents[i] = updated
						}
						break
					}
				}
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

期望：无错误。

- [ ] **Step 6: 运行现有测试**

```bash
go test ./...
```

期望：所有测试通过。

- [ ] **Step 7: 手动冒烟测试**

```bash
go run .
```

验证以下行为（按顺序测试）：

| 场景 | 预期 |
|------|------|
| 进入 editProvider，查看底部帮助栏 | 非末字段显示 `enter 下一项`，末字段（自定义模型名称）显示 `enter 提交` |
| editProvider 第一个字段 Description | 显示 `· shift+tab 返回上一字段  · Ctrl+C 取消编辑` |
| editProvider 中按 Shift+Tab | 跳回上一字段 |
| editProvider 中按 Ctrl+C | 回到 Provider 列表（不退出程序） |
| 编辑命名 Agent 时按 Ctrl+C | 回到命名 Agent 列表（不退出程序） |
| Ctrl+C 在 Provider 列表选单 | 直接退出程序（保持现有行为） |
| Step 2 选单底部帮助栏 | 显示 `enter 确认`（Select.Next） |
| 正常完成所有步骤 | 保存并显示成功框 |

- [ ] **Step 8: 提交**

```bash
git add app.go
git commit -m "feat: editNamedAgent 支持 Ctrl+C 取消返回上级菜单"
```

---

## 完成后验证

```bash
go test ./...
go build ./...
```

两者均无错误即完成。
