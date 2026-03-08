package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	OpenClawDir      = ".openclaw"
	ConfigFile       = "openclaw.json"
	AuthProfilesDir  = "agents/main/agent"
	AuthProfilesFile = "auth-profiles.json"
	DefaultBaseUrl   = "https://www.dmxapi.cn/v1"
	ProviderName     = "dmxapi"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	homeDir string
}

// NewConfigManager 创建配置管理器
func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return &ConfigManager{homeDir: homeDir}, nil
}

// GetConfigPath 获取主配置文件路径
func (cm *ConfigManager) GetConfigPath() string {
	return filepath.Join(cm.homeDir, OpenClawDir, ConfigFile)
}

// GetAuthProfilesPath 获取 auth-profiles.json 路径
func (cm *ConfigManager) GetAuthProfilesPath() string {
	return filepath.Join(cm.homeDir, OpenClawDir, AuthProfilesDir, AuthProfilesFile)
}

// LoadConfig 读取主配置文件
func (cm *ConfigManager) LoadConfig() (map[string]interface{}, error) {
	configPath := cm.GetConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在，请先运行 openclaw onboard 初始化")
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return config, nil
}

// LoadAuthProfiles 读取 auth-profiles.json
func (cm *ConfigManager) LoadAuthProfiles() (map[string]interface{}, error) {
	authPath := cm.GetAuthProfilesPath()
	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("auth-profiles.json 不存在")
		}
		return nil, fmt.Errorf("读取 auth-profiles.json 失败: %w", err)
	}

	var authProfiles map[string]interface{}
	if err := json.Unmarshal(data, &authProfiles); err != nil {
		return nil, fmt.Errorf("解析 auth-profiles.json 失败: %w", err)
	}

	return authProfiles, nil
}

// SaveConfig 保存主配置文件（带备份）
func (cm *ConfigManager) SaveConfig(config map[string]interface{}) error {
	configPath := cm.GetConfigPath()

	// 备份原配置（写入前先清理旧备份，保留最新 5 个）
	if _, err := os.Stat(configPath); err == nil {
		cm.cleanOldBackups(configPath, 5)
		backupPath := configPath + ".backup." + time.Now().Format("20060102150405")
		data, _ := os.ReadFile(configPath)
		os.WriteFile(backupPath, data, 0600)
	}

	// 保存新配置
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// SaveAuthProfiles 保存 auth-profiles.json
func (cm *ConfigManager) SaveAuthProfiles(authProfiles map[string]interface{}) error {
	authPath := cm.GetAuthProfilesPath()

	// 确保目录存在
	dir := filepath.Dir(authPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	data, err := json.MarshalIndent(authProfiles, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 auth-profiles 失败: %w", err)
	}

	if err := os.WriteFile(authPath, data, 0600); err != nil {
		return fmt.Errorf("写入 auth-profiles.json 失败: %w", err)
	}

	return nil
}

// GetApiKey 获取 API Key
func (cm *ConfigManager) GetApiKey() (string, error) {
	authProfiles, err := cm.LoadAuthProfiles()
	if err != nil {
		return "", err
	}

	profiles, ok := authProfiles["profiles"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("profiles 字段格式错误")
	}

	profileKey := ProviderName + ":default"
	dmxapiProfile, ok := profiles[profileKey].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("未找到 %s 配置", profileKey)
	}

	key, ok := dmxapiProfile["key"].(string)
	if !ok {
		return "", fmt.Errorf("API Key 格式错误")
	}

	return key, nil
}

// SaveApiKey 保存 API Key
func (cm *ConfigManager) SaveApiKey(key string) error {
	authProfiles, err := cm.LoadAuthProfiles()
	if err != nil {
		// 如果文件不存在，创建新的
		authProfiles = map[string]interface{}{
			"version":  1,
			"profiles": map[string]interface{}{},
		}
	}

	profiles, ok := authProfiles["profiles"].(map[string]interface{})
	if !ok {
		profiles = map[string]interface{}{}
		authProfiles["profiles"] = profiles
	}

	profileKey := ProviderName + ":default"
	profiles[profileKey] = map[string]interface{}{
		"type":     "api_key",
		"provider": ProviderName,
		"key":      key,
	}

	return cm.SaveAuthProfiles(authProfiles)
}

// UpdateModelsJson 直接更新 models.json 中的 dmxapi provider baseUrl 和 api 字段。
// 必要原因：openclaw 的 ensureOpenClawModelsJson 合并逻辑会保留 models.json 中的旧 baseUrl，
// 即使 openclaw.json 已更新、网关已重启，models.json 中的旧值也会被保留。
func (cm *ConfigManager) UpdateModelsJson(baseUrl, apiFormat string) error {
	modelsPath := filepath.Join(cm.homeDir, OpenClawDir, AuthProfilesDir, "models.json")

	data, err := os.ReadFile(modelsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			// 文件存在但读取失败，输出警告避免静默失效
			fmt.Fprintf(os.Stderr, "警告: 无法读取 models.json: %v\n", err)
		}
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 无法解析 models.json: %v\n", err)
		return nil
	}

	providers, ok := raw["providers"].(map[string]interface{})
	if !ok {
		return nil
	}
	dmxapi, ok := providers[ProviderName].(map[string]interface{})
	if !ok {
		return nil // dmxapi 尚未注册，跳过（openclaw 启动时会从 openclaw.json 创建）
	}

	dmxapi["baseUrl"] = baseUrl
	dmxapi["api"] = apiFormat
	providers[ProviderName] = dmxapi
	raw["providers"] = providers

	updated, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 无法序列化 models.json: %v\n", err)
		return nil
	}
	if err := os.WriteFile(modelsPath, append(updated, '\n'), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "警告: 无法更新 models.json: %v\n", err)
	}
	return nil
}

