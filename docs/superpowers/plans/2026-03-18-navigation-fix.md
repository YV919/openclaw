# 表单导航全面修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 openclaw-config TUI 的三个导航缺陷：表单内字段无法用 Shift+Tab 后退、步骤间无法返回上一步、底部键提示显示英文 "enter next"。

**Architecture:** 所有改动集中在 `app.go`，分五个原子任务：(1) 补全中文键绑定；(2-4) Step 2/3/4 函数签名改为 `(bool, error)` 并加返回选项；(5) `Run()` 改为步骤导航循环。每个任务编译通过后提交。

**Tech Stack:** Go 1.23, `charmbracelet/huh v0.6.0`

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `app.go` | 修改 | 唯一改动文件：5 处修改 |
| `app_test.go` | 不改动 | 现有测试仍然通过 |

---

## Task 1：补全 `chineseKeyMap()` 中文键绑定

**Files:**
- Modify: `app.go:292-309`（`chineseKeyMap` 函数体）

### 背景

`huh.KeyMap` 包含 Input / Confirm / Note / Form 等字段类型的键绑定，当前只设置了 Select 和 MultiSelect。未设置的字段显示英文默认提示，且 Shift+Tab 在 Input 字段内无效。

- [ ] **Step 1: 在 `chineseKeyMap()` 末尾（`return km` 之前）追加 7 行**

打开 `app.go`，定位到 `chineseKeyMap()` 函数，在 `return km` 前插入：

```go
	km.Input.Next     = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
	km.Input.Prev     = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Confirm.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Confirm.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Note.Next      = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
	km.Form.NextGroup = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "下一步"))
	km.Form.PrevGroup = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一步"))
```

改后 `chineseKeyMap()` 完整内容（供参考，确认行数正确）：

```go
func chineseKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.MultiSelect.Toggle     = key.NewBinding(key.WithKeys(" ", "x"), key.WithHelp("x", "切换选中"))
	km.MultiSelect.Up         = key.NewBinding(key.WithKeys("up", "k", "ctrl+p"), key.WithHelp("↑", "向上"))
	km.MultiSelect.Down       = key.NewBinding(key.WithKeys("down", "j", "ctrl+n"), key.WithHelp("↓", "向下"))
	km.MultiSelect.Filter     = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "过滤"))
	km.MultiSelect.Prev       = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "返回"))
	km.MultiSelect.Next       = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.MultiSelect.SelectAll  = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "全选"))
	km.MultiSelect.SelectNone = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "取消全选"), key.WithDisabled())
	km.Select.Up     = key.NewBinding(key.WithKeys("up", "k", "ctrl+k", "ctrl+p"), key.WithHelp("↑", "向上"))
	km.Select.Down   = key.NewBinding(key.WithKeys("down", "j", "ctrl+j", "ctrl+n"), key.WithHelp("↓", "向下"))
	km.Select.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "过滤"))
	km.Select.Prev   = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "返回"))
	km.Select.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Input.Next     = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
	km.Input.Prev     = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Confirm.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Confirm.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Note.Next      = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
	km.Form.NextGroup = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "下一步"))
	km.Form.PrevGroup = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一步"))
	return km
}
```

- [ ] **Step 2: 编译验证**

```bash
cd /Users/yesongyun/代码/openclaw_config
go build ./...
```

期望：无错误输出。

- [ ] **Step 3: 运行现有测试**

```bash
go test ./...
```

期望：`ok  	openclaw_config` 等，所有测试通过。

- [ ] **Step 4: 提交**

```bash
git add app.go
git commit -m "feat: 补全 chineseKeyMap 中文键绑定（Input/Confirm/Note/Form）"
```

---

## Task 2：`runStep2MainAgent` 加返回选项，签名改为 `(bool, error)`

**Files:**
- Modify: `app.go:450-488`（`runStep2MainAgent` 函数）

### 背景

当前函数签名为 `func (a *App) runStep2MainAgent(...) error`，选单无"返回"选项。需改为 `(bool, error)`，bool=true 代表用户选择了"← 返回上一步"。

- [ ] **Step 1: 修改函数签名与实现**

将 `app.go` 中 `runStep2MainAgent` 整体替换为：

