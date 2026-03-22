# 快速配置 / 高级配置双模式实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 openclaw-config 增加启动时的快速配置 / 高级配置分流，其中快速配置只配置 Provider 与主模型，其余 Agent 默认继承主模型，并支持恢复进入快速模式前的高级配置。

**架构：** 在 `app.go` 的启动入口前置模式选择，快速模式走独立短流程并复用现有 Step 1 / Step 2 页面。快速模式进入时在内存中创建 agent 配置快照、清空高级配置为默认继承态，结束时通过确认页决定保留快速结果或恢复原样。

**技术栈：** Go, Charmbracelet Huh/Bubble Tea, Go test

---

## 文件结构与责任

| 文件 | 责任 |
|------|------|
| `app.go` | 新增模式选择、快速配置流程、快照恢复与确认页 |
| `app_test.go` | 覆盖快速模式状态切换、恢复逻辑、入口文案等回归测试 |
| `docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md` | 已确认的设计说明 |
| `docs/superpowers/plans/2026-03-22-dual-mode-setup.md` | 当前实现计划 |

### Task 1: 先补快速模式状态切换测试

**Files:**
- Modify: `app_test.go`

- [ ] **步骤 1：编写失败的测试**

为以下行为增加测试：

```go
func TestPrepareQuickSetupBacksUpAndClearsAdvancedConfig(t *testing.T)
func TestRestoreQuickSetupSnapshotRestoresMainSubAndNamedAgents(t *testing.T)
func TestHasAdvancedConfigDetectsSubOrNamedOverrides(t *testing.T)
```

测试目标：
- 进入快速模式时会备份 `MainAgent/SubAgent/NamedAgents`
- 快速模式初始化后 `SubAgent` 清空、`NamedAgents` 置空
- 选择恢复原样后完整恢复进入快速模式前的 agent 配置
- 仅主模型存在时不应被识别为“已有高级配置”

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./...`

预期：新测试因缺少快速模式辅助函数而失败。

### Task 2: 实现快速模式辅助逻辑与入口文案

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **步骤 1：新增最少实现代码**

在 `app.go` 中新增：

```go
type setupMode string
type quickSetupSnapshot struct { ... }

func hasAdvancedConfig(cfg *config.FullConfig) bool
func prepareQuickSetup(cfg *config.FullConfig) quickSetupSnapshot
func restoreQuickSetupSnapshot(cfg *config.FullConfig, snapshot quickSetupSnapshot)
func pickSetupMode(cfg *config.FullConfig) (setupMode, error)
```

要求：
- `prepareQuickSetup` 复制 `MainAgent/SubAgent/NamedAgents`
- 清空 `SubAgent` 与 `NamedAgents`
- `pickSetupMode` 文案明确区分快速配置与高级配置

- [ ] **步骤 2：运行测试验证通过**

运行：`go test ./...`

预期：Task 1 新增测试通过，其它现有测试保持通过。

### Task 3: 接入快速配置独立流程

**Files:**
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **步骤 1：编写失败的测试**

为快速配置确认页的说明文案增加测试：

```go
func TestQuickSetupSummaryDescriptionMentionsInheritanceAndRestore(t *testing.T)
```

要求描述中明确包含：
- 已临时备份原高级配置
- 子 Agent / 命名 Agent 当前改为默认继承主模型
- 用户可完成配置或恢复原样

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./...`

预期：因确认页描述函数不存在而失败。

- [ ] **步骤 3：编写最少实现代码**

在 `Run()` 中：
- banner 后先选择模式
- 选择高级配置时保持原 1~4 步循环
- 选择快速配置时执行独立 `runQuickSetup(...)`

在快速流程中：
- 进入时创建快照并切到默认继承态
- 复用 `runStep1Providers`
- 构建模型选项并复用 `runStep2MainAgent`
- 在确认页提供 `完成配置`、`恢复原样`、`返回上一步`
- `恢复原样` 时还原快照后结束流程

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./...`

预期：新增文案测试通过，所有现有测试继续通过。

### Task 4: 格式化、验证并准备提交

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Create: `docs/superpowers/plans/2026-03-22-dual-mode-setup.md`

- [ ] **步骤 1：格式化代码**

运行：`gofmt -w app.go app_test.go`

- [ ] **步骤 2：完整验证**

运行：`go test ./...`

预期：全部测试通过。

- [ ] **步骤 3：检查 git 变更与忽略规则**

运行：

```bash
git status --short
sed -n '1,220p' .gitignore
```

预期：本次新增和修改文件不在 `.gitignore` 中。

- [ ] **步骤 4：提交本次改动**

运行：

```bash
git add app.go app_test.go docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md docs/superpowers/plans/2026-03-22-dual-mode-setup.md
git commit -m "feat(配置向导): 增加快速配置与高级配置双模式入口"
```

预期：生成本地提交，工作区只剩用户未要求处理的改动。