// GetDMXAPIConfig 获取当前 DMXAPI 配置
func (cm *ConfigManager) GetDMXAPIConfig() (*DMXAPIConfig, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	dmxConfig := &DMXAPIConfig{
		BaseUrl: DefaultBaseUrl,
	}

	// 获取 BaseUrl
	if models, ok := config["models"].(map[string]interface{}); ok {
		if providers, ok := models["providers"].(map[string]interface{}); ok {
			if dmxapi, ok := providers[ProviderName].(map[string]interface{}); ok {
				if baseUrl, ok := dmxapi["baseUrl"].(string); ok {
					dmxConfig.BaseUrl = baseUrl
				}
			}
		}
	}

	// 获取当前模型
	if agents, ok := config["agents"].(map[string]interface{}); ok {
		if defaults, ok := agents["defaults"].(map[string]interface{}); ok {
			if model, ok := defaults["model"].(map[string]interface{}); ok {
				if primary, ok := model["primary"].(string); ok {
					// 格式为 "dmxapi/模型ID"，需要去掉前缀
					if len(primary) > len(ProviderName)+1 {
						dmxConfig.CurrentModel = primary[len(ProviderName)+1:]
					}
				}
			}
		}
	}

	// 获取 API Key
	apiKey, err := cm.GetApiKey()
	if err == nil {
		dmxConfig.ApiKey = apiKey
	}

	return dmxConfig, nil
}

// DetectAPIFormat 根据模型 ID 自动检测应使用的 API 格式
func DetectAPIFormat(modelID string) string {
	lower := strings.ToLower(modelID)
	if strings.HasPrefix(lower, "claude") {
		return "anthropic-messages"
	}
	if strings.HasSuffix(lower, "-cc") {
		return "anthropic-messages"
	}
	if strings.HasPrefix(lower, "gemini") {
		return "google-generative-ai"
	}
	if strings.HasPrefix(lower, "gpt-5") {
		return "openai-responses"
	}
	// o1/o3/o4 系列推理模型使用 Responses API
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"-") {
			return "openai-responses"
		}
	}
	return "openai-completions"
}