```go
func (a *App) runStep2MainAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (bool, error) {
	primary := fullCfg.MainAgent.Primary
	fallback := fullCfg.MainAgent.Fallback
	if !containsOptValue(allOpts, primary) {
		primary = ""
	}
	if !containsOptValue(allOptsWithNone, fallback) {
		fallback = ""
	}
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	optsWithBack := append(allOpts, huh.NewOption("← 返回上一步", "__back__"))

	form := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("主 Agent 模型 (Primary)").
			Description("agents.defaults.model.primary").
			Options(optsWithBack...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("主 Agent 备用模型 (Fallback)").
			Description("可选，留空表示不配置备用模型").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	if primary == "__back__" {
		return true, nil
	}
	fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return false, nil
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

期望：**编译报错（预期）**——`Run()` 中调用处签名不匹配。这是正常的，Task 2-4 只改函数本身，Task 5 统一修复 `Run()` 的调用。**不要在此步骤修复这些错误，继续 Task 3。**

- [ ] **Step 3: 提交（即使当前不可编译）**

```bash
git add app.go
git commit -m "feat: runStep2MainAgent 改为 (bool, error)，加返回上一步选项"
```

---

## Task 3：`runStep3SubAgent` 加返回选项，签名改为 `(bool, error)`

**Files:**
- Modify: `app.go:492-557`（`runStep3SubAgent` 函数）

- [ ] **Step 1: 修改函数签名与实现**

将 `app.go` 中 `runStep3SubAgent` 整体替换为：

```go
func (a *App) runStep3SubAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (bool, error) {
	const sameAsMain = "__same__"
	subChoice := sameAsMain
	if fullCfg.SubAgent.Primary != "" {
		subChoice = "__custom__"
	}

	form1 := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", "__custom__"),
				huh.NewOption("← 返回上一步", "__back__"),
			).
			Value(&subChoice),
	))
	if err := form1.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	if subChoice == "__back__" {
		return true, nil
	}
	if subChoice == sameAsMain {
		fullCfg.SubAgent = config.AgentModelConfig{}
		return false, nil
	}

	// 单独指定
	primary := fullCfg.SubAgent.Primary
	fallback := fullCfg.SubAgent.Fallback
	if !containsOptValue(allOpts, primary) {
		primary = ""
	}
	if !containsOptValue(allOptsWithNone, fallback) {
		fallback = ""
	}
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	form2 := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 主模型 (Primary)").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("子 Agent 备用模型 (Fallback)").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form2.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	fullCfg.SubAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return false, nil
}
```

- [ ] **Step 2: 编译（同 Task 2，预期 `Run()` 中调用仍报错，不要修复，继续 Task 4）**

```bash
go build ./...
```

- [ ] **Step 3: 提交**

```bash
git add app.go
git commit -m "feat: runStep3SubAgent 改为 (bool, error)，加返回上一步选项"
```

---

## Task 4：`runStep4NamedAgents` 和 `pickNamedAgentAction` 加返回选项

**Files:**
- Modify: `app.go:560-588`（`pickNamedAgentAction`）
- Modify: `app.go:650-704`（`runStep4NamedAgents`）

- [ ] **Step 1: 修改 `pickNamedAgentAction`**

将 `pickNamedAgentAction` 中构建 opts 的部分修改，在 `[继续 →]` 前插入返回选项：

```go
func pickNamedAgentAction(agents []config.NamedAgentConfig) (string, error) {
	var opts []huh.Option[string]
	for _, na := range agents {
		modelLabel := na.Model.Primary
		if modelLabel == "" {
			modelLabel = "同主 Agent"
		}
		label := fmt.Sprintf("%s  (%s)", na.ID, modelLabel)
		opts = append(opts, huh.NewOption(label, na.ID))
	}
	opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
	opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))

	var selected string
	form := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("命名 Agent 管理").
			Description("为特定 agent id 指定不同模型").
			Options(opts...).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}
