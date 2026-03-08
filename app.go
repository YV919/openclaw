package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
)

type App struct {
	configManager *config.ConfigManager
}

func NewApp() *App {
	cm, err := config.NewConfigManager()
	if err != nil {
		log.Printf("警告: 初始化配置管理器失败: %v", err)
	}
	return &App{configManager: cm}
}

func (a *App) Run() error {
	// 加载现有配置作为表单默认值
	baseUrl := config.DefaultBaseUrl
	apiKey, model := "", ""
	if a.configManager != nil {
		if cur, err := a.configManager.GetDMXAPIConfig(); err == nil {
			baseUrl, apiKey, model = cur.BaseUrl, cur.ApiKey, cur.CurrentModel
			// 迁移旧默认值（无 /v1）到新默认值，避免老用户需要手动重新填写
			if baseUrl == "https://www.dmxapi.cn" {
				baseUrl = "https://www.dmxapi.cn/v1"
			}
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

	printBanner()

	selected := model
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Base URL").Placeholder("https://www.dmxapi.cn/v1").Value(&baseUrl),
		huh.NewInput().Title("API Key").Placeholder("sk-...").Value(&apiKey),
		huh.NewSelect[string]().Title("模型").Options(opts...).Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return fmt.Errorf("表单运行失败: %w", err)
	}

	// 若选了自定义：第二段表单收集模型名
	finalModel := selected
	if selected == "__custom__" {
		customModel = ""  // 重置，避免旧模型名拼接
		f2 := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("自定义模型名称").Value(&customModel),
		))
		if err := f2.Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintln(os.Stderr, "已取消")
				os.Exit(0)
			}
			return fmt.Errorf("表单运行失败: %w", err)
		}
		finalModel = strings.TrimSpace(customModel)
		if finalModel == "" {
			return fmt.Errorf("模型名称不能为空")
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API Key 不能为空")
	}
	trimmedUrl := strings.TrimSpace(baseUrl)
	if trimmedUrl == "" {
		baseUrl = config.DefaultBaseUrl
	} else {
		if _, err := url.ParseRequestURI(trimmedUrl); err != nil {
			return fmt.Errorf("Base URL 格式无效: %w", err)
		}
		baseUrl = trimmedUrl
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

	printSuccess(dmxConfig, config.DetectAPIFormat(finalModel))
	return nil
}

func printSuccess(cfg *config.DMXAPIConfig, apiFormat string) {
	green := lipgloss.Color("42")

	info := fmt.Sprintf(
		"  Base URL : %s\n  模型     : %s\n  API Key  : %s\n  API 格式 : %s",
		cfg.BaseUrl, cfg.CurrentModel, cfg.ApiKey, apiFormat,
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(green).
		Padding(1, 2).
		Render("✓ 配置已保存！\n\n" + info)

	fmt.Println()
	fmt.Println(box)

	huh.NewForm(huh.NewGroup( //nolint:errcheck — Note 表单无实质错误
		huh.NewNote().
			Title("提示").
			Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
			Next(true).
			NextLabel("按 Enter 退出"),
	)).Run() //nolint:errcheck
}

func printBanner() {
	purple := lipgloss.Color("63")  // xterm-256 亮紫
	gray := lipgloss.Color("240")   // 深灰

	art := "  ██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗\n" +
		"  ██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║\n" +
		"  ██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║\n" +
		"  ██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║\n" +
		"  ██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║\n" +
		"  ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝"

	logo := lipgloss.NewStyle().Foreground(purple).Render(art)

	subtitle := lipgloss.NewStyle().Bold(true).
		Render("  OpenClaw 配置工具  ·  openclaw-config " + Version)

	sep := lipgloss.NewStyle().Foreground(gray).
		Render("  ────────────────────────────────────────────────")

	note := lipgloss.NewStyle().Foreground(gray).
		Render("  支持 Claude / Gemini / GPT-5 / 其他兼容模型，自动检测 API 格式")

	fmt.Println(logo)
	fmt.Println()
	fmt.Println(subtitle)
	fmt.Println(sep)
	fmt.Println(note)
	fmt.Println()
}