// UpdateDMXAPIConfig 更新 DMXAPI 配置
func (cm *ConfigManager) UpdateDMXAPIConfig(dmxConfig *DMXAPIConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	// 确保 models 结构存在
	models, ok := config["models"].(map[string]interface{})
	if !ok {
		models = map[string]interface{}{}
		config["models"] = models
	}

	providers, ok := models["providers"].(map[string]interface{})
	if !ok {
		providers = map[string]interface{}{}
		models["providers"] = providers
	}

	dmxapi, ok := providers[ProviderName].(map[string]interface{})
	if !ok {
		dmxapi = map[string]interface{}{}
		providers[ProviderName] = dmxapi
	}

	// 更新 BaseUrl
	dmxapi["baseUrl"] = dmxConfig.BaseUrl
	// 同步写入 apiKey（Gemini / OpenAI 格式从此处读取密钥）
	dmxapi["apiKey"] = dmxConfig.ApiKey

	// 根据模型名称自动检测并更新 api 格式（缓存结果，避免重复调用）
	apiFormat := DetectAPIFormat(dmxConfig.CurrentModel)
	dmxapi["api"] = apiFormat

	// 每次覆写 models 数组，只保留当前模型（清除历史积累的旧条目）
	modelId := dmxConfig.CurrentModel
	dmxapi["models"] = []interface{}{
		map[string]interface{}{
			"id":            modelId,
			"name":          modelId,
			"reasoning":     false,
			"input":         []string{"text"},
			"cost":          map[string]interface{}{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
			"contextWindow": 200000,
			"maxTokens":     8192,
		},
	}

	// 确保 mode 字段存在
	if models["mode"] == nil {
		models["mode"] = "merge"
	}

	// 确保 agents 结构存在
	agents, ok := config["agents"].(map[string]interface{})
	if !ok {
		agents = map[string]interface{}{}
		config["agents"] = agents
	}

	defaults, ok := agents["defaults"].(map[string]interface{})
	if !ok {
		defaults = map[string]interface{}{}
		agents["defaults"] = defaults
	}

	// 重置 models 别名，只保留当前模型（清除旧别名积累）
	fullModelId := ProviderName + "/" + modelId
	modelsAlias := map[string]interface{}{
		fullModelId: map[string]interface{}{"alias": modelId},
	}
	defaults["models"] = modelsAlias

	// 更新 primary model
	modelConfig, ok := defaults["model"].(map[string]interface{})
	if !ok {
		modelConfig = map[string]interface{}{}
		defaults["model"] = modelConfig
	}
	modelConfig["primary"] = fullModelId

	// 确保 auth 结构存在
	auth, ok := config["auth"].(map[string]interface{})
	if !ok {
		auth = map[string]interface{}{}
		config["auth"] = auth
	}

	authProfiles, ok := auth["profiles"].(map[string]interface{})
	if !ok {
		authProfiles = map[string]interface{}{}
		auth["profiles"] = authProfiles
	}

	profileKey := ProviderName + ":default"
	authProfiles[profileKey] = map[string]interface{}{
		"provider": ProviderName,
		"mode":     "api_key",
	}

	// 保存主配置
	if err := cm.SaveConfig(config); err != nil {
		return err
	}

	// 同步 models.json 中的 baseUrl/api，绕过 openclaw 合并逻辑对旧值的保留
	_ = cm.UpdateModelsJson(dmxConfig.BaseUrl, apiFormat)

	// 保存 API Key
	if dmxConfig.ApiKey != "" {
		if err := cm.SaveApiKey(dmxConfig.ApiKey); err != nil {
			return err
		}
	}

	return nil
}

// cleanOldBackups 在写入新备份前删除超出 maxKeep 限制的旧备份，确保总备份数不超过 maxKeep 个。
// 备份文件名后缀为时间戳（20060102150405），字典序即时间顺序，最旧的排最前。
func (cm *ConfigManager) cleanOldBackups(configPath string, maxKeep int) {
	dir := filepath.Dir(configPath)
	prefix := filepath.Base(configPath) + ".backup."
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			backups = append(backups, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(backups) // 时间戳字典序即时间顺序，最旧的排最前
	// 写入新备份后共 len(backups)+1 个，需删除多余的最旧备份
	if len(backups) >= maxKeep {
		for _, old := range backups[:len(backups)-maxKeep+1] {
			os.Remove(old)
		}
	}
}
