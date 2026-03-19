package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

	// Step 1-4: 步骤导航循环（支持后退）
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
	if err := runForm(form); err != nil {
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
	form := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Provider 管理").
			Description("选择要操作的 Provider，或添加新的").
			Options(opts...).
			Value(&selected),
	))
	if err := runForm(form); err != nil {
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
	if err := runForm(form); err != nil {
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
	km.Input.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "下一项"))
	km.Input.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "上一项"))
	km.Confirm.Next = key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter", "确认"))
	km.Confirm.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "上一项"))
	km.Note.Next = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "继续"))
	km.Note.Prev = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "返回"))
	km.Input.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "提交"))
	km.Select.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.MultiSelect.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	km.Note.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "继续"))
	km.Confirm.Submit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "确认"))
	return km
}

func renderHelpFooter(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("  " + text)
}

const defaultHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认"

const providerModelListFocusedHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认"

func providerEditorHelpFooter(modelListFocused bool) string {
	if modelListFocused {
		return renderHelpFooter(providerModelListFocusedHelpFooterText)
	}
	return renderHelpFooter(defaultHelpFooterText)
}

// helpFooter 是统一显示在所有表单底部的导航提示栏
var helpFooter = renderHelpFooter(defaultHelpFooterText)

// formModel 将 huh.Form 包裹为 tea.Model，在 View() 底部追加固定帮助栏。
// 替代 form.Run()，配合 runForm() 使用。
type formModel struct {
	form           *huh.Form
	helpFooterView func() string
}

func (m *formModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// huh.Form.Update 返回新的 tea.Model，必须类型断言并回写 m.form，
	// 否则表单状态永远不会推进。bubbletea 事件循环为单线程，无竞态风险。
	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	return m, cmd
}

func (m *formModel) View() string {
	v := m.form.View()
	if v == "" {
		return "" // form.go:610：quitting 时 View() 确定返回空字符串
	}
	if m.helpFooterView != nil {
		if footer := m.helpFooterView(); footer != "" {
			return v + "\n" + footer
		}
	}
	return v + "\n" + helpFooter
}

func prepareFormForRun(form *huh.Form) *huh.Form {
	form.WithShowHelp(false)
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Quit
	return form
}

// runForm 替代 form.Run()，通过 bubbletea 程序展示带固定帮助栏的表单。
func runForm(form *huh.Form) error {
	return runFormWithFooter(form, nil)
}

func runFormWithFooter(form *huh.Form, helpFooterView func() string) error {
	form = prepareFormForRun(form)
	m := &formModel{form: form, helpFooterView: helpFooterView}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if result.(*formModel).form.State == huh.StateAborted {
		return huh.ErrUserAborted
	}
	return nil
}

// newForm 创建带中文 KeyMap 的 huh.Form（统一入口，避免重复调用 WithKeyMap）
func newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithKeyMap(chineseKeyMap()).WithShowHelp(false)
}

func presetModelSet() map[string]bool {
	set := make(map[string]bool, len(models.PresetModels))
	for _, m := range models.PresetModels {
		set[m] = true
	}
	return set
}

