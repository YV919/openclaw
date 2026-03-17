package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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

	printBanner()

	// 展示兼容性修复日志
	if len(fixLogs) > 0 {
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
		fmt.Println(yellow.Render(fmt.Sprintf("  ⚠ 已自动修正 %d 处配置：", len(fixLogs))))
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		for _, log := range fixLogs {
			fmt.Println(dim.Render("    · " + log))
		}
		fmt.Println()
	}

	// Step 1: Provider 管理
	if err := a.runStep1Providers(fullCfg); err != nil {
		return err
	}

	// 构建全部模型选项（供后续步骤使用）
	allModelOpts := buildAllModelOpts(fullCfg.Providers)
	allModelOptsWithNone := append(
		[]huh.Option[string]{huh.NewOption("（不配置）", "")},
		allModelOpts...,
	)

	// Step 2: 主 Agent 模型
	if err := a.runStep2MainAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 3: 子 Agent 模型
	if err := a.runStep3SubAgent(fullCfg, allModelOpts, allModelOptsWithNone); err != nil {
		return err
	}

	// Step 4: 命名 Agent（可选）
	if err := a.runStep4NamedAgents(fullCfg, allModelOpts); err != nil {
		return err
	}

	// 最终写入
	if err := a.configManager.SaveFullConfig(fullCfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	printSuccess(fullCfg)
	return nil
}

// buildAllModelOpts 从所有 provider 构建完整模型选项列表（格式 "provider/model"）
func buildAllModelOpts(providers []config.ProviderConfig) []huh.Option[string] {
	var opts []huh.Option[string]
	for _, p := range providers {
		for _, m := range p.Models {
			fullID := p.Name + "/" + m
			opts = append(opts, huh.NewOption(fullID, fullID))
		}
	}
	return opts
}

// containsOptValue 检查 opts 中是否存在 value 对应的选项
func containsOptValue(opts []huh.Option[string], value string) bool {
	for _, o := range opts {
		if o.Value == value {
			return true
		}
	}
	return false
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
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		fmt.Println(yellow.Render(fmt.Sprintf("  ⚠ Provider %q 被以下配置引用：", name)))
		for _, d := range deps {
			fmt.Println(dim.Render("    · " + d))
		}
		fmt.Println()
	}

	var confirmed bool
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("确认删除 Provider %q？", name)).
			Description("删除后相关 NamedAgent 的模型引用将被清空，主/子 Agent 的引用将在下一步重新选择。").
			Value(&confirmed),
	))
	if err := form.Run(); err != nil {
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
			p, err := editProvider(config.ProviderConfig{})
			if err != nil {
				return err
			}
			fullCfg.Providers = append(fullCfg.Providers, p)
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
						updated, err := editProvider(p)
						if err != nil {
							return err
						}
						fullCfg.Providers[i] = updated
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
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider 管理").
			Description("选择要操作的 Provider，或添加新的").
			Options(opts...).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}

// pickProviderItemAction 弹出 Provider 二级操作菜单
func pickProviderItemAction(name string) (string, error) {
	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("Provider: %s", name)).
			Options(
				huh.NewOption("编辑", "__edit__"),
				huh.NewOption("删除", "__delete__"),
				huh.NewOption("← 返回", "__back__"),
			).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}

// chineseKeyMap 返回带中文说明的 KeyMap，仅覆盖 MultiSelect 和 Select 的提示文字
func chineseKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.MultiSelect.Toggle = key.NewBinding(key.WithKeys(" ", "x"), key.WithHelp("x", "切换选中"))
	km.MultiSelect.Up = key.NewBinding(key.WithKeys("up", "k", "ctrl+p"), key.WithHelp("↑", "向上"))
	km.MultiSelect.Down = key.NewBinding(key.WithKeys("down", "j", "ctrl+n"), key.WithHelp("↓", "向下"))
	km.MultiSelect.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "过滤"))
	km.MultiSelect.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "返回"))
	km.MultiSelect.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.MultiSelect.SelectAll = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "全选"))
	km.MultiSelect.SelectNone = key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "取消全选"), key.WithDisabled())
	km.Select.Up = key.NewBinding(key.WithKeys("up", "k", "ctrl+k", "ctrl+p"), key.WithHelp("↑", "向上"))
	km.Select.Down = key.NewBinding(key.WithKeys("down", "j", "ctrl+j", "ctrl+n"), key.WithHelp("↓", "向下"))
	km.Select.Filter = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "过滤"))
	km.Select.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "返回"))
	km.Select.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	return km
}

var apiFormatOpts = []huh.Option[string]{
	huh.NewOption("openai-completions  (GPT / 通用兼容)", "openai-completions"),
	huh.NewOption("anthropic-messages  (Claude)", "anthropic-messages"),
	huh.NewOption("openai-responses    (GPT-5 / o 系列)", "openai-responses"),
	huh.NewOption("google-generative-ai (Gemini)", "google-generative-ai"),
}

