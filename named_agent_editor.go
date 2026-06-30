package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// pickNamedAgentAction 命名 Agent 一级选单
func pickNamedAgentAction(agents []config.NamedAgentConfig) (string, error) {
	var opts []huh.Option[string]
	for _, na := range agents {
		modelLabel := na.Model.Primary
		if modelLabel == "" {
			modelLabel = "同主 Agent"
		}
		label := fmt.Sprintf("%s  (%s)", na.ID, modelLabel)
		opts = append(opts, huh.NewOption(label, na.ID))
	}
	opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
	opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))

	var selected string
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("命名 Agent 管理").
			Description("为特定 agent id 指定不同模型").
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

// pickNamedAgentItemAction 命名 Agent 二级操作菜单
func pickNamedAgentItemAction(id string) (string, error) {
	var selected string
	form := ui.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("命名 Agent: %s", id)).
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

// editNamedAgent 编辑已有命名 Agent（Agent ID 只读），支持 Primary + Fallback。
// 返回值：(更新后的 Agent, 是否取消, error)
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, bool, error) {
	primary := agent.Model.Primary
	fallback := agent.Model.Fallback

	form := ui.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("Agent ID").
			Description(agent.ID),
		huh.NewSelect[string]().
			Title("使用模型 (Primary)").
			Options(allOptsWithSame...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("备用模型 (Fallback)").
			Description("可选，留空表示不配置").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := ui.RunForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return config.NamedAgentConfig{}, true, nil
		}
		return config.NamedAgentConfig{}, false, err
	}
	return config.NamedAgentConfig{
		ID:    agent.ID,
		Model: config.AgentModelConfig{Primary: primary, Fallback: fallback},
	}, false, nil
}
