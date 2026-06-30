package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// ErrUserCancelled 用户取消操作的哨兵错误
var ErrUserCancelled = errors.New("user cancelled")

type App struct {
	configManager *config.ConfigManager
}

type setupMode string

const (
	setupModeQuick    setupMode = "quick"
	setupModeAdvanced setupMode = "advanced"
)

func NewApp() *App {
	cm, err := config.NewConfigManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告: 初始化配置管理器失败: %v\n", err)
	}
	return &App{configManager: cm}
}

func (a *App) Run() error {
	if a.configManager == nil {
		cm, err := config.NewConfigManager()
		if err != nil {
			return fmt.Errorf("初始化配置管理器失败: %w", err)
		}
		a.configManager = cm
	}

	// 加载现有配置（含兼容性迁移）
	fullCfg, fixLogs, err := a.configManager.LoadFullConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	updateStatus, err := checkForUpdates(
		&http.Client{Timeout: 5 * time.Second},
		Version,
		defaultReleasesURL,
	)
	if err != nil {
		updateStatus = releaseUpdateStatus{}
	}

	printBanner(updateStatus)

	// 展示兼容性修复日志
	if len(fixLogs) > 0 {
		fmt.Println(ui.FormatWarning(fmt.Sprintf("已自动修正 %d 处配置：", len(fixLogs)), fixLogs))
		fmt.Println()
	}

	mode, err := pickSetupMode(fullCfg)
	if err != nil {
		if errors.Is(err, ErrUserCancelled) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}

	switch mode {
	case setupModeQuick:
		if err := a.runQuickSetup(fullCfg); err != nil {
			return err
		}
	case setupModeAdvanced:
		if err := a.runAdvancedSetup(fullCfg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知配置模式: %q", mode)
	}

	// 最终写入
	if err := a.configManager.SaveFullConfig(fullCfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	printSuccess(fullCfg)
	return nil
}

func pickSetupMode(cfg *config.FullConfig) (setupMode, error) {
	var selected setupMode
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[setupMode]().
			Title("选择配置模式").
			Description(setupModeDescription(cfg)).
			Options(
				huh.NewOption("快速配置", setupModeQuick),
				huh.NewOption("高级配置", setupModeAdvanced),
			).
			Value(&selected),
	))
	if err := ui.RunForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrUserCancelled
		}
		return "", err
	}
	return selected, nil
}

func setupModeDescription(cfg *config.FullConfig) string {
	description := "快速配置：只设置 Provider 和主模型，其他 Agent 默认继承主模型。\n高级配置：完整设置主 Agent、子 Agent 和命名 Agent。"
	if hasAdvancedConfig(cfg) {
		description += "\n当前检测到已有高级配置；进入快速配置后会先临时备份，稍后可恢复原样。"
	}
	return description
}
