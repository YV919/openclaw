package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// ── Advanced Setup ──────────────────────────────────────────────────────

func (a *App) runAdvancedSetup(fullCfg *config.FullConfig) error {
	var allModelOpts []huh.Option[string]
	var allModelOptsWithNone []huh.Option[string]

	step := 1
	for step >= 1 && step <= 4 {
		// 每次进入 Step 2 前重建模型选项（Step 1 可能修改了 Providers）
		if step == 2 {
			allModelOpts = buildAllModelOpts(fullCfg.Providers)
			allModelOptsWithNone = append(
				[]huh.Option[string]{huh.NewOption("（不配置）", "")},
				allModelOpts...,
			)
		}

		var back bool
		var err error
		switch step {
		case 1:
			err = a.runStep1Providers(fullCfg) // Step 1 是首步，无需返回上一步
		case 2:
			back, err = a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone)
		case 3:
			back, err = a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone)
		case 4:
			back, err = a.runStep4NamedAgents(fullCfg, allModelOpts)
		}
		if err != nil {
			return err
		}
		if back {
			step--
		} else {
			step++
		}
	}

	return nil
}

// ── Step 1: Provider 管理 ──────────────────────────────────────────────────

func (a *App) runStep1Providers(fullCfg *config.FullConfig) error {
	for {
		action, err := pickProviderAction(fullCfg.Providers)
		if err != nil {
			return err
		}
		if action == "__continue__" {
			break
		}
		if action == "__add__" {
			p, cancelled, err := editProvider(config.ProviderConfig{})
			if err != nil {
				return err
			}
			if !cancelled {
				fullCfg.Providers = append(fullCfg.Providers, p)
			}
		} else {
			// 选中已有 provider → 弹二级菜单
			subAction, err := pickProviderItemAction(action)
			if err != nil {
				return err
			}
			switch subAction {
			case "__edit__":
				for i, p := range fullCfg.Providers {
					if p.Name == action {
						updated, cancelled, err := editProvider(p)
						if err != nil {
							return err
						}
						if !cancelled {
							fullCfg.Providers[i] = updated
						}
						break
					}
				}
			case "__delete__":
				if err := deleteProvider(fullCfg, action); err != nil {
					return err
				}
			}
			// "__back__" 直接继续外层 for
		}
	}
	return nil
}

func pickProviderAction(providers []config.ProviderConfig) (string, error) {
	var opts []huh.Option[string]
	for _, p := range providers {
		label := fmt.Sprintf("%s  (%s)", p.Name, p.BaseUrl)
		opts = append(opts, huh.NewOption(label, p.Name))
	}
	opts = append(opts, huh.NewOption("[+ 添加新 Provider]", "__add__"))
	if len(providers) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
	}

	var selected string
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider 管理").
			Description(ui.ProviderManagementDescription()).
			Options(opts...).
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

func pickProviderItemAction(name string) (string, error) {
	var selected string
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Provider: %s", name)).
			Options(
				huh.NewOption("编辑", "__edit__"),
				huh.NewOption("删除", "__delete__"),
				huh.NewOption("← 返回", "__back__"),
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

// ── Step 2: 主 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep2MainAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (bool, error) {
	primary := fullCfg.MainAgent.Primary
	fallback := fullCfg.MainAgent.Fallback
	if !containsOptValue(allOpts, primary) {
		primary = ""
	}
	if !containsOptValue(allOptsWithNone, fallback) {
		fallback = ""
	}
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	optsWithBack := append(append([]huh.Option[string](nil), allOpts...), huh.NewOption("← 返回上一步", "__back__"))

	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("主 Agent 模型 (Primary)").
			Description("agents.defaults.model.primary").
			Options(optsWithBack...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("主 Agent 备用模型 (Fallback)").
			Description("可选，留空表示不配置备用模型").
			Options(allOptsWithNone...).
			Value(&fallback),
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
	fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return false, nil
}

// ── Step 3: 子 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep3SubAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (bool, error) {
	const sameAsMain = "__same__"
	subChoice := sameAsMain
	if fullCfg.SubAgent.Primary != "" {
		subChoice = "__custom__"
	}

	form1 := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", "__custom__"),
				huh.NewOption("← 返回上一步", "__back__"),
			).
			Value(&subChoice),
	))
	if err := ui.RunForm(form1); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	if subChoice == "__back__" {
		return true, nil
	}
	if subChoice == sameAsMain {
		fullCfg.SubAgent = config.AgentModelConfig{}
		return false, nil
	}

	// 单独指定
	primary := fullCfg.SubAgent.Primary
	fallback := fullCfg.SubAgent.Fallback
	if !containsOptValue(allOpts, primary) {
		primary = ""
	}
	if !containsOptValue(allOptsWithNone, fallback) {
		fallback = ""
	}
	if primary == "" && len(allOpts) > 0 {
		primary = allOpts[0].Value
	}

	form2 := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 主模型 (Primary)").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("子 Agent 备用模型 (Fallback)").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := ui.RunForm(form2); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return false, err
	}
	fullCfg.SubAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return false, nil
}

// ── Step 4: 命名 Agent（可选） ─────────────────────────────────────────────
func (a *App) runStep4NamedAgents(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
) (bool, error) {
	const sameAsMain = ""
	allOptsWithSame := append(
		[]huh.Option[string]{huh.NewOption("同主 Agent", sameAsMain)},
		allOpts...,
	)
	allOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allOpts...,
	)

	for {
		action, err := pickNamedAgentAction(fullCfg.NamedAgents)
		if err != nil {
			return false, err
		}

		switch action {
		case "__back__":
			return true, nil
		case "__continue__":
			return false, nil
		default:
			// 选中已有 Agent → 二级菜单
			subAction, err := pickNamedAgentItemAction(action)
			if err != nil {
				return false, err
			}
			switch subAction {
			case "__edit__":
				for i, na := range fullCfg.NamedAgents {
					if na.ID == action {
						updated, cancelled, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return false, err
						}
						if !cancelled {
							fullCfg.NamedAgents[i] = updated
						}
						break
					}
				}
			case "__delete__":
				newAgents := make([]config.NamedAgentConfig, 0, len(fullCfg.NamedAgents))
				for _, na := range fullCfg.NamedAgents {
					if na.ID != action {
						newAgents = append(newAgents, na)
					}
				}
				fullCfg.NamedAgents = newAgents
			}
			// "__back__" 继续外层 for
		}
	}
}