func editProvider(p config.ProviderConfig) (config.ProviderConfig, error) {
	name := p.Name
	baseUrl := p.BaseUrl
	apiKey := p.ApiKey
	apiFormat := p.ApiFormat
	if apiFormat == "" {
		apiFormat = "openai-completions"
	}

	// 构建预设模型集合，用于快速判断自定义模型
	presetSet := make(map[string]bool, len(models.PresetModels))
	for _, m := range models.PresetModels {
		presetSet[m] = true
	}

	// 构建 MultiSelect 选项：预设 + 已有自定义模型 + "自定义..."
	presetOpts := make([]huh.Option[string], 0, len(models.PresetModels)+1)
	for _, m := range models.PresetModels {
		presetOpts = append(presetOpts, huh.NewOption(m, m))
	}
	for _, m := range p.Models {
		if !presetSet[m] {
			presetOpts = append(presetOpts, huh.NewOption(m+" (自定义)", m))
		}
	}
	presetOpts = append(presetOpts, huh.NewOption("自定义...", "__custom__"))

	selectedModels := make([]string, 0)
	for _, m := range p.Models {
		selectedModels = append(selectedModels, m)
	}

	customModel := ""

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
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
			Value(&name),
		huh.NewInput().
			Title("Base URL").
			Placeholder("https://www.dmxapi.cn/v1").
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
			Value(&baseUrl),
		huh.NewInput().
			Title("API Key").
			Placeholder("sk-...").
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("API Key 不能为空")
				}
				return nil
			}).
			Value(&apiKey),
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description("选择此 provider 支持的模型（至少一项）；可同时选「自定义...」再在下方填入自定义名称").
			Options(presetOpts...).
			Validate(func(selected []string) error {
				// 只检查完全未选的情况；选了 __custom__ 放行，由 form.Run() 后的逻辑做最终校验
				if len(selected) == 0 {
					return fmt.Errorf("请至少选择一个模型")
				}
				return nil
			}).
			Value(&selectedModels),
		huh.NewInput().
			Title("自定义模型名称（可选，留空跳过）").
			Placeholder("my-custom-model").
			Value(&customModel),
		huh.NewSelect[string]().
			Title("API 格式").
			Description("不确定时选 openai-completions。若同一 provider 含不同格式模型，请分拆为多个 provider").
			Options(apiFormatOpts...).
			Value(&apiFormat),
	)).WithKeyMap(chineseKeyMap())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.ProviderConfig{}, err
	}

	// 整理模型列表
	finalModels := []string{}
	for _, m := range selectedModels {
		if m != "__custom__" {
			finalModels = append(finalModels, m)
		}
	}
	if ct := strings.TrimSpace(customModel); ct != "" {
		finalModels = append(finalModels, ct)
	}
	// 若没有任何选择，返回错误
	if len(finalModels) == 0 {
		return config.ProviderConfig{}, fmt.Errorf("provider %q 的模型列表不能为空", name)
	}

	// 若 apiFormat 为空，根据第一个模型自动推断
	if apiFormat == "" {
		apiFormat = config.DetectAPIFormat(finalModels[0])
	}

	return config.ProviderConfig{
		Name:      strings.TrimSpace(name),
		BaseUrl:   strings.TrimSpace(baseUrl),
		ApiKey:    strings.TrimSpace(apiKey),
		Models:    finalModels,
		ApiFormat: apiFormat,
	}, nil
}

// ── Step 2: 主 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep2MainAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) error {
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

	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("主 Agent 模型 (Primary)").
			Description("agents.defaults.model.primary").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("主 Agent 备用模型 (Fallback)").
			Description("可选，留空表示不配置备用模型").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}
	fullCfg.MainAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return nil
}

// ── Step 3: 子 Agent 模型 ──────────────────────────────────────────────────

func (a *App) runStep3SubAgent(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) error {
	const sameAsMain = "__same__"
	subChoice := sameAsMain
	if fullCfg.SubAgent.Primary != "" {
		subChoice = "__custom__"
	}

	form1 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", "__custom__"),
			).
			Value(&subChoice),
	))
	if err := form1.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}

	if subChoice == sameAsMain {
		fullCfg.SubAgent = config.AgentModelConfig{}
		return nil
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

	form2 := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 主模型 (Primary)").
			Options(allOpts...).
			Value(&primary),
		huh.NewSelect[string]().
			Title("子 Agent 备用模型 (Fallback)").
			Options(allOptsWithNone...).
			Value(&fallback),
	))
	if err := form2.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return err
	}
	fullCfg.SubAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return nil
}

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
	opts = append(opts, huh.NewOption("[+ 添加新命名 Agent]", "__add__"))
	if len(agents) > 0 {
		opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))
	}

	var selected string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("命名 Agent 管理").
			Description("为特定 agent id 指定不同模型").
			Options(opts...).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
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
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title(fmt.Sprintf("命名 Agent: %s", id)).
			Options(
				huh.NewOption("编辑", "__edit__"),
				huh.NewOption("删除", "__delete__"),
				huh.NewOption("← 返回", "__back__"),
			).
			Value(&selected),
	))
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return "", err
	}
	return selected, nil
}

