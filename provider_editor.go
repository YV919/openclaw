package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
	"openclaw_config/internal/ui"
)

// ── Provider 编辑器 ──────────────────────────────────────────────────────

// buildAllModelOpts 从所有 provider 构建完整模型选项列表（格式 "provider/model"）
func buildAllModelOpts(providers []config.ProviderConfig) []huh.Option[string] {
	var opts []huh.Option[string]
	for _, p := range providers {
		for _, m := range p.Models {
			trimmed := strings.TrimSpace(m)
			if trimmed == "" {
				continue
			}
			fullID := p.Name + "/" + trimmed
			opts = append(opts, huh.NewOption(fullID, fullID))
		}
	}
	return opts
}

// checkProviderDeps 返回引用了指定 provider 模型的 Agent 描述列表
func checkProviderDeps(providerName string, cfg *config.FullConfig) []string {
	prefix := providerName + "/"
	var deps []string
	if strings.HasPrefix(cfg.MainAgent.Primary, prefix) {
		deps = append(deps, "主 Agent (primary)")
	}
	if strings.HasPrefix(cfg.MainAgent.Fallback, prefix) {
		deps = append(deps, "主 Agent (fallback)")
	}
	if strings.HasPrefix(cfg.SubAgent.Primary, prefix) {
		deps = append(deps, "子 Agent (primary)")
	}
	if strings.HasPrefix(cfg.SubAgent.Fallback, prefix) {
		deps = append(deps, "子 Agent (fallback)")
	}
	for _, na := range cfg.NamedAgents {
		if strings.HasPrefix(na.Model.Primary, prefix) {
			deps = append(deps, fmt.Sprintf("命名 Agent [%s] (primary)", na.ID))
		}
		if strings.HasPrefix(na.Model.Fallback, prefix) {
			deps = append(deps, fmt.Sprintf("命名 Agent [%s] (fallback)", na.ID))
		}
	}
	return deps
}

// deleteProvider 删除指定 provider，删除前检查依赖并警告，确认后清空相关 NamedAgent 的模型引用
func deleteProvider(fullCfg *config.FullConfig, name string) error {
	deps := checkProviderDeps(name, fullCfg)
	if len(deps) > 0 {
		fmt.Println(ui.FormatWarning(fmt.Sprintf("Provider %q 被以下配置引用：", name), deps))
		fmt.Println()
	}

	var confirmed bool
	form := ui.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("确认删除 Provider %q？", name)).
			Description("删除后相关 NamedAgent 的模型引用将被清空，主/子 Agent 的引用将在下一步重新选择。").
			Value(&confirmed),
	))
	if err := ui.RunForm(form); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	if !confirmed {
		return nil
	}

	// 从 Providers 中移除
	newProviders := make([]config.ProviderConfig, 0, len(fullCfg.Providers))
	for _, p := range fullCfg.Providers {
		if p.Name != name {
			newProviders = append(newProviders, p)
		}
	}
	fullCfg.Providers = newProviders

	// 清空引用该 provider 的 NamedAgent 模型字段
	prefix := name + "/"
	for i := range fullCfg.NamedAgents {
		if strings.HasPrefix(fullCfg.NamedAgents[i].Model.Primary, prefix) {
			fullCfg.NamedAgents[i].Model.Primary = ""
		}
		if strings.HasPrefix(fullCfg.NamedAgents[i].Model.Fallback, prefix) {
			fullCfg.NamedAgents[i].Model.Fallback = ""
		}
	}
	return nil
}

func presetModelSet() map[string]bool {
	set := make(map[string]bool, len(models.PresetModels))
	for _, m := range models.PresetModels {
		set[m] = true
	}
	return set
}

func splitProviderModelsForEdit(providerModels []string) ([]string, []string) {
	return splitProviderModelsWithPreset(providerModels, presetModelSet())
}

func splitProviderModelsWithPreset(providerModels []string, presetSet map[string]bool) ([]string, []string) {
	selectedModels := make([]string, 0, len(providerModels))
	customModels := make([]string, 0, len(providerModels))

	for _, modelID := range providerModels {
		trimmed := strings.TrimSpace(modelID)
		if trimmed == "" {
			continue
		}
		selectedModels = appendUniqueStrings(selectedModels, trimmed)
		if presetSet[trimmed] {
			continue
		}
		customModels = appendUniqueStrings(customModels, trimmed)
	}

	return selectedModels, customModels
}

