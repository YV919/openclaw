package main

import (
	"context"

	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cm, err := config.NewConfigManager()
	if err == nil {
		a.configManager = cm
	}
}

// GetPresetModels 返回预设模型列表
func (a *App) GetPresetModels() []string {
	return models.PresetModels
}

// GetCurrentConfig 获取当前配置
func (a *App) GetCurrentConfig() (*config.DMXAPIConfig, error) {
	if a.configManager == nil {
		return nil, nil
	}
	return a.configManager.GetDMXAPIConfig()
}

// SaveConfig 保存配置
func (a *App) SaveConfig(baseUrl, apiKey, model string) error {
	if a.configManager == nil {
		cm, err := config.NewConfigManager()
		if err != nil {
			return err
		}
		a.configManager = cm
	}

	dmxConfig := &config.DMXAPIConfig{
		BaseUrl:      baseUrl,
		ApiKey:       apiKey,
		CurrentModel: model,
	}

	return a.configManager.UpdateDMXAPIConfig(dmxConfig)
}
