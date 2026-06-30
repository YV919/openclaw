package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// Provider/NamedAgent 操作常量
const (
	actionAdd      = "__add__"
	actionContinue = "__continue__"
	actionEdit     = "__edit__"
	actionDelete   = "__delete__"
	actionSame     = "__same__"
	actionCustom   = "__custom__"
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
		if action == actionContinue {
			break
		}
		if action == actionAdd {
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
			case actionEdit:
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
			case actionDelete:
				if err := deleteProvider(fullCfg, action); err != nil {
					return err
				}
			}
			// actionBack 直接继续外层 for
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
	opts = append(opts, huh.NewOption("[+ 添加新 Provider]", actionAdd))
	if len(providers) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", actionContinue))
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
			return "", ErrUserCancelled
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
				huh.NewOption("编辑", actionEdit),
				huh.NewOption("删除", actionDelete),
				huh.NewOption("← 返回", actionBack),
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

	optsWithBack := append(append([]huh.Option[string](nil), allOpts...), huh.NewOption("← 返回上一步", actionBack))

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
			return false, ErrUserCancelled
		}
		return false, err
	}
	if primary == actionBack {
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
	const sameAsMain = actionSame
	subChoice := sameAsMain
	if fullCfg.SubAgent.Primary != "" {
		subChoice = actionCustom
	}

	form1 := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", actionCustom),
				huh.NewOption("← 返回上一步", actionBack),
			).
			Value(&subChoice),
	))
	if err := ui.RunForm(form1); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, ErrUserCancelled
		}
		return false, err
	}
	if subChoice == actionBack {
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
			return false, ErrUserCancelled
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
		case actionBack:
			return true, nil
		case actionContinue:
			return false, nil
		default:
			// 选中已有 Agent → 二级菜单
			subAction, err := pickNamedAgentItemAction(action)
			if err != nil {
				return false, err
			}
			switch subAction {
			case actionEdit:
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
			case actionDelete:
				newAgents := make([]config.NamedAgentConfig, 0, len(fullCfg.NamedAgents))
				for _, na := range fullCfg.NamedAgents {
					if na.ID != action {
						newAgents = append(newAgents, na)
					}
				}
				fullCfg.NamedAgents = newAgents
			}
			// actionBack 继续外层 for
		}
	}
}