```

- [ ] **Step 2: 修改 `runStep4NamedAgents` 签名和 switch**

```go
func (a *App) runStep4NamedAgents(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
) (bool, error) {
	const sameAsMain = ""
	allOptsWithSame := append(
		[]huh.Option[string]{huh.NewOption("同主 Agent", sameAsMain)},
		allOpts...,
	)
	allOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allOpts...,
	)

	for {
		action, err := pickNamedAgentAction(fullCfg.NamedAgents)
		if err != nil {
			return false, err
		}

		switch action {
		case "__back__":
			return true, nil
		case "__continue__":
			return false, nil
		default:
			// 选中已有 Agent → 二级菜单
			subAction, err := pickNamedAgentItemAction(action)
			if err != nil {
				return false, err
			}
			switch subAction {
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
			case "__delete__":
				newAgents := make([]config.NamedAgentConfig, 0, len(fullCfg.NamedAgents))
				for _, na := range fullCfg.NamedAgents {
					if na.ID != action {
						newAgents = append(newAgents, na)
					}
				}
				fullCfg.NamedAgents = newAgents
			}
			// "__back__" 继续外层 for
		}
	}
}
```

- [ ] **Step 3: 编译（预期 `Run()` 中调用仍报错）**

```bash
go build ./...
```

- [ ] **Step 4: 提交**

```bash
git add app.go
git commit -m "feat: runStep4NamedAgents 改为 (bool, error)，加返回上一步选项"
```

---

## Task 5：`Run()` 改为步骤导航循环（修复编译 + 完成核心逻辑）

**Files:**
- Modify: `app.go:57-91`（`Run()` 函数体中 Step 1-4 调用部分）

### 背景

前四个任务已完成函数改造，此任务将 `Run()` 中的顺序调用替换为步骤循环，同时修复编译错误。

- [ ] **Step 1: 替换 `Run()` 中 Step 1-4 的调用部分**

找到 `Run()` 中如下代码段（大约在 `printBanner()` 调用之后）：

```go
	// Step 1: Provider 管理
	if err := a.runStep1Providers(fullCfg); err != nil {
		return err
	}

	// 构建全部模型选项（供后续步骤使用）
	allModelOpts := buildAllModelOpts(fullCfg.Providers)
	allModelOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allModelOpts...,
	)

	// Step 2: 主 Agent 模型
	if err := a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 3: 子 Agent 模型
	if err := a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 4: 命名 Agent（可选）
	if err := a.runStep4NamedAgents(fullCfg, allModelOpts); err != nil {
		return err
	}
```

替换为：

```go
	// Step 1-4: 步骤导航循环（支持后退）
	var allModelOpts []huh.Option[string]
	var allModelOptsWithNone []huh.Option[string]

	step := 1
	for step >= 1 && step <= 4 {
		// 每次进入 Step 2 前重建模型选项（Step 1 可能修改了 Providers）
		if step == 2 {
			allModelOpts = buildAllModelOpts(fullCfg.Providers)
			allModelOptsWithNone = append(
				[]huh.Option[string]{huh.NewOption("（不配置）", "")},
				allModelOpts...,
			)
		}

		var back bool
		var err error
		switch step {
		case 1:
			err = a.runStep1Providers(fullCfg)
		case 2:
			back, err = a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone)
		case 3:
			back, err = a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone)
		case 4:
			back, err = a.runStep4NamedAgents(fullCfg, allModelOpts)
		}
		if err != nil {
			return err
		}
		if back {
			step--
		} else {
			step++
		}
	}
```

- [ ] **Step 2: 编译验证（此时应无错误）**

```bash
go build ./...
```

期望：无任何错误。

- [ ] **Step 3: 运行现有测试**

```bash
go test ./...
```

期望：所有测试通过（`TestDetectFormatFromModels` 及其他）。

- [ ] **Step 4: 手动冒烟测试**

```bash
go run .
```

验证以下行为：

| 场景 | 预期 |
|------|------|
| 进入 editProvider，在 API Key 输入框按 Shift+Tab | 跳回 Base URL 输入框 |
| Step 2 选单底部有 `← 返回上一步` | ✓ 出现该选项 |
| Step 2 选单底部键提示 | 显示 `enter 确认`（Select 字段提示），整体表单底部若有多 Group 时显示 `enter 下一步` |
| Step 2 选 `← 返回上一步` | 回到 Step 1 Provider 管理屏 |
| Step 3 选 `← 返回上一步` | 回到 Step 2 主 Agent 模型选择 |
| Step 4 选 `← 返回上一步` | 回到 Step 3 子 Agent 选择 |
| Ctrl+C 在任意步骤 | 直接退出，打印"已取消" |
| 正常完成 4 步后保存 | 打印成功框，显示已配置的 Provider 和模型 |

- [ ] **Step 5: 提交**

```bash
git add app.go
git commit -m "feat: Run() 改为步骤导航循环，支持 Step 2-4 返回上一步"
```

---

## 完成后验证

```bash
go test ./...
go build ./...
```

两者均无错误即完成。
