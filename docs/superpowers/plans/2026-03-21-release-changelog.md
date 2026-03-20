# Release 发布流程补齐独立更新日志 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 openclaw_config 的 CNB Tag 发布流程新增独立 `CHANGELOG.md` 自动回写，并把 AI 润色后的版本说明同时用于仓库 changelog 和 CNB Release 描述。

**Architecture:** 保持 `git-cliff` 作为唯一日志来源，先生成原始 `RELEASE_NOTES`，再生成 `AI_RELEASE_NOTES`。发布流水线在构建前完成 changelog 润色与回写，回写失败时不阻塞后续二进制构建与 Release 创建。

**Tech Stack:** CNB `.cnb.yml`, `git-cliff`, shell script, Markdown

---

## 文件结构与责任

| 文件 | 责任 |
|------|------|
| `.cnb.yml` | 定义 CNB Tag 发布顺序、变量导出、AI 润色和 `CHANGELOG.md` 回写 |
| `cliff.toml` | 定义最新版本 changelog 的 Markdown 模板 |
| `CHANGELOG.md` | 仓库内持久化的版本历史文件 |

---

### Task 1: 补齐 `CHANGELOG.md` 模板基础

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 1: 新建 `CHANGELOG.md`**

写入标准顶部结构：

```md
# 更新日志

本文件由 CNB Release 流程自动维护，按版本倒序记录面向用户的更新说明。
```

- [ ] **Step 2: 检查文件命名和位置**

确认文件路径为仓库根目录 `CHANGELOG.md`，不要使用小写文件名，也不要放到 `docs/`。

- [ ] **Step 3: 运行 git 状态检查**

Run: `git status --short`

Expected: 出现新文件 `CHANGELOG.md`

---

### Task 2: 让 `git-cliff` 输出可直接写入 changelog 的版本块

**Files:**
- Modify: `cliff.toml`

- [ ] **Step 1: 修改 `body` 模板**

把当前仅分组输出 commit 的模板，改为带版本号和日期的结构：

```toml
body = """
{% if version %}
## {{ version }} - {{ timestamp | date(format="%Y-%m-%d") }}
{% else %}
## Unreleased
{% endif %}
{% for group, commits in commits | group_by(attribute="group") %}
### {{ group | upper_first }}
{% for commit in commits %}
- {{ commit.message | upper_first }}
{%- if commit.body %}
{{ commit.body | indent(prefix="  ") }}
{%- endif %}
{% endfor %}
{% endfor %}
"""
```

保留现有 commit 分组规则，不改 `commit_parsers`。

- [ ] **Step 2: 人工检查 AI 输入前提**

确认生成结果第一行会是 `## ` 开头，并自带版本号和日期，从而满足 AI prompt 中“保留版本号和日期”的要求。

- [ ] **Step 3: 运行差异检查**

Run: `git diff -- cliff.toml`

Expected: 只出现模板层面的改动，没有 parser 规则的意外变化。

---

### Task 3: 扩展 `.cnb.yml` 的 Tag 发布流程

**Files:**
- Modify: `.cnb.yml`

- [ ] **Step 1: 保留现有 tag 同步阶段**

继续保留 `sync tag to github` 阶段，不改 GitHub 同步目标。

- [ ] **Step 2: 调整 changelog 生成阶段**

保留现有 `git fetch --unshallow origin || true`、`git fetch --tags origin` 和 `git-cliff --latest --strip header`，继续导出：

```yaml
exports:
  stdout: RELEASE_NOTES
```

- [ ] **Step 3: 新增 AI 润色阶段**

在 changelog 生成后、构建前新增一个 stage：

```yaml
- name: polish changelog with ai
```

要求：
- 通过临时 prompt 文件拼接原始 `RELEASE_NOTES`
- 调用 `${AI_API_URL}/v1/messages`
- 带 1 次失败重试
- 校验输出必须以 `## ` 开头
- 最终导出 `AI_RELEASE_NOTES`

- [ ] **Step 4: 新增 `CHANGELOG.md` 回写阶段**

在 AI 润色后新增一个 stage：

```yaml
- name: update CHANGELOG.md
```

要求：
- 若 `AI_RELEASE_NOTES` 为空则跳过
- `git fetch origin main`
- `git checkout -b temp-changelog origin/main`
- 定位 `CHANGELOG.md` 中首个 `## ` 行
- 将新日志插入在旧版本块之前
- `git add CHANGELOG.md`
- `git commit -m "docs: update CHANGELOG.md for ${CNB_BRANCH}"`
- 将 `origin` 设置为 CNB 仓库鉴权地址后推送到 `main`
- 推送失败时告警并 `exit 0`

- [ ] **Step 5: 保持构建、Release、附件上传和清理阶段**

保留现有 5 平台构建逻辑与产物命名，确保附件仍来自 `./dist/openclaw-config-*`。

- [ ] **Step 6: 更新 Release 描述**

将 Release 描述正文中的：

```yaml
${RELEASE_NOTES}
```

替换为：

```yaml
${AI_RELEASE_NOTES}
```

其他下载表格和使用说明保持现有风格。

- [ ] **Step 7: 人工检查 YAML 结构**

Run: `sed -n '1,260p' .cnb.yml`

Expected:
- stage 顺序变为“同步 -> 原始 changelog -> AI -> 回写 CHANGELOG -> build -> release -> attachments -> cleanup”
- YAML 缩进一致
- `exports` 变量名没有拼写错误

---

### Task 4: 验证并准备提交

**Files:**
- Modify: `.cnb.yml`
- Modify: `cliff.toml`
- Create: `CHANGELOG.md`
- Create: `docs/superpowers/specs/2026-03-21-release-changelog-design.md`
- Create: `docs/superpowers/plans/2026-03-21-release-changelog.md`

- [ ] **Step 1: 检查 git 变更**

Run: `git status --short`

Expected: 只包含本次新增和修改文件。

- [ ] **Step 2: 检查 `.gitignore`**

Run: `sed -n '1,220p' .gitignore`

Expected: 本次文件都不在忽略规则内。

- [ ] **Step 3: 提交本次改动**

Run:

```bash
git add .cnb.yml cliff.toml CHANGELOG.md docs/superpowers/specs/2026-03-21-release-changelog-design.md docs/superpowers/plans/2026-03-21-release-changelog.md
git commit -m "ci: automate release changelog publishing"
```

Expected: 生成本地提交，工作区干净。
