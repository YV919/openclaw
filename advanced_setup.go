package main

import (
	"fmt"
	"strings"

	"openclaw_config/internal/config"
)

func (a *App) runAdvancedSetup(fullCfg *config.FullConfig) error {
	steps := []func() stepResult{
		// Step 1: Provider 管理
		func() stepResult {
			for {
				clearScreen()
				printSectionHeader("Step 1: Provider 管理")
				if len(fullCfg.Providers) > 0 {
					for _, p := range fullCfg.Providers {
						fmt.Printf("  %s✔%s %s%s%s  (%s)  [%s]\n", cGreen, cReset, cCyan+cBold, p.Name, cReset, p.BaseUrl, p.ApiFormat)
					}
					fmt.Println()
				}

				items := []menuItem{
					{Label: "➕ 添加新 Provider", Desc: ""},
				}
				for _, p := range fullCfg.Providers {
					items = append(items, menuItem{Label: p.Name, Desc: p.BaseUrl})
				}
				items = append(items, menuItem{Label: "继续 →", Desc: "进入下一步"})
				items = append(items, menuItem{Label: "← 返回", Desc: "取消高级配置"})

				choice := selectMenu("Provider 管理：", items)
				if choice < 0 || choice == len(items)-1 {
					return stepExit
				}
				if choice == len(items)-2 {
					if len(fullCfg.Providers) == 0 {
						printError("请至少添加一个 Provider")
						waitReturn()
						continue
					}
					return stepNext
				}
				if choice == 0 {
					// 添加
					p, cancelled := editProviderTerminal(config.ProviderConfig{})
					if !cancelled {
						fullCfg.Providers = append(fullCfg.Providers, p)
					}
				} else {
					// 编辑/删除已有
					idx := choice - 1
					p := fullCfg.Providers[idx]
					subItems := []menuItem{
						{Label: "编辑", Desc: "修改此 Provider"},
						{Label: "删除", Desc: "删除此 Provider"},
						{Label: "← 返回", Desc: ""},
					}
					subChoice := selectMenu("Provider: "+p.Name, subItems)
					switch subChoice {
					case 0:
						updated, cancelled := editProviderTerminal(p)
						if !cancelled {
							fullCfg.Providers[idx] = updated
						}
					case 1:
						yes, _ := styledConfirm("确认删除 "+p.Name+"？", false)
						if yes {
							fullCfg.Providers = append(fullCfg.Providers[:idx], fullCfg.Providers[idx+1:]...)
							// 清空引用
							clearProviderRefs(fullCfg, p.Name)
							printSuccessMsg("已删除: " + p.Name)
							waitReturn()
						}
					}
				}
			}
		},
		// Step 2: 主 Agent 模型
		func() stepResult {
			allModels := buildAllModelList(fullCfg.Providers)
			if len(allModels) == 0 {
				printError("没有可用的模型，请先返回添加 Provider 并配置模型")
				waitReturn()
				return stepBack
			}

			clearScreen()
			printSectionHeader("Step 2: 主 Agent 模型")
			primary := pickModelFromList("主 Agent 模型 (Primary)", allModels, fullCfg.MainAgent.Primary)
			if primary == "" {
				return stepBack
			}

			clearScreen()
			printSectionHeader("Step 2: 主 Agent 备用模型")
			fallbackItems := make([]menuItem, len(allModels)+1)
			for i, m := range allModels {
				fallbackItems[i] = menuItem{Label: m, Desc: ""}
			}
			fallbackItems[len(allModels)] = menuItem{Label: "（不配置）", Desc: "不设置备用模型"}
			fc := selectMenu("主 Agent 备用模型 (Fallback)：", fallbackItems)
			var fallback string
			if fc >= 0 && fc < len(allModels) {
				fallback = allModels[fc]
			}

			fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
			return stepNext
		},
		// Step 3: 子 Agent 模型
		func() stepResult {
			clearScreen()
			printSectionHeader("Step 3: 子 Agent 模型")
			items := []menuItem{
				{Label: "同主 Agent（不单独配置）", Desc: "默认继承主模型"},
				{Label: "单独指定", Desc: "为子 Agent 选择不同模型"},
				{Label: "← 返回上一步", Desc: ""},
			}
			choice := selectMenu("子 Agent 模型：", items)
			switch choice {
			case 0:
				fullCfg.SubAgent = config.AgentModelConfig{}
				return stepNext
			case 1:
				allModels := buildAllModelList(fullCfg.Providers)
				clearScreen()
				printSectionHeader("Step 3: 子 Agent 主模型")
				primary := pickModelFromList("子 Agent 主模型", allModels, fullCfg.SubAgent.Primary)
				if primary == "" {
					return stepStay
				}
				fullCfg.SubAgent = config.AgentModelConfig{Primary: primary}
				return stepNext
			default:
				return stepBack
			}
		},
		// Step 4: 命名 Agent
		func() stepResult {
			for {
				clearScreen()
				printSectionHeader("Step 4: 命名 Agent 管理")
				if len(fullCfg.NamedAgents) > 0 {
					for _, na := range fullCfg.NamedAgents {
						model := na.Model.Primary
						if model == "" {
							model = "同主 Agent"
						}
						fmt.Printf("  %s%s%s → %s\n", cCyan, na.ID, cReset, model)
					}
					fmt.Println()
				}

				items := []menuItem{
					{Label: "➕ 添加命名 Agent", Desc: "为特定 agent id 指定不同模型"},
				}
				for _, na := range fullCfg.NamedAgents {
					items = append(items, menuItem{Label: na.ID, Desc: "编辑 / 删除"})
				}
				items = append(items, menuItem{Label: "继续 →", Desc: "完成命名 Agent 配置"})
				items = append(items, menuItem{Label: "← 返回上一步", Desc: ""})

				choice := selectMenu("命名 Agent：", items)
				if choice < 0 || choice == len(items)-1 {
					return stepBack
				}
				if choice == len(items)-2 {
					return stepNext
				}
				if choice == 0 {
					// 添加
					id, esc := styledInput("Agent ID")
					if esc || strings.TrimSpace(id) == "" {
						continue
					}
					fullCfg.NamedAgents = append(fullCfg.NamedAgents, config.NamedAgentConfig{
						ID: strings.TrimSpace(id),
					})
				} else {
					// 编辑/删除
					idx := choice - 1
					na := fullCfg.NamedAgents[idx]
					subItems := []menuItem{
						{Label: "编辑模型", Desc: "修改此 Agent 的模型"},
						{Label: "删除", Desc: "删除此命名 Agent"},
						{Label: "← 返回", Desc: ""},
					}
					subChoice := selectMenu("Agent: "+na.ID, subItems)
					switch subChoice {
					case 0:
						allModels := buildAllModelList(fullCfg.Providers)
						primary := pickModelFromList("模型 for "+na.ID, allModels, na.Model.Primary)
						if primary != "" {
							fullCfg.NamedAgents[idx].Model.Primary = primary
						}
					case 1:
						yes, _ := styledConfirm("确认删除 "+na.ID+"？", false)
						if yes {
							fullCfg.NamedAgents = append(fullCfg.NamedAgents[:idx], fullCfg.NamedAgents[idx+1:]...)
							printSuccessMsg("已删除: " + na.ID)
							waitReturn()
						}
					}
				}
			}
		},
	}

	runSteps(steps)
	return nil
}