func splitProviderModelsForEdit(providerModels []string) ([]string, []string) {
	presetSet := presetModelSet()
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
	presetSet := presetModelSet()
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

func providerModelListTheme() *huh.Theme {
	theme := huh.ThemeCharm()
	theme.Focused.SelectedPrefix = lipgloss.NewStyle().SetString("[✓] ")
	theme.Focused.UnselectedPrefix = lipgloss.NewStyle().SetString("[ ] ")
	theme.Blurred.SelectedPrefix = lipgloss.NewStyle().SetString("[✓] ")
	theme.Blurred.UnselectedPrefix = lipgloss.NewStyle().SetString("[ ] ")
	return theme
}

const providerModelListBaseDescription = "选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个"

const (
	providerModelListTitleLines           = 1
	providerModelListBaseDescriptionLines = 1
	providerModelListOverflowLines        = 1
)

type providerModelListPresentation struct {
	fieldHeight      int
	visibleRows      int
	hiddenCount      int
	showOverflowHint bool
}

func computeProviderModelListPresentation(availableFieldHeight int, optionCount int) providerModelListPresentation {
	minFieldHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	if optionCount <= 0 {
		return providerModelListPresentation{
			fieldHeight:      minFieldHeight - providerModelListOverflowLines,
			visibleRows:      1,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	fullFieldHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + optionCount
	if availableFieldHeight >= fullFieldHeight {
		return providerModelListPresentation{
			fieldHeight:      fullFieldHeight,
			visibleRows:      optionCount,
			hiddenCount:      0,
			showOverflowHint: false,
		}
	}

	budgetWithHint := max(minFieldHeight, availableFieldHeight)
	visibleRows := max(1, budgetWithHint-providerModelListTitleLines-providerModelListBaseDescriptionLines-providerModelListOverflowLines)
	return providerModelListPresentation{
		fieldHeight:      providerModelListTitleLines + providerModelListBaseDescriptionLines + visibleRows,
		visibleRows:      visibleRows,
		hiddenCount:      max(0, optionCount-visibleRows),
		showOverflowHint: true,
	}
}

func providerModelListOverflowHint(p providerModelListPresentation) string {
	if !p.showOverflowHint || p.hiddenCount <= 0 {
		return ""
	}
	return fmt.Sprintf("↓ 更多模型（还有 %d 项，继续向下查看）", p.hiddenCount)
}

const providerFieldGapLines = 2

func providerModelListAvailableFieldHeight(windowHeight int, otherViews ...string) int {
	reserved := lipgloss.Height(helpFooter)
	for _, view := range otherViews {
		reserved += lipgloss.Height(view)
	}
	reserved += len(otherViews) * providerFieldGapLines

	minFieldHeight := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	return max(minFieldHeight, windowHeight-reserved)
}

type providerModelListField struct {
	field            *huh.MultiSelect[string]
	measureAvailable func(int) int
	optionCount      func() int
	lastWindowHeight int
	presentation     providerModelListPresentation
	focused          bool
}

func newProviderModelListField(
	field *huh.MultiSelect[string],
	measureAvailable func(int) int,
	optionCount func() int,
) *providerModelListField {
	return &providerModelListField{
		field:            field,
		measureAvailable: measureAvailable,
		optionCount:      optionCount,
	}
}

func (f *providerModelListField) syncPresentation(windowHeight int) {
	available := f.measureAvailable(windowHeight)
	presentation := computeProviderModelListPresentation(available, f.optionCount())
	if presentation == f.presentation {
		return
	}
	f.presentation = presentation
	f.field.Description(providerModelListBaseDescription)
	f.field.Height(presentation.fieldHeight)
}

func (f *providerModelListField) Init() tea.Cmd { return f.field.Init() }

func (f *providerModelListField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.lastWindowHeight = windowMsg.Height
	}
	if f.lastWindowHeight > 0 {
		f.syncPresentation(f.lastWindowHeight)
	}
	m, cmd := f.field.Update(msg)
	f.field = m.(*huh.MultiSelect[string])
	return f, cmd
}

func (f *providerModelListField) View() string {
	if f.lastWindowHeight > 0 {
		f.syncPresentation(f.lastWindowHeight)
	}
	view := f.field.View()
	if hint := providerModelListOverflowHint(f.presentation); hint != "" {
		return view + "\n" + hint
	}
	return view
}

func (f *providerModelListField) Blur() tea.Cmd {
	f.focused = false
	return f.field.Blur()
}

func (f *providerModelListField) Focus() tea.Cmd {
	f.focused = true
	return f.field.Focus()
}

func (f *providerModelListField) IsFocused() bool { return f.focused }
func (f *providerModelListField) Error() error    { return f.field.Error() }
func (f *providerModelListField) Run() error      { return f.field.Run() }
func (f *providerModelListField) Skip() bool      { return f.field.Skip() }
func (f *providerModelListField) Zoom() bool      { return f.field.Zoom() }
func (f *providerModelListField) KeyBinds() []key.Binding { return f.field.KeyBinds() }

func (f *providerModelListField) WithTheme(theme *huh.Theme) huh.Field {
	f.field = f.field.WithTheme(theme).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) WithAccessible(accessible bool) huh.Field {
	f.field = f.field.WithAccessible(accessible).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) WithKeyMap(k *huh.KeyMap) huh.Field {
	f.field = f.field.WithKeyMap(k).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) WithWidth(width int) huh.Field {
	f.field = f.field.WithWidth(width).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) WithHeight(height int) huh.Field {
	f.field = f.field.WithHeight(height).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) WithPosition(position huh.FieldPosition) huh.Field {
	f.field = f.field.WithPosition(position).(*huh.MultiSelect[string])
	return f
}

func (f *providerModelListField) GetKey() string { return f.field.GetKey() }
func (f *providerModelListField) GetValue() any  { return f.field.GetValue() }

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

type providerCustomRegistration struct {
	handled bool
	added   bool
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

type providerCustomModelInput struct {
	input    *huh.Input
	accessor *huh.PointerAccessor[string]
	keymap   *huh.KeyMap
	onSubmit func(string) providerCustomRegistration
}

func newProviderCustomModelInput(value *string, onSubmit func(string) providerCustomRegistration) *providerCustomModelInput {
	accessor := huh.NewPointerAccessor(value)
	return &providerCustomModelInput{
		input: huh.NewInput().
			Title("自定义模型名称（可选，多个可用逗号/换行分隔）").
			Placeholder("my-custom-model-a, my-custom-model-b").
			Accessor(accessor),
		accessor: accessor,
		onSubmit: onSubmit,
	}
}

func (f *providerCustomModelInput) Init() tea.Cmd { return f.input.Init() }

func (f *providerCustomModelInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && f.keymap != nil {
		if key.Matches(keyMsg, f.keymap.Input.Next, f.keymap.Input.Submit) {
			result := f.onSubmit(f.accessor.Get())
			if result.handled {
				f.accessor.Set("")
				f.input.Accessor(f.accessor)
				return f, huh.PrevField
			}
		}
	}
	m, cmd := f.input.Update(msg)
	f.input = m.(*huh.Input)
	return f, cmd
}

func (f *providerCustomModelInput) View() string            { return f.input.View() }
func (f *providerCustomModelInput) Blur() tea.Cmd           { return f.input.Blur() }
func (f *providerCustomModelInput) Focus() tea.Cmd          { return f.input.Focus() }
func (f *providerCustomModelInput) Error() error            { return f.input.Error() }
func (f *providerCustomModelInput) Run() error              { return f.input.Run() }
func (f *providerCustomModelInput) Skip() bool              { return f.input.Skip() }
func (f *providerCustomModelInput) Zoom() bool              { return f.input.Zoom() }
func (f *providerCustomModelInput) KeyBinds() []key.Binding { return f.input.KeyBinds() }

func (f *providerCustomModelInput) WithTheme(theme *huh.Theme) huh.Field {
	f.input = f.input.WithTheme(theme).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) WithAccessible(accessible bool) huh.Field {
	f.input = f.input.WithAccessible(accessible).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) WithKeyMap(k *huh.KeyMap) huh.Field {
	f.keymap = k
	f.input = f.input.WithKeyMap(k).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) WithWidth(width int) huh.Field {
	f.input = f.input.WithWidth(width).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) WithHeight(height int) huh.Field {
	f.input = f.input.WithHeight(height).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) WithPosition(position huh.FieldPosition) huh.Field {
	f.input = f.input.WithPosition(position).(*huh.Input)
	return f
}

func (f *providerCustomModelInput) GetKey() string { return f.input.GetKey() }
func (f *providerCustomModelInput) GetValue() any  { return f.input.GetValue() }

func appendUniqueStrings(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func editProvider(p config.ProviderConfig) (config.ProviderConfig, bool, error) {
	name := p.Name
	// 格式感知的展示剥离：仅对自动补全 /v1 的格式剥除，google-generative-ai 原样
	baseUrl := p.BaseUrl
	if p.ApiFormat != "google-generative-ai" {
		baseUrl = strings.TrimSuffix(strings.TrimRight(p.BaseUrl, "/"), "/v1")
	}
	apiKey := p.ApiKey

	selectedModels, customModelRegistry := splitProviderModelsForEdit(p.Models)
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

	customModelField := newProviderCustomModelInput(
		&customModelInput,
		func(raw string) providerCustomRegistration {
			parsed := parseCustomModelInput(raw)
			if len(parsed) == 0 {
				return providerCustomRegistration{}
			}
			var added []string
			selectedModels, customModelRegistry, added = registerProviderCustomModels(selectedModels, customModelRegistry, raw)
			result := providerCustomRegistration{
				handled: true,
				added:   len(added) > 0,
			}
			if result.added {
				modelOptionsVersion++
			}
			return result
		},
	)

	modelListField := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
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
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(windowHeight int) int {
			return providerModelListAvailableFieldHeight(
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

	form := newForm(huh.NewGroup(
		nameField,
		baseURLField,
		apiKeyField,
		modelListField,
		customModelField,
	))

	if err := runFormWithFooter(form, func() string {
		return providerEditorHelpFooter(modelListField.IsFocused())
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
	if err := runForm(form); err != nil {
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
	if err := runForm(form1); err != nil {
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
	if err := runForm(form2); err != nil {
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
	if err := runForm(form); err != nil {
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
	if err := runForm(form); err != nil {
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
	if err := runForm(form); err != nil {
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

	f := newForm(huh.NewGroup(
		huh.NewNote().
			Title("提示").
			Description("✓ 配置已保存，下次请求时自动生效（支持热切换，无需重启网关）。").
			Next(true).
			NextLabel("按 Enter 退出"),
	))
	runForm(f) //nolint:errcheck
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
