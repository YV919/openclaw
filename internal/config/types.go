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
