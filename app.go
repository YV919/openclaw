package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
)

type App struct {
	configManager *config.ConfigManager
}

func NewApp() *App {
	cm, _ := config.NewConfigManager()
	return &App{configManager: cm}
}

func (a *App) Run() error {
	// 加载现有配置作为表单默认值
	baseUrl := config.DefaultBaseUrl
	apiKey, model := "", ""
	if a.configManager != nil {
		if cur, err := a.configManager.GetDMXAPIConfig(); err == nil {
			baseUrl, apiKey, model = cur.BaseUrl, cur.ApiKey, cur.CurrentModel
		}
	}

	// 判断当前模型是否在预设列表中
	customModel := ""
	inPreset := false
	for _, m := range models.PresetModels {
		if m == model {
			inPreset = true
			break
		}
	}
	if !inPreset && model != "" {
		customModel = model
		model = "__custom__"
	}

	// 构建 Select 选项（预设模型 + "自定义模型..."）
	opts := make([]huh.Option[string], 0, len(models.PresetModels)+1)
	for _, m := range models.PresetModels {
		opts = append(opts, huh.NewOption(m, m))
	}
	opts = append(opts, huh.NewOption("自定义模型...", "__custom__"))

	fmt.Println("────────────────────────────────────────")
	fmt.Println("  OpenClaw DMXAPI 配置工具")
	fmt.Println("  注意：只能使用兼容 v1/messages 接口的模型")
	fmt.Println("────────────────────────────────────────")

	selected := model
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Base URL").Placeholder("https://www.dmxapi.cn").Value(&baseUrl),
		huh.NewInput().Title("API Key").Placeholder("sk-...").Password(true).Value(&apiKey),
		huh.NewSelect[string]().Title("模型").Options(opts...).Value(&selected),
	))
	if err := form.Run(); err != nil {
		return fmt.Errorf("已取消")
	}

	// 若选了自定义：第二段表单收集模型名
	finalModel := selected
	if selected == "__custom__" {
		f2 := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("自定义模型名称").Value(&customModel),
		))
		if err := f2.Run(); err != nil {
			return fmt.Errorf("已取消")
		}
		finalModel = strings.TrimSpace(customModel)
		if finalModel == "" {
			return fmt.Errorf("模型名称不能为空")
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API Key 不能为空")
	}
	if strings.TrimSpace(baseUrl) == "" {
		baseUrl = config.DefaultBaseUrl
	}

	// 懒初始化 configManager（首次运行无配置文件时）
	if a.configManager == nil {
		cm, err := config.NewConfigManager()
		if err != nil {
			return fmt.Errorf("初始化配置管理器失败: %w", err)
		}
		a.configManager = cm
	}

	dmxConfig := &config.DMXAPIConfig{
		BaseUrl:      strings.TrimSpace(baseUrl),
		ApiKey:       strings.TrimSpace(apiKey),
		CurrentModel: finalModel,
	}
	if err := a.configManager.UpdateDMXAPIConfig(dmxConfig); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 成功提示
	maskKey := func(k string) string {
		if len(k) <= 8 {
			return "****"
		}
		return k[:8] + "***"
	}
	fmt.Printf("\n✓ 配置已保存！\n")
	fmt.Printf("  Base URL : %s\n", dmxConfig.BaseUrl)
	fmt.Printf("  模型     : %s\n", dmxConfig.CurrentModel)
	fmt.Printf("  API Key  : %s\n", maskKey(dmxConfig.ApiKey))
	fmt.Printf("\n执行以下命令使配置生效:\n  openclaw gateway restart\n\n")
	return nil
}
