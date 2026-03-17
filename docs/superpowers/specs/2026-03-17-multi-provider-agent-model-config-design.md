# Design: 多 Provider + 多 Agent 模型配置

**日期**: 2026-03-17
**项目**: openclaw_config
**状态**: 待实现

---

## 背景

当前工具只支持单一 `dmxapi` provider + 单一模型配置。需要扩展为：
- 支持多个 API provider（每个有独立 URL + Key + 模型列表）
- 支持主 agent / 子 agent / 命名 agent 各自配置不同模型
- 兼容已有配置，检测并自动修复异常字段

---

## 目标

1. 多 provider 管理（添加、编辑）
2. 每个 provider 下注册多个模型
3. 主 agent (`agents.defaults.model`) 和子 agent (`agents.defaults.subagents.model`) 可独立选模型
4. 命名 agent (`agents.list`) 各自可指定模型
5. 启动时检测配置异常并自动修复，修复内容展示给用户

> 注：provider 删除功能不在本期范围内。

---

## 数据模型

新增 Go 结构体（`internal/config/types.go`）：

```go
// ProviderConfig 单个 API provider 的完整配置
type ProviderConfig struct {
    Name      string   // provider 唯一标识 slug，如 "dmxapi"、"my-proxy"
    BaseUrl   string
    ApiKey    string
    Models    []string // 该 provider 下注册的模型 ID 列表（不含 provider 前缀）
    ApiFormat string   // 由 DetectAPIFormat(Models[0]) 自动推断，可在 UI 中覆盖
}

// AgentModelConfig 某个 agent 的模型分配
type AgentModelConfig struct {
    Primary  string // 完整模型 ID，格式 "provider/model-id"，空表示未配置
    Fallback string // 单个 fallback，空表示不配置（UI 用单个 Input 收集）
}

// NamedAgentConfig 命名 agent 配置
type NamedAgentConfig struct {
    ID    string           // agent id，如 "my-coder"
    Model AgentModelConfig
}

// FullConfig 工具内部视图（不对应单一文件格式）
type FullConfig struct {
    Providers   []ProviderConfig
    MainAgent   AgentModelConfig   // agents.defaults.model
    SubAgent    AgentModelConfig   // agents.defaults.subagents.model；Primary 为空 = 同主 agent
    NamedAgents []NamedAgentConfig // agents.list 中本工具管理的条目
}
```

现有 `DMXAPIConfig` 和 `UpdateDMXAPIConfig` 等方法**保留不变**。

### Fallbacks 设计说明

`huh` v0.6.0 无原生动态数组控件。为保持 UI 简洁，**Fallback 仅支持单条**（`string` 而非 `[]string`），写入 JSON 时转为 `fallbacks: [fallback]`（若为空则省略该字段）。

---

## 兼容性检测与修复

新建 `internal/config/migration.go`，在 `LoadFullConfig()` 调用时执行：

| 检测项 | 触发条件 | 修复动作 |
|--------|----------|----------|
| 旧默认 URL | baseUrl == `https://www.dmxapi.cn`（无 `/v1`） | 补全为 `https://www.dmxapi.cn/v1` |
| provider key 含非法字符 | key 含空格、大写字母、特殊字符（非 `[a-z0-9-]`） | 转小写，空格/特殊字符替换为 `-`，连续 `-` 合并 |
| provider key 为空 | key == "" | 从 BaseUrl 的 host 部分生成 slug |
| models 数组为空 | provider 存在但 Models 为空 | 从 `agents.defaults.model.primary` 反推模型 ID 补全 |
| models.json 与 openclaw.json 不一致 | 任一 provider 的 baseUrl/api 字段差异 | 以 openclaw.json 为准同步修正 |

`LoadFullConfig` 将修复的所有内容收集为 `[]string`（每条一句描述）返回，**不自动写文件**，由调用方在 Step 1 展示给用户后再走 `SaveFullConfig`。