// editNamedAgent 编辑已有命名 Agent（Agent ID 只读）
// 注：__add__ 分支目前只收集 Primary（与现有行为一致），editNamedAgent 支持 Primary + Fallback。
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, error) {
	primary := agent.Model.Primary
	fallback := agent.Model.Fallback

	form := huh.NewForm(huh.NewGroup(
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
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintln(os.Stderr, "已取消")
			os.Exit(0)
		}
		return config.NamedAgentConfig{}, err
	}
	return config.NamedAgentConfig{
		ID:    agent.ID,
		Model: config.AgentModelConfig{Primary: primary, Fallback: fallback},
	}, nil
}

// ── Step 4: 命名 Agent（可选） ─────────────────────────────────────────────
func (a *App) runStep4NamedAgents(
	fullCfg *config.FullConfig,
	allOpts []huh.Option[string],
) error {
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
			return err
		}

		switch action {
		case "__continue__":
			return nil

		case "__add__":
			agentID := ""
			modelPrimary := ""
			if len(allOpts) > 0 {
				modelPrimary = allOpts[0].Value
			}
			form := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Agent ID").
					Placeholder("my-coder").
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("Agent ID 不能为空")
						}
						return nil
					}).
					Value(&agentID),
				huh.NewSelect[string]().
					Title("使用模型").
					Options(allOptsWithSame...).
					Value(&modelPrimary),
			))
			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Fprintln(os.Stderr, "已取消")
					os.Exit(0)
				}
				return err
			}
			id := strings.TrimSpace(agentID)
			upserted := false
			for i, na := range fullCfg.NamedAgents {
				if na.ID == id {
					fullCfg.NamedAgents[i] = config.NamedAgentConfig{
						ID:    id,
						Model: config.AgentModelConfig{Primary: modelPrimary},
					}
					upserted = true
					break
				}
			}
			if !upserted {
				fullCfg.NamedAgents = append(fullCfg.NamedAgents, config.NamedAgentConfig{
					ID:    id,
					Model: config.AgentModelConfig{Primary: modelPrimary},
				})
			}

		default:
			// 选中已有 Agent → 二级菜单
			subAction, err := pickNamedAgentItemAction(action)
			if err != nil {
				return err
			}
			switch subAction {
			case "__edit__":
				for i, na := range fullCfg.NamedAgents {
					if na.ID == action {
						updated, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return err
						}
						fullCfg.NamedAgents[i] = updated
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

// ── 输出 ──────────────────────────────────────────────────────────────────

func printSuccess(cfg *config.FullConfig) {
	green := lipgloss.Color("42")

	var lines []string
	for _, p := range cfg.Providers {
		lines = append(lines, fmt.Sprintf("  Provider  : %s  (%s)  [%s]", p.Name, p.BaseUrl, p.ApiFormat))
		lines = append(lines, fmt.Sprintf("  模型      : %s", strings.Join(p.Models, ", ")))
		lines = append(lines, "")
	}
	lines = append(lines, fmt.Sprintf("  主 Agent  : %s", cfg.MainAgent.Primary))
	if cfg.SubAgent.Primary != "" {
		lines = append(lines, fmt.Sprintf("  子 Agent  : %s", cfg.SubAgent.Primary))
	}
	if len(cfg.NamedAgents) > 0 {
		for _, na := range cfg.NamedAgents {
			lines = append(lines, fmt.Sprintf("  Named [%s]: %s", na.ID, na.Model.Primary))
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(green).
		Padding(1, 2).
		Render("✓ 配置已保存！\n\n" + strings.Join(lines, "\n"))

	fmt.Println()
	fmt.Println(box)

	huh.NewForm(huh.NewGroup( //nolint:errcheck
		huh.NewNote().
			Title("提示").
			Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
			Next(true).
			NextLabel("按 Enter 退出"),
	)).Run() //nolint:errcheck
}

func printBanner() {
	purple := lipgloss.Color("63")
	gray := lipgloss.Color("240")

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
		Render("  多 Provider · 多模型 · 主/子/命名 Agent 独立配置")

	fmt.Println(logo)
	fmt.Println()
	fmt.Println(subtitle)
	fmt.Println(sep)
	fmt.Println(note)
	fmt.Println()
}
