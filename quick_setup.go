package main

import (
	"fmt"
	"strings"

	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
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

	var provider config.ProviderConfig
	if len(fullCfg.Providers) > 0 {
		provider = fullCfg.Providers[0]
	}

	steps := []func() stepResult{
		// Step 1: Provider 配置
		func() stepResult {
			printSectionHeader("配置 Provider")
			p, cancelled := editProviderTerminal(provider)
			if cancelled {
				return stepBack
			}
			provider = p
			applyQuickProviderSelection(fullCfg, provider)
			return stepNext
		},
		// Step 2: 主模型选择
		func() stepResult {
			if len(provider.Models) == 0 {
				printError("当前 Provider 没有模型，请先返回添加模型")
				waitReturn()
				return stepBack
			}
			printSectionHeader("选择主 Agent 模型")
			items := make([]menuItem, len(provider.Models))
			for i, m := range provider.Models {
				items[i] = menuItem{Label: m, Desc: ""}
			}
			items = append(items, menuItem{Label: "← 返回上一步", Desc: ""})

			choice := selectMenu("主 Agent 模型：", items)
			if choice < 0 || choice == len(items)-1 {
				return stepBack
			}
			model := provider.Models[choice]
			applyQuickPrimaryModel(fullCfg, provider.Name, model)
			return stepNext
		},
		// Step 3: 确认
		func() stepResult {
			printSectionHeader("快速配置确认")
			fmt.Println("  当前快速配置最终只保留 1 个 Provider，只设置 1 个主 Agent 模型。")
			fmt.Println("  子 Agent / 命名 Agent 已改为默认继承主 Agent。")
			if hadAdvanced {
				fmt.Println("  你之前的高级配置已临时备份，可选择恢复原样。")
			}
			fmt.Println()

			items := []menuItem{
				{Label: "✔ 完成配置", Desc: "保存当前配置"},
				{Label: "↩ 恢复原样", Desc: "取消快速配置，恢复之前的状态"},
				{Label: "← 返回上一步", Desc: ""},
			}
			choice := selectMenu("请选择：", items)
			switch choice {
			case 0:
				return stepExit
			case 1:
				restoreQuickSetupSnapshot(fullCfg, snapshot)
				return stepExit
			default:
				return stepBack
			}
		},
	}

	runSteps(steps)
	return nil
}

