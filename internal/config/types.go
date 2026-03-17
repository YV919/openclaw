package config

// DMXAPIConfig 简化的 DMXAPI 配置结构
type DMXAPIConfig struct {
	BaseUrl      string `json:"baseUrl"`
	ApiKey       string `json:"apiKey"`
	CurrentModel string `json:"currentModel"`
}

// ModelDefinition 模型定义结构
type ModelDefinition struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Reasoning     bool     `json:"reasoning"`
	Input         []string `json:"input"`
	Cost          Cost     `json:"cost"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
}

// Cost 模型成本结构
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ProviderConfig 单个 API provider 的完整配置
type ProviderConfig struct {
	Name      string   // provider 唯一标识 slug，如 "dmxapi"、"my-proxy"
	BaseUrl   string
	ApiKey    string
	Models    []string // 该 provider 下注册的模型 ID 列表（不含 provider 前缀）
	ApiFormat string   // openai-completions / anthropic-messages / openai-responses / google-generative-ai
}

// AgentModelConfig 某个 agent 的模型分配
type AgentModelConfig struct {
	Primary  string // 完整模型 ID，格式 "provider/model-id"；空表示未配置（沿用默认）
	Fallback string // 单个 fallback；空表示不配置
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
	SubAgent    AgentModelConfig   // agents.defaults.subagents.model；Primary 空 = 同主 agent
	NamedAgents []NamedAgentConfig // agents.list 中本工具管理的条目
}