### 配置文件不存在时的行为

若 `~/.openclaw/openclaw.json` 不存在（首次使用），`LoadFullConfig` 返回 `(&FullConfig{}, nil, nil)`，允许用户从头添加 provider。

---

## 向导交互流程（4 步）

所有步骤数据在**内存**中聚合，最后一次性写入文件，中途不写。

### Step 1 — Provider 管理

1. 若有修复日志（兼容性修复），在 banner 下方用黄色 lipgloss 样式展示"已自动修正 N 处配置"及详情
2. 展示已有 provider 列表（Select），选项包括：
   - `[编辑] {name}  ({baseUrl})`（每个已有 provider 一项）
   - `[+ 添加新 Provider]`
   - `[继续 →]`（至少有 1 个 provider 时可选）
3. 进入单 provider 编辑表单：
   - `Name`：slug 输入框，实时校验 `[a-z0-9-]+`
   - `Base URL`：输入框，校验 URL 格式
   - `API Key`：输入框，非空校验
   - `Models`：MultiSelect（预设候选）+ 额外自定义输入（"自定义模型..."选项，同现有逻辑）
   - `ApiFormat`：Select（openai-completions / anthropic-messages / openai-responses / google-generative-ai），默认由 `DetectAPIFormat(Models[0])` 预填
4. 编辑完成后返回 provider 列表，可继续编辑其他 provider 或添加新的，直到选"继续 →"

### Step 2 — 主 Agent 模型

从所有已注册模型（`provider/model-id` 格式）中选择：
- `Primary`：Select，必选
- `Fallback`：Select + "（不配置）"选项，可选

### Step 3 — 子 Agent 模型

- Select 两个选项：
  - `同主 Agent（不单独配置）`
  - `单独指定`
- 若选"单独指定"，展示同 Step 2 的表单

### Step 4 — 命名 Agent（可选）

1. Confirm："是否配置命名 Agent？" → 可跳过
2. 若确认，循环执行：
   - 输入 Agent ID（slug 校验）
   - 选 Model（同 Step 2 的 Primary 选择）
   - Confirm："是否继续添加命名 Agent？" → No 时退出循环

命名 agent 的模型选项也包含"同主 Agent"。

---