func editProviderTerminal(p config.ProviderConfig) (config.ProviderConfig, bool) {
	name := p.Name
	baseUrl := p.BaseUrl
	if p.ApiFormat != "google-generative-ai" {
		baseUrl = strings.TrimSuffix(strings.TrimRight(p.BaseUrl, "/"), "/v1")
	}
	apiKey := p.ApiKey
	models := append([]string(nil), p.Models...)

	steps := []func() stepResult{
		// Step 1: Name
		func() stepResult {
			printSectionHeader("Provider 标识名")
			fmt.Println("  唯一英文 ID，只含小写字母、数字和连字符")
			if name != "" {
				fmt.Printf("  当前值: %s%s%s\n", cCyan, name, cReset)
			}
			v, esc := styledInputDefault("标识名", name)
			if esc {
				return stepBack
			}
			v = strings.TrimSpace(v)
			if v == "" {
				printError("标识名不能为空")
				waitReturn()
				return stepStay
			}
			name = v
			return stepNext
		},
		// Step 2: Base URL
		func() stepResult {
			printSectionHeader("Base URL")
			fmt.Println("  末尾的 /v1 会自动补全")
			if baseUrl != "" {
				fmt.Printf("  当前值: %s%s%s\n", cCyan, baseUrl, cReset)
			}
			defaultURL := "https://www.dmxapi.cn"
			if baseUrl != "" {
				defaultURL = baseUrl
			}
			v, esc := styledInputDefault("Base URL", defaultURL)
			if esc {
				return stepBack
			}
			baseUrl = strings.TrimSpace(v)
			if baseUrl == "" {
				baseUrl = defaultURL
			}
			return stepNext
		},
		// Step 3: API Key
		func() stepResult {
			printSectionHeader("API Key")
			if apiKey != "" {
				fmt.Printf("  当前值: %s%s%s\n", cCyan, maskKey(apiKey), cReset)
			}
			v, esc := readPassword("API Key (sk-...)")
			if esc {
				return stepBack
			}
			if strings.TrimSpace(v) == "" && apiKey == "" {
				printError("API Key 不能为空")
				waitReturn()
				return stepStay
			}
			if strings.TrimSpace(v) != "" {
				apiKey = strings.TrimSpace(v)
			}
			return stepNext
		},
		// Step 4: Models
		func() stepResult {
			printSectionHeader("模型列表")
			fmt.Println("  选择此 Provider 支持的预设模型")
			fmt.Println()

			// 显示预设模型列表
			presetSet := presetModelSet()
			for i, m := range models {
				marker := "  "
				if presetSet[m] {
					marker = cGreen + "✔ " + cReset
				} else {
					marker = cCyan + "◆ " + cReset
				}
				fmt.Printf("  %s%d.%s %s\n", marker, i+1, cReset, m)
			}
			fmt.Println()
			fmt.Printf("  %s当前共 %d 个模型%s\n", cDim, len(models), cReset)
			fmt.Println()

			items := []menuItem{
				{Label: "➕ 添加模型", Desc: "从预设列表选择或输入自定义模型"},
				{Label: "➖ 移除模型", Desc: "删除已有模型"},
				{Label: "继续 →", Desc: "确认模型列表"},
				{Label: "← 返回上一步", Desc: ""},
			}
			for {
				choice := selectMenu("模型管理：", items)
				switch choice {
				case 0: // 添加
					model := pickModelToAdd(models)
					if model != "" {
						models = appendUniqueStrings(models, model)
						printSuccessMsg("已添加: " + model)
					}
				case 1: // 移除
					if len(models) == 0 {
						printError("没有可移除的模型")
						continue
					}
					removeItems := make([]menuItem, len(models))
					for i, m := range models {
						removeItems[i] = menuItem{Label: m, Desc: ""}
					}
					rmChoice := selectMenu("选择要移除的模型：", removeItems)
					if rmChoice >= 0 {
						models = append(models[:rmChoice], models[rmChoice+1:]...)
						printSuccessMsg("已移除")
					}
				case 2: // 继续
					if len(models) == 0 {
						printError("请至少选择一个模型")
						continue
					}
					return stepNext
				default: // 返回
					return stepBack
				}
			}
		},
	}

	runSteps(steps)

	if len(models) == 0 {
		return config.ProviderConfig{}, true
	}

	apiFormat := detectFormatFromModels(models)
	baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)

	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		BaseUrl:   strings.TrimSpace(baseUrl),
		ApiKey:    strings.TrimSpace(apiKey),
		Models:    models,
		ApiFormat: apiFormat,
	}, false
}

func pickModelToAdd(existing []string) string {
	existingSet := make(map[string]bool, len(existing))
	for _, m := range existing {
		existingSet[m] = true
	}

	var items []menuItem
	var modelIDs []string

	// 预设模型
	for _, m := range models.PresetModels {
		if existingSet[m] {
			items = append(items, menuItem{Label: m, Desc: "已添加"})
		} else {
			items = append(items, menuItem{Label: m, Desc: "预设"})
			modelIDs = append(modelIDs, m)
		}
	}

	items = append(items, menuItem{Label: "✏️  自定义模型", Desc: "手动输入模型名称"})
	items = append(items, menuItem{Label: "← 返回", Desc: ""})

	choice := selectMenu("选择模型：", items)
	if choice < 0 || choice == len(items)-1 {
		return ""
	}

	if choice < len(models.PresetModels) {
		// 预设模型
		if existingSet[models.PresetModels[choice]] {
			printWarning("该模型已添加")
			return ""
		}
		return models.PresetModels[choice]
	}

	// 自定义模型
	v, esc := styledInput("模型名称")
	if esc || strings.TrimSpace(v) == "" {
		return ""
	}
	return strings.TrimSpace(v)
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

func hasAdvancedConfig(cfg *config.FullConfig) bool {
	return cfg.SubAgent.Primary != "" || cfg.SubAgent.Fallback != "" || len(cfg.NamedAgents) > 0
}
