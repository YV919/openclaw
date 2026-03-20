# 设计文档：Release 发布流程补齐独立更新日志

**日期**：2026-03-21
**状态**：最终稿

---

## 背景

当前仓库的 CNB 发布流程已经具备以下能力：

1. 主分支推送时同步到 GitHub
2. Tag 推送时交叉编译 5 个平台二进制
3. Tag 推送时生成基础 `RELEASE_NOTES`
4. Tag 推送时创建 CNB Release 并上传附件

但现有流程仍有两个缺口：

1. 仓库内没有持续累积的独立更新日志文件，用户无法直接在代码仓库里查看历史版本说明
2. Release 描述使用的是原始 `git-cliff` 输出，内容更偏向 commit 摘要，不够面向普通用户

---

## 目标

1. 在仓库根目录新增标准命名的 [`CHANGELOG.md`](/Users/yesongyun/代码/openclaw_config/CHANGELOG.md)
2. 在 Tag 发布流程中自动生成最新版本更新日志，并将其插入 `CHANGELOG.md` 顶部
3. 使用 AI 对 `git-cliff` 生成的最新版本日志进行中文润色
4. 将同一份润色后的日志同时用于：
   - `CHANGELOG.md`
   - CNB Release 描述
5. 当 AI 调用失败或 changelog 回写失败时，不阻塞本次 Release 产物发布

---

## 不做的事

- 不改动主分支推送到 GitHub 的现有同步逻辑
- 不新增 GitHub Actions 或其他外部 CI 系统逻辑
- 不为历史 tag 批量补写旧版本 changelog
- 不引入额外的 changelog 生成器，继续沿用现有 `git-cliff`
- 不增加 PR 评审工作流，本次只聚焦 release 发布链路

---

## 设计概览

### 1. 独立更新日志文件

新增仓库根目录 [`CHANGELOG.md`](/Users/yesongyun/代码/openclaw_config/CHANGELOG.md)，作为标准的版本变更历史文件。

文件结构保持简单：

- 顶部标题 `# 更新日志`
- 简短说明，说明文件由发布流程自动维护
- 后续每次 tag 发布时，插入一个新的 `## <版本> - <日期>` 小节

这样既能保证仓库内有稳定入口，也能让自动脚本容易定位第一个版本标题并插入新内容。

### 2. `git-cliff` 模板补齐版本号和日期

现有 [`cliff.toml`](/Users/yesongyun/代码/openclaw_config/cliff.toml) 只输出分组后的 commit 列表，没有显式版本号和日期标题。  
这会导致后续 AI 润色阶段无法“保留版本号和日期”，因为输入中根本没有这部分信息。

因此需要修改 `cliff.toml` 的 `body` 模板，使 `git-cliff --latest --strip header` 输出包含：

```md
## vX.Y.Z - YYYY-MM-DD
### ✨ 新功能
- ...
```

这样生成的原始日志就同时适合：

- 直接作为 AI 输入
- AI 失败时的回退输出
- 写入 `CHANGELOG.md`
- 展示在 Release 描述中

### 3. Tag 发布流程重排

新的 Tag 发布阶段顺序为：

1. 同步 tag 到 GitHub
2. 生成原始 changelog
3. AI 润色 changelog
4. 更新 `CHANGELOG.md` 并推回 CNB `main`
5. 构建 5 平台二进制
6. 创建 Release
7. 上传附件
8. 清理旧版本

这样做的原因：

- 先产生日志，后续所有步骤都可复用
- `CHANGELOG.md` 更新独立于二进制构建，失败不会污染构建结果
- Release 描述始终优先使用润色后的说明

### 4. AI 润色策略

沿用你提供的参考方案思路：

- 把 `RELEASE_NOTES` 作为原始输入
- 使用导入环境中的 AI 变量（如 `AI_API_URL`、`AI_API_KEY`、`AI_MODEL`）
- 要求 AI 输出面向普通用户的中文说明
- 限制输出必须以 `## ` 开头
- 增加一次失败重试
- 若 AI 调用失败或格式不合法，则回退到原始 `RELEASE_NOTES`

这样能在保证可读性的同时，避免外部 API 故障导致发布中断。

### 5. `CHANGELOG.md` 回写策略

发布环境通常运行在 detached HEAD 上，因此更新 `CHANGELOG.md` 需要显式处理分支与推送。

流程设计如下：

1. 配置 git 用户为 CNB Bot
2. `git fetch origin main`
3. 基于 `origin/main` 创建临时分支
4. 在 `CHANGELOG.md` 中定位第一个 `## ` 标题
5. 将新日志插入到该标题之前
6. 仅提交 `CHANGELOG.md`
7. 使用带鉴权的 CNB 仓库地址推回 `main`

如果推送失败（例如并发 tag 导致 `main` 发生变化），则打印告警并 `exit 0` 跳过本次 changelog 回写，不影响后续 Release。

### 6. Release 描述

Release 描述继续保留当前的下载表格、平台使用说明和项目说明，但将正文中的 `${RELEASE_NOTES}` 改为 `${AI_RELEASE_NOTES}`。

这样用户在 Release 页面看到的是更友好的版本说明，而不是原始 commit 汇总。

---

## 数据流

```text
git-cliff --latest
  -> RELEASE_NOTES
  -> AI polish
       -> AI_RELEASE_NOTES
           -> update CHANGELOG.md
           -> release description
```

失败回退流：

```text
AI 调用失败 / 输出格式不合法
  -> 使用 RELEASE_NOTES 作为 AI_RELEASE_NOTES

CHANGELOG.md 推送失败
  -> 跳过仓库回写
  -> 继续 build / release / attachments
```

---

## 边界情况

| 场景 | 处理方式 |
|------|----------|
| `CHANGELOG.md` 尚不存在 | 仓库内预先新增该文件，避免发布时首次生成逻辑过于复杂 |
| `CHANGELOG.md` 里尚无 `## ` 标题 | 新日志直接追加到文件末尾 |
| AI API 返回非 200 | 进行 1 次重试，仍失败则回退原始日志 |
| AI 返回空内容或不以 `## ` 开头 | 视为格式校验失败，回退原始日志 |
| `git push` 回写 `main` 失败 | 输出告警并跳过，不中断发布 |
| tag 发布环境为浅克隆 | 保留 `git fetch --unshallow origin || true` 和 `git fetch --tags origin` |

---

## 改动文件清单

| 文件 | 改动内容 |
|------|----------|
| `.cnb.yml` | 重排 tag 发布流程，新增 AI 润色和 `CHANGELOG.md` 回写阶段，Release 描述改用 `AI_RELEASE_NOTES` |
| `cliff.toml` | 让最新日志输出包含版本号和日期标题 |
| `CHANGELOG.md` | 新增标准更新日志文件，作为自动维护入口 |

---

## 验证策略

1. 本地校验 YAML 与模板改动没有明显语法问题
2. 使用本地 `git-cliff` 配置思路人工检查输出结构是否满足 `## 版本 - 日期`
3. 检查 `.cnb.yml` 中 stage 顺序是否与设计一致
4. 检查 `CHANGELOG.md` 初始结构是否能被插入逻辑识别
5. 最后检查 git 仓库与 `.gitignore`，把新增和修改文件提交到本地 git