// buildAllModelList 从所有 Provider 构建 "provider/model" 格式的模型列表
func buildAllModelList(providers []config.ProviderConfig) []string {
	var list []string
	for _, p := range providers {
		for _, m := range p.Models {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				list = append(list, p.Name+"/"+trimmed)
			}
		}
	}
	return list
}

// pickModelFromList 从模型列表中选择一个
func pickModelFromList(title string, models []string, current string) string {
	items := make([]menuItem, len(models))
	for i, m := range models {
		desc := ""
		if m == current {
			desc = "当前"
		}
		items[i] = menuItem{Label: m, Desc: desc}
	}
	items = append(items, menuItem{Label: "← 返回", Desc: ""})

	choice := selectMenu(title+"：", items)
	if choice < 0 || choice == len(items)-1 {
		return ""
	}
	return models[choice]
}

// clearProviderRefs 清空引用指定 Provider 的 Agent 模型字段
func clearProviderRefs(cfg *config.FullConfig, providerName string) {
	prefix := providerName + "/"
	if strings.HasPrefix(cfg.MainAgent.Primary, prefix) {
		cfg.MainAgent.Primary = ""
	}
	if strings.HasPrefix(cfg.MainAgent.Fallback, prefix) {
		cfg.MainAgent.Fallback = ""
	}
	if strings.HasPrefix(cfg.SubAgent.Primary, prefix) {
		cfg.SubAgent.Primary = ""
	}
	if strings.HasPrefix(cfg.SubAgent.Fallback, prefix) {
		cfg.SubAgent.Fallback = ""
	}
	for i := range cfg.NamedAgents {
		if strings.HasPrefix(cfg.NamedAgents[i].Model.Primary, prefix) {
			cfg.NamedAgents[i].Model.Primary = ""
		}
		if strings.HasPrefix(cfg.NamedAgents[i].Model.Fallback, prefix) {
			cfg.NamedAgents[i].Model.Fallback = ""
		}
	}
}