## 文件改动范围

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/config/types.go` | 新增 | `ProviderConfig`、`AgentModelConfig`、`FullConfig`、`NamedAgentConfig` |
| `internal/config/migration.go` | 新建 | 兼容性检测与自动修复逻辑 |
| `internal/config/manager.go` | 扩展 | 新增 `LoadFullConfig()` / `SaveFullConfig()`，保留所有现有方法 |
| `internal/models/presets.go` | 不动 | 继续作为 Step 1 Models MultiSelect 的候选列表 |
| `app.go` | 重写 | 4 步向导替换原单表单逻辑 |
| `main.go` | 不动 | — |

---

## `manager.go` 关键新方法

### `LoadFullConfig`

```go
// LoadFullConfig 读取并解析为工具视图，执行兼容性检测（不写文件）
// fixLogs: 每条描述一处自动修复，调用方负责展示
// 若 openclaw.json 不存在，返回空 FullConfig（非错误）
func (cm *ConfigManager) LoadFullConfig() (*FullConfig, []string, error)
```

### `SaveFullConfig`

```go
// SaveFullConfig 将工具视图写回磁盘
// - 读取现有完整 JSON，保留 gateway/tools/session/commands 等无关字段
// - 全量覆写 models.providers（替换整个 providers map）
// - 覆写 agents.defaults.model 和 agents.defaults.subagents（若 SubAgent.Primary 非空）
// - 对 agents.list 做 upsert：按 ID 更新本工具管理的条目，保留其他条目不变
// - 同步写 auth-profiles.json：每个 provider 写入 "{name}:default" profile
// - 同步写 models.json：遍历所有 provider，更新各自的 baseUrl/api 字段
func (cm *ConfigManager) SaveFullConfig(cfg *FullConfig) error
```

### `UpdateModelsJson` 重构

现有方法签名不变（向后兼容），新增重载逻辑：内部遍历 models.json 中所有 provider 条目，按 name 匹配 `FullConfig.Providers` 逐一更新 `baseUrl` 和 `api` 字段。

### auth-profiles.json 写入规则

每个 `ProviderConfig` 对应 auth-profiles.json 中一条记录，profile key 为 `{provider.Name}:default`：

```json
{
  "dmxapi:default": {
    "type": "api_key",
    "provider": "dmxapi",
    "key": "sk-xxx"
  },
  "my-proxy:default": {
    "type": "api_key",
    "provider": "my-proxy",
    "key": "sk-yyy"
  }
}
```

### `agents.list` upsert 判别规则

`SaveFullConfig` 按 `NamedAgentConfig.ID` 在 `agents.list` 中做 upsert：
- 若已有条目 ID 与本工具管理的某条 `NamedAgentConfig.ID` 相同 → 仅覆写该条目的 `model` 字段，其余字段保留
- 若不存在 → 追加新条目，仅包含 `id` 和 `model` 字段
- `agents.list` 中 ID 不在本次 `FullConfig.NamedAgents` 列表内的条目 → 完全保留不动

**不存在"本工具专属标记字段"**，判别依据仅为本次提交的 `FullConfig.NamedAgents` ID 集合。

> 注意：通过本工具写入的命名 agent 条目**无法通过本工具删除**（删除功能不在本期范围）。如需删除，用户需手动编辑 `openclaw.json`。

---

## 边界场景处理

| 场景 | 处理方式 |
|------|----------|
| Step 1 中 `Models` 为空时 `DetectAPIFormat(Models[0])` | 跳过 `DetectAPIFormat` 调用，`ApiFormat` Select 预选 `openai-completions` 作为安全默认值，用户可修改；`Models` 为空时 Step 1 表单校验"至少选一个模型"，阻止提交 |
| `ProviderConfig.Models` 为空时调用 `SaveFullConfig` | 返回 error，拒绝写入（防止 UI 校验被绕过的防御层）；`ApiFormat` 始终通过 Select 控件有预选值，不会出现空字符串 |
| BaseUrl host 含端口（`host:port`）或 IP 点（`1.2.3.4`）生成 slug | 用 `url.Parse` + `Hostname()` 提取 host（自动去端口），再对 `.` 替换为 `-`，然后走 `[^a-z0-9-]` 过滤规则 |
| 同一 provider 下混合注册 anthropic + openai 模型（`ApiFormat` 语义冲突） | `ApiFormat` 是 provider 级别字段，代理聚合场景下由用户手动指定；UI 在该字段旁展示提示"不同 API 格式的模型请分拆为不同 provider" |
| Fallback 候选模型来源 | 所有 provider 的全部模型（同 Primary 候选池），允许跨 provider；openclaw 运行时自行处理跨 provider fallback 时的 ApiFormat 切换，本工具不干预 |
| `NamedAgentConfig.Model.Primary` 选"同主 Agent" | Primary 存为空字符串 `""`，`SaveFullConfig` 遇到空 Primary 时整个 `model` 字段（含 fallback）均不写入，openclaw 自动沿用 `agents.defaults.model` |
| `agents.list` 追加新条目的最小字段集 | openclaw `AgentEntrySchema` 中仅 `id` 为必填，其余全部 optional；追加条目仅写 `{id, model}` 即合法 |
| `Providers` 列表为空时调用 `SaveFullConfig` | 直接返回 error，拒绝写入，防止误清空现有配置；向导中"继续 →"在 provider 数量为 0 时禁用，正常流程不会触发此保护 |

---

## 不在本次范围内

- provider 删除功能
- provider 连通性测试
- agents.list 中非模型字段（system prompt、workspace 等）
- imageModel / pdfModel 等特殊用途模型配置
