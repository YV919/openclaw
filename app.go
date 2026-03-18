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
	form := newForm(huh.NewGroup(
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
	form := newForm(huh.NewGroup(
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
	form := newForm(huh.NewGroup(
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

// chineseKeyMap 返回带中文说明的 KeyMap，覆盖所有字段类型（Input/Confirm/Note/Select/MultiSelect）的导航提示
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
	km.Input.Next   = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
	km.Input.Prev   = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"),    key.WithHelp("shift+tab", "上一项"))
	km.Note.Next    = key.NewBinding(key.WithKeys("enter"),        key.WithHelp("enter", "继续"))
	return km
}

// newForm 创建带中文 KeyMap 的 huh.Form（统一入口，避免重复调用 WithKeyMap）
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithKeyMap(chineseKeyMap())
}

func editProvider(p config.ProviderConfig) (config.ProviderConfig, error) {
	name := p.Name
	// 格式感知的展示剥离：仅对自动补全 /v1 的格式剥除，google-generative-ai 原样
	baseUrl := p.BaseUrl
	if p.ApiFormat != "google-generative-ai" {
		baseUrl = strings.TrimSuffix(strings.TrimRight(p.BaseUrl, "/"), "/v1")
	}
	apiKey := p.ApiKey

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

	form := newForm(huh.NewGroup(
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
			Description("空格/x 切换选中，↑↓ 移动，enter 确认\n选择此 provider 支持的模型（至少一项）；可同时选「自定义...」再在下方填入自定义名称").
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
	))

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

	// 先确定 apiFormat，再规范化 URL（NormalizeBaseURL 依赖 apiFormat）
	apiFormat := detectFormatFromModels(finalModels)
	baseUrl = config.NormalizeBaseURL(strings.TrimSpace(baseUrl), apiFormat)

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

	form := newForm(huh.NewGroup(
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
	if err := form.Run(); err != nil {
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

	form1 := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("子 Agent 模型 (subagents)").
			Options(
				huh.NewOption("同主 Agent（不单独配置）", sameAsMain),
				huh.NewOption("单独指定", "__custom__"),
				huh.NewOption("← 返回上一步", "__back__"),
			).
			Value(&subChoice),
	))
	if err := form1.Run(); err != nil {
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

	form2 := newForm(huh.NewGroup(
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
		return false, err
	}
	fullCfg.SubAgent = config.AgentModelConfig{Primary: primary, Fallback: fallback}
	return false, nil
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
	opts = append(opts, huh.NewOption("← 返回上一步", "__back__"))
	opts = append(opts, huh.NewOption("[继续 →]", "__continue__"))

	var selected string
	form := newForm(huh.NewGroup(
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
	form := newForm(huh.NewGroup(
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

// editNamedAgent 编辑已有命名 Agent（Agent ID 只读），支持 Primary + Fallback。
func editNamedAgent(
	agent config.NamedAgentConfig,
	allOptsWithSame []huh.Option[string],
	allOptsWithNone []huh.Option[string],
) (config.NamedAgentConfig, error) {
	primary := agent.Model.Primary
	fallback := agent.Model.Fallback

	form := newForm(huh.NewGroup(
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
						updated, err := editNamedAgent(na, allOptsWithSame, allOptsWithNone)
						if err != nil {
							return false, err
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

	newForm(huh.NewGroup( //nolint:errcheck
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

// detectFormatFromModels 根据模型列表自动推断 API 格式。
// 若所有模型格式一致（含单模型情形），直接返回该格式。
// 若存在冲突，打印黄色警告并返回第一个模型的格式。
// 调用方保证 models 非空。
func detectFormatFromModels(models []string) string {
	seen := make(map[string]bool, len(models))
	for _, m := range models {
		seen[config.DetectAPIFormat(m)] = true
	}
	if len(seen) == 1 {
		for f := range seen {
			return f
		}
	}
	// 存在冲突：打印警告，返回第一个模型的格式
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	fmt.Println(yellow.Render("  ⚠ 所选模型包含不同 API 格式，将使用第一个模型的格式："))
	for _, m := range models {
		fmt.Println(dim.Render(fmt.Sprintf("    · %s → %s", m, config.DetectAPIFormat(m))))
	}
	fmt.Println(dim.Render("  建议：将不同格式的模型拆分为独立 provider"))
	fmt.Println()
	return config.DetectAPIFormat(models[0])
}
