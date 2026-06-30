package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

type quickSetupSnapshot struct {
	Providers   []config.ProviderConfig
	MainAgent   config.AgentModelConfig
	SubAgent    config.AgentModelConfig
	NamedAgents []config.NamedAgentConfig
}

func (a *App) runQuickSetup(fullCfg *config.FullConfig) error {
	hadAdvanced := hasAdvancedConfig(fullCfg)
	snapshot := prepareQuickSetup(fullCfg)
	provider := config.ProviderConfig{}
	if len(fullCfg.Providers) > 0 {
		provider = fullCfg.Providers[0]
	}

	step := 1
	for step >= 1 && step <= 3 {
		switch step {
		case 1:
			selectedProvider, _, err := a.runQuickStep1Provider(fullCfg)
			if err != nil {
				return err
			}
			provider = selectedProvider
			step++
		case 2:
			back, err := a.runQuickStep2PrimaryModel(provider, fullCfg)
			if err != nil {
				return err
			}
			if back {
				step--
			} else {
				step++
			}
		case 3:
			action, err := pickQuickSetupAction(hadAdvanced)
			if err != nil {
				return err
			}
			switch action {
			case "__back__":
				step--
			case "__restore__":
				restoreQuickSetupSnapshot(fullCfg, snapshot)
				return nil
			case "__finish__":
				return nil
			default:
				return fmt.Errorf("未知快速配置操作: %q", action)
			}
		}
	}

	return nil
}

func (a *App) runQuickStep1Provider(fullCfg *config.FullConfig) (config.ProviderConfig, bool, error) {
	current := config.ProviderConfig{}
	if len(fullCfg.Providers) > 0 {
		current = fullCfg.Providers[0]
	}

	provider, cancelled, err := editProvider(current)
	if err != nil {
		return config.ProviderConfig{}, false, err
	}
	if cancelled {
		fmt.Fprintln(os.Stderr, "已取消")
		os.Exit(0)
	}

	applyQuickProviderSelection(fullCfg, provider)
	return provider, false, nil
}

func (a *App) runQuickStep2PrimaryModel(provider config.ProviderConfig, fullCfg *config.FullConfig) (bool, error) {
	opts := buildQuickPrimaryModelOptions(provider)
	if len(opts) == 0 {
		return true, nil
	}

	primary := strings.TrimPrefix(fullCfg.MainAgent.Primary, provider.Name+"/")
	if !containsOptValue(opts, primary) && len(opts) > 0 {
		primary = opts[0].Value
	}

	optsWithBack := append(append([]huh.Option[string](nil), opts...), huh.NewOption("← 返回上一步", "__back__"))
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("主 Agent 模型").
			Description("只从当前 Provider 的 models 中选择一个主模型").
			Options(optsWithBack...).
			Value(&primary),
	))
	if err := ui.RunForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	if primary == "__back__" {
		return true, nil
	}

	applyQuickPrimaryModel(fullCfg, provider.Name, primary)
	return false, nil
}

func pickQuickSetupAction(hadAdvanced bool) (string, error) {
	var selected string
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("快速配置确认").
			Description(quickSetupSummaryDescription(hadAdvanced)).
			Options(
				huh.NewOption("完成配置", "__finish__"),
				huh.NewOption("恢复原样", "__restore__"),
				huh.NewOption("← 返回上一步", "__back__"),
			).
			Value(&selected),
	))
	if err := ui.RunForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}

func quickSetupSummaryDescription(hadAdvanced bool) string {
	lines := []string{
		"当前快速配置最终只保留 1 个 Provider，并只设置 1 个主 Agent 模型。",
		"子 Agent / 命名 Agent 已改为默认继承主 Agent。",
	}
	if hadAdvanced {
		lines = append(lines, "你之前的高级配置已临时备份，可选择恢复原样。")
	} else {
		lines = append(lines, "当前进入快速配置前的状态也已临时备份，可直接恢复原样。")
	}
	lines = append(lines, "请选择完成配置，或恢复原样。")
	return strings.Join(lines, "\n")
}

func prepareQuickSetup(cfg *config.FullConfig) quickSetupSnapshot {
	snapshot := quickSetupSnapshot{
		Providers:   cloneProviders(cfg.Providers),
		MainAgent:   cfg.MainAgent,
		SubAgent:    cfg.SubAgent,
		NamedAgents: cloneNamedAgents(cfg.NamedAgents),
	}
	cfg.SubAgent = config.AgentModelConfig{}
	cfg.NamedAgents = nil
	return snapshot
}

func restoreQuickSetupSnapshot(cfg *config.FullConfig, snapshot quickSetupSnapshot) {
	cfg.Providers = cloneProviders(snapshot.Providers)
	cfg.MainAgent = snapshot.MainAgent
	cfg.SubAgent = snapshot.SubAgent
	cfg.NamedAgents = cloneNamedAgents(snapshot.NamedAgents)
}

func applyQuickProviderSelection(cfg *config.FullConfig, provider config.ProviderConfig) {
	cfg.Providers = []config.ProviderConfig{provider}
}

func applyQuickPrimaryModel(cfg *config.FullConfig, providerName string, model string) {
	cfg.MainAgent.Primary = providerName + "/" + model
	cfg.MainAgent.Fallback = ""
	cfg.SubAgent = config.AgentModelConfig{}
	cfg.NamedAgents = nil
}

func buildQuickPrimaryModelOptions(provider config.ProviderConfig) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(provider.Models))
	for _, model := range provider.Models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		opts = append(opts, huh.NewOption(trimmed, trimmed))
	}
	return opts
}

func hasAdvancedConfig(cfg *config.FullConfig) bool {
	return cfg.SubAgent.Primary != "" || cfg.SubAgent.Fallback != "" || len(cfg.NamedAgents) > 0
}
