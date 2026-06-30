package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"openclaw_config/internal/config"
)

// ErrUserCancelled 用户取消操作的哨兵错误
var ErrUserCancelled = errors.New("user cancelled")

type App struct {
	configManager *config.ConfigManager
}

func NewApp() (*App, error) {
	cm, err := config.NewConfigManager()
	if err != nil {
		return nil, fmt.Errorf("初始化配置管理器失败: %w", err)
	}
	return &App{configManager: cm}, nil
}

func (a *App) Run() error {
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

	for {
		clearScreen()
		printLogo()
		printBannerInfo(updateStatus)

		if len(fixLogs) > 0 {
			printWarning(fmt.Sprintf("已自动修正 %d 处配置：", len(fixLogs)))
			for _, log := range fixLogs {
				fmt.Println("    " + cDim + "· " + log + cReset)
			}
			fmt.Println()
		}

		// 主菜单
		items := []menuItem{
			{Label: "⚡ 快速配置", Desc: "只设置 Provider 和主模型，其他默认继承"},
			{Label: "⚙️  高级配置", Desc: "完整设置主 Agent、子 Agent、命名 Agent"},
			{Label: "退出", Desc: ""},
		}
		choice := selectMenu("选择配置模式：", items)
		if choice < 0 || choice == 2 {
			// ESC 或退出
			if choice == 2 {
				fmt.Println("再见 👋")
			}
			return nil
		}

		switch choice {
		case 0:
			if err := a.runQuickSetup(fullCfg); err != nil {
				if errors.Is(err, ErrUserCancelled) {
					continue
				}
				return err
			}
		case 1:
			if err := a.runAdvancedSetup(fullCfg); err != nil {
				if errors.Is(err, ErrUserCancelled) {
					continue
				}
				return err
			}
		}

		// 保存
		if err := a.configManager.SaveFullConfig(fullCfg); err != nil {
			return fmt.Errorf("保存配置失败: %w", err)
		}

		clearScreen()
		printLogo()
		printSuccessMsg("配置已保存！")
		printConfigSummaryDouble(fullCfg)
		fmt.Println()
		printInfo("默认在 OpenClaw 的 hybrid reload 下自动生效。")
		printInfo("若修改 gateway/plugins/discovery，请执行: openclaw gateway restart")
		waitReturn()
	}
}

func printBannerInfo(updateStatus releaseUpdateStatus) {
	if updateLine := buildUpdateStatusLine(updateStatus); updateLine != "" {
		if updateStatus.HasUpdate {
			fmt.Printf("  %s⚠ %s%s\n", cYellow+cBold, updateLine, cReset)
		} else {
			fmt.Printf("  %s%s%s\n", cDim, updateLine, cReset)
		}
		fmt.Println()
	}
}