func buildProviderModelOptions(selectedModels []string, customModelRegistry []string) []huh.Option[string] {
	return buildProviderModelOptionsWithPreset(selectedModels, customModelRegistry, presetModelSet())
}

func buildProviderModelOptionsWithPreset(selectedModels []string, customModelRegistry []string, presetSet map[string]bool) []huh.Option[string] {
	selectedSet := make(map[string]bool, len(selectedModels))
	for _, modelID := range selectedModels {
		if trimmed := strings.TrimSpace(modelID); trimmed != "" {
			selectedSet[trimmed] = true
		}
	}

	opts := make([]huh.Option[string], 0, len(models.PresetModels)+len(customModelRegistry))
	for _, modelID := range models.PresetModels {
		opt := huh.NewOption(modelID, modelID)
		if selectedSet[modelID] {
			opt = opt.Selected(true)
		}
		opts = append(opts, opt)
	}

	for _, modelID := range customModelRegistry {
		trimmed := strings.TrimSpace(modelID)
		if trimmed == "" || presetSet[trimmed] {
			continue
		}
		opt := huh.NewOption(trimmed, trimmed)
		if selectedSet[trimmed] {
			opt = opt.Selected(true)
		}
		opts = append(opts, opt)
	}

	return opts
}

func finalProviderModels(selectedModels []string) []string {
	finalModels := make([]string, 0, len(selectedModels))
	for _, modelID := range selectedModels {
		if trimmed := strings.TrimSpace(modelID); trimmed != "" {
			finalModels = appendUniqueStrings(finalModels, trimmed)
		}
	}
	return finalModels
}

func parseCustomModelInput(input string) []string {
	normalized := strings.NewReplacer(
		"；", ",",
		";", ",",
		"，", ",",
		"\r\n", "\n",
		"\r", "\n",
		"\n", ",",
	).Replace(input)

	parts := strings.Split(normalized, ",")
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			models = appendUniqueStrings(models, trimmed)
		}
	}
	return models
}

func registerProviderCustomModels(selectedModels []string, customModelRegistry []string, input string) ([]string, []string, []string) {
	presetSet := presetModelSet()
	updatedSelected := append([]string(nil), selectedModels...)
	updatedRegistry := append([]string(nil), customModelRegistry...)
	addedCustomModels := make([]string, 0)

	for _, modelID := range parseCustomModelInput(input) {
		updatedSelected = appendUniqueStrings(updatedSelected, modelID)
		if presetSet[modelID] {
			continue
		}
		beforeLen := len(updatedRegistry)
		updatedRegistry = appendUniqueStrings(updatedRegistry, modelID)
		if len(updatedRegistry) > beforeLen {
			addedCustomModels = append(addedCustomModels, modelID)
		}
	}

	return updatedSelected, updatedRegistry, addedCustomModels
}

