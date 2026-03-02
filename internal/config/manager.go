package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	OpenClawDir      = ".openclaw"
	ConfigFile       = "openclaw.json"
	AuthProfilesDir  = "agents/main/agent"
	AuthProfilesFile = "auth-profiles.json"
	DefaultBaseUrl   = "https://www.dmxapi.cn"
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

	// 备份原配置
	if _, err := os.Stat(configPath); err == nil {
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
	return "openai-completions"
}

// UpdateDMXAPIConfig 更新 DMXAPI 配置
func (cm *ConfigManager) UpdateDMXAPIConfig(dmxConfig *DMXAPIConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	// 确保 models 结构存在
	if config["models"] == nil {
		config["models"] = map[string]interface{}{}
	}
	models := config["models"].(map[string]interface{})

	if models["providers"] == nil {
		models["providers"] = map[string]interface{}{}
	}
	providers := models["providers"].(map[string]interface{})

	if providers[ProviderName] == nil {
		providers[ProviderName] = map[string]interface{}{}
	}
	dmxapi := providers[ProviderName].(map[string]interface{})

	// 更新 BaseUrl
	dmxapi["baseUrl"] = dmxConfig.BaseUrl
	// 同步写入 apiKey（Gemini / OpenAI 格式从此处读取密钥）
	dmxapi["apiKey"] = dmxConfig.ApiKey

	// 根据模型名称自动检测并更新 api 格式
	dmxapi["api"] = DetectAPIFormat(dmxConfig.CurrentModel)

	// 确保 models 数组存在并包含当前模型
	modelId := dmxConfig.CurrentModel
	modelExists := false
	if modelsArr, ok := dmxapi["models"].([]interface{}); ok {
		for _, m := range modelsArr {
			if modelMap, ok := m.(map[string]interface{}); ok {
				if modelMap["id"] == modelId {
					modelExists = true
					break
				}
			}
		}
		if !modelExists {
			// 添加新模型
			newModel := map[string]interface{}{
				"id":            modelId,
				"name":          modelId,
				"reasoning":     false,
				"input":         []string{"text"},
				"cost":          map[string]interface{}{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0},
				"contextWindow": 200000,
				"maxTokens":     8192,
			}
			dmxapi["models"] = append(modelsArr, newModel)
		}
	} else {
		// 创建 models 数组
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
	}

	// 确保 mode 字段存在
	if models["mode"] == nil {
		models["mode"] = "merge"
	}

	// 确保 agents 结构存在
	if config["agents"] == nil {
		config["agents"] = map[string]interface{}{}
	}
	agents := config["agents"].(map[string]interface{})

	if agents["defaults"] == nil {
		agents["defaults"] = map[string]interface{}{}
	}
	defaults := agents["defaults"].(map[string]interface{})

	// 更新 models 别名
	if defaults["models"] == nil {
		defaults["models"] = map[string]interface{}{}
	}
	modelsAlias := defaults["models"].(map[string]interface{})
	fullModelId := ProviderName + "/" + modelId
	modelsAlias[fullModelId] = map[string]interface{}{"alias": modelId}

	// 更新 primary model
	if defaults["model"] == nil {
		defaults["model"] = map[string]interface{}{}
	}
	modelConfig := defaults["model"].(map[string]interface{})
	modelConfig["primary"] = fullModelId

	// 确保 auth 结构存在
	if config["auth"] == nil {
		config["auth"] = map[string]interface{}{}
	}
	auth := config["auth"].(map[string]interface{})

	if auth["profiles"] == nil {
		auth["profiles"] = map[string]interface{}{}
	}
	authProfiles := auth["profiles"].(map[string]interface{})

	profileKey := ProviderName + ":default"
	authProfiles[profileKey] = map[string]interface{}{
		"provider": ProviderName,
		"mode":     "api_key",
	}

	// 保存主配置
	if err := cm.SaveConfig(config); err != nil {
		return err
	}

	// 保存 API Key
	if dmxConfig.ApiKey != "" {
		if err := cm.SaveApiKey(dmxConfig.ApiKey); err != nil {
			return err
		}
	}

	return nil
}