func editProvider(p config.ProviderConfig) (config.ProviderConfig, bool, error) {
	presetSet := presetModelSet()
	name := p.Name
	// 格式感知的展示剥离：仅对自动补全 /v1 的格式剥除，google-generative-ai 原样
	baseUrl := p.BaseUrl
	if p.ApiFormat != "google-generative-ai" {
		baseUrl = strings.TrimSuffix(strings.TrimRight(p.BaseUrl, "/"), "/v1")
	}
	apiKey := p.ApiKey

	selectedModels, customModelRegistry := splitProviderModelsWithPreset(p.Models, presetSet)
	customModelInput := ""
	modelOptionsVersion := 0

	nameField := huh.NewInput().
		Title("Provider 标识名").
		Description("唯一英文 ID，只含小写字母、数字和连字符，用于区分不同服务商（如 dmxapi-cn、dmxapi-ssvip）").
		Placeholder("my-proxy").
		Validate(func(s string) error {
			s = strings.TrimSpace(s)
			if s == "" {
				return fmt.Errorf("name 不能为空")
			}
			for _, c := range s {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
					return fmt.Errorf("只允许小写字母、数字和连字符，当前输入含非法字符: %c", c)
				}
			}
			return nil
		}).
		Value(&name)

	baseURLField := huh.NewInput().
		Title("Base URL").
		Description("末尾的 /v1 会自动补全，无需手动填写").
		Placeholder("https://www.dmxapi.cn").
		Validate(func(s string) error {
			s = strings.TrimSpace(s)
			if s == "" {
				return fmt.Errorf("Base URL 不能为空")
			}
			u, err := url.ParseRequestURI(s)
			if err != nil || u.Scheme == "" {
				return fmt.Errorf("URL 格式无效（需包含 http:// 或 https://）")
			}
			return nil
		}).
		Value(&baseUrl)

	apiKeyField := huh.NewInput().
		Title("API Key").
		Placeholder("sk-...").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("API Key 不能为空")
			}
			return nil
		}).
		Value(&apiKey)

	customModelField := ui.NewProviderCustomModelInput(
		&customModelInput,
		func(raw string) ui.ProviderCustomRegistration {
			parsed := parseCustomModelInput(raw)
			if len(parsed) == 0 {
				return ui.ProviderCustomRegistration{}
			}
			var added []string
			selectedModels, customModelRegistry, added = registerProviderCustomModels(selectedModels, customModelRegistry, raw)
			result := ui.ProviderCustomRegistration{
				Handled: true,
				Added:   len(added) > 0,
			}
			if result.Added {
				modelOptionsVersion++
			}
			return result
		},
	)

	modelListField := ui.NewProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(ui.ProviderModelListBaseDescription).
			OptionsFunc(func() []huh.Option[string] {
				return buildProviderModelOptions(selectedModels, customModelRegistry)
			}, &modelOptionsVersion).
			Validate(func(selected []string) error {
				if len(finalProviderModels(selected)) == 0 {
					return fmt.Errorf("请至少选择一个模型")
				}
				return nil
			}).
			Value(&selectedModels).
			WithTheme(ui.ProviderModelListTheme()).(*huh.MultiSelect[string]),
		func(windowHeight int) int {
			return ui.ProviderModelListAvailableFieldHeight(
				windowHeight,
				nameField.View(),
				baseURLField.View(),
				apiKeyField.View(),
				customModelField.View(),
			)
		},
		func() int {
			return len(buildProviderModelOptions(selectedModels, customModelRegistry))
		},
	)

	form := ui.NewForm(huh.NewGroup(
		nameField,
		baseURLField,
		apiKeyField,
		modelListField,
		customModelField,
	))

	if err := ui.RunFormWithFooter(form, func() string {
		return ui.ProviderEditorHelpFooter(modelListField.IsFocused())
	}); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return config.ProviderConfig{}, true, nil
		}
		return config.ProviderConfig{}, false, err
	}

	finalModels := finalProviderModels(selectedModels)
	if len(finalModels) == 0 {
		return config.ProviderConfig{}, false, fmt.Errorf("provider %q 的模型列表不能为空", name)
	}

	apiFormat := detectFormatFromModels(finalModels)
	baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)

	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		BaseUrl:   strings.TrimSpace(baseUrl),
		ApiKey:    strings.TrimSpace(apiKey),
		Models:    finalModels,
		ApiFormat: apiFormat,
	}, false, nil
}

// detectFormatFromModels 根据模型列表自动推断 API 格式。
// 若所有模型格式一致（含单模型情形），直接返回该格式。
// 若存在冲突，打印黄色警告并返回第一个模型的格式。
// 调用方保证 models 非空。
func detectFormatFromModels(modelIDs []string) string {
	seen := make(map[string]bool, len(modelIDs))
	for _, m := range modelIDs {
		seen[config.DetectAPIFormat(m)] = true
	}
	if len(seen) == 1 {
		for f := range seen {
			return f
		}
	}
	// 存在冲突：打印警告，返回第一个模型的格式
	var formatLines []string
	for _, m := range modelIDs {
		formatLines = append(formatLines, fmt.Sprintf("%s → %s", m, config.DetectAPIFormat(m)))
	}
	fmt.Println(ui.FormatWarning("所选模型包含不同 API 格式，将使用第一个模型的格式：", formatLines))
	dim := ui.DimStyle()
	fmt.Println(dim.Render("  建议：将不同格式的模型拆分为独立 provider"))
	fmt.Println()
	return config.DetectAPIFormat(modelIDs[0])
}
