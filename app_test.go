package main

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// suppressStdout 将 os.Stdout 重定向到 pipe，用于抑制测试中的警告输出
func suppressStdout(f func()) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	io.ReadAll(r) //nolint:errcheck
}

func optionValues(opts []huh.Option[string]) []string {
	values := make([]string, 0, len(opts))
	for _, opt := range opts {
		values = append(values, opt.Value)
	}
	return values
}

func TestProviderManagementDescriptionUsesFormatReminderCopy(t *testing.T) {
	got := ui.ProviderManagementDescription()
	want := "同一 Provider 只配置一种模型格式。\nOpenAI 兼容、GPT-5 系列、Anthropic、Gemini 请分开配置。"

	if got != want {
		t.Fatalf("ui.ProviderManagementDescription() = %q, want %q", got, want)
	}
}

func TestSavedConfigNoticeDescriptionMatchesHybridReloadGuidance(t *testing.T) {
	got := ui.SavedConfigNoticeDescription()
	want := "✓ 配置已保存。默认在 OpenClaw 的 hybrid reload 下，大多数 agent/model 变更会自动生效。\n若修改 gateway/plugins/discovery/canvasHost，请执行 openclaw gateway restart。"

	if got != want {
		t.Fatalf("ui.SavedConfigNoticeDescription() = %q, want %q", got, want)
	}
}

func TestSuccessDismissFieldDoesNotSkip(t *testing.T) {
	field := ui.NewSuccessDismissField("测试说明")

	if field.Skip() {
		t.Fatal("success dismiss field should block until Enter")
	}
}

func TestSuccessDismissFieldIgnoresNonSubmitKeys(t *testing.T) {
	field := ui.NewSuccessDismissField("测试说明")

	_, cmd := field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Fatal("non-submit key should not advance success dismiss field")
	}
}

func TestDetectFormatFromModels(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{"claude 前缀", []string{"claude-sonnet-4-6"}, "anthropic-messages"},
		{"-cc 后缀", []string{"MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"gemini 前缀", []string{"gemini-3.1-pro-preview"}, "google-generative-ai"},
		{"gpt-5 前缀", []string{"gpt-5.2"}, "openai-responses"},
		{"gpt-5 带后缀变体", []string{"gpt-5.3-codex"}, "openai-responses"},
		{"o1 裸前缀", []string{"o1"}, "openai-responses"},
		{"o3-mini 带连字符", []string{"o3-mini"}, "openai-responses"},
		{"默认 openai-completions", []string{"qwen-turbo"}, "openai-completions"},
		{"单模型无冲突路径", []string{"claude-opus-4-6"}, "anthropic-messages"},
		{"多模型一致格式", []string{"claude-sonnet-4-6", "MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"冲突时取第一个", []string{"claude-sonnet-4-6", "gpt-5.2"}, "anthropic-messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			suppressStdout(func() {
				got = detectFormatFromModels(tt.models)
			})
			if got != tt.expected {
				t.Errorf("detectFormatFromModels(%v) = %q, want %q", tt.models, got, tt.expected)
			}
		})
	}
}

func TestPrepareFormForRunSetsQuitCommands(t *testing.T) {
	form := ui.PrepareFormForRun(ui.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("提示").
			Description("测试表单").
			Next(true),
	)))

	if _, ok := form.SubmitCmd().(tea.QuitMsg); !ok {
		t.Fatalf("SubmitCmd should emit tea.QuitMsg")
	}
	if _, ok := form.CancelCmd().(tea.QuitMsg); !ok {
		t.Fatalf("CancelCmd should emit tea.QuitMsg")
	}
}

func TestProviderModelSplitSeparatesPresetAndCustomRegistry(t *testing.T) {
	selected, customRegistry := splitProviderModelsForEdit([]string{
		"claude-opus-4-6",
		"custom-alpha",
		"gpt-5.2",
		"custom-beta",
	})

	if !reflect.DeepEqual(selected, []string{"claude-opus-4-6", "custom-alpha", "gpt-5.2", "custom-beta"}) {
		t.Fatalf("selected = %v, want %v", selected, []string{"claude-opus-4-6", "custom-alpha", "gpt-5.2", "custom-beta"})
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha", "custom-beta"}) {
		t.Fatalf("customRegistry = %v, want %v", customRegistry, []string{"custom-alpha", "custom-beta"})
	}
}

func TestParseCustomModelInputSupportsMultipleEntries(t *testing.T) {
	got := parseCustomModelInput("custom-alpha, custom-beta\ncustom-gamma，custom-beta ; custom-delta")
	want := []string{"custom-alpha", "custom-beta", "custom-gamma", "custom-delta"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCustomModelInput() = %v, want %v", got, want)
	}
}

func TestProviderModelOptionsOrderPresetBeforeCustom(t *testing.T) {
	opts := buildProviderModelOptions(
		[]string{"claude-opus-4-6", "custom-beta"},
		[]string{"custom-alpha", "custom-beta"},
	)

	values := optionValues(opts)
	if len(values) < 2 {
		t.Fatalf("option count = %d, want at least 2", len(values))
	}
	gotTail := values[len(values)-2:]
	wantTail := []string{"custom-alpha", "custom-beta"}
	if !reflect.DeepEqual(gotTail, wantTail) {
		t.Fatalf("tail values = %v, want %v", gotTail, wantTail)
	}
}

func TestProviderModelOptionsSkipPresetNamedCustoms(t *testing.T) {
	opts := buildProviderModelOptions(
		[]string{"gpt-5.2"},
		[]string{"gpt-5.2", "custom-alpha"},
	)

	values := optionValues(opts)
	count := 0
	for _, value := range values {
		if value == "gpt-5.2" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("gpt-5.2 count = %d, want 1", count)
	}
}

func TestProviderModelFinalSelectionIsOnlySaveSource(t *testing.T) {
	got := finalProviderModels([]string{"claude-opus-4-6", "custom-alpha", "custom-alpha", ""})
	want := []string{"claude-opus-4-6", "custom-alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("finalProviderModels() = %v, want %v", got, want)
	}
}

func TestProviderModelListThemeUsesAsciiCheckboxPrefixes(t *testing.T) {
	theme := ui.ProviderModelListTheme()
	if theme.Focused.SelectedPrefix.String() != "[✓] " {
		t.Fatalf("focused selected prefix = %q, want %q", theme.Focused.SelectedPrefix.String(), "[✓] ")
	}
	if theme.Focused.UnselectedPrefix.String() != "[ ] " {
		t.Fatalf("focused unselected prefix = %q, want %q", theme.Focused.UnselectedPrefix.String(), "[ ] ")
	}
	if theme.Blurred.SelectedPrefix.String() != "[✓] " {
		t.Fatalf("blurred selected prefix = %q, want %q", theme.Blurred.SelectedPrefix.String(), "[✓] ")
	}
	if theme.Blurred.UnselectedPrefix.String() != "[ ] " {
		t.Fatalf("blurred unselected prefix = %q, want %q", theme.Blurred.UnselectedPrefix.String(), "[ ] ")
	}
}

func TestComputeProviderModelListPresentationShowsAllOptionsWhenSpaceAllows(t *testing.T) {
	got := ui.ComputeProviderModelListPresentation(12, 4)
	want := ui.ProviderModelListPresentation{
		FieldHeight:      6,
		VisibleRows:      4,
		HiddenCount:      0,
		ShowOverflowHint: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestComputeProviderModelListPresentationReservesOverflowHintLine(t *testing.T) {
	got := ui.ComputeProviderModelListPresentation(8, 10)
	want := ui.ProviderModelListPresentation{
		FieldHeight:      7,
		VisibleRows:      5,
		HiddenCount:      5,
		ShowOverflowHint: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestProviderModelListOverflowHintWithOverflowShowsRemainingCount(t *testing.T) {
	got := ui.ProviderModelListOverflowHint(ui.ProviderModelListPresentation{
		FieldHeight:      7,
		VisibleRows:      5,
		HiddenCount:      5,
		ShowOverflowHint: true,
	})
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if got != want {
		t.Fatalf("overflow hint = %q, want %q", got, want)
	}
}

func TestProviderModelListOverflowHintWithoutOverflowReturnsEmpty(t *testing.T) {
	got := ui.ProviderModelListOverflowHint(ui.ProviderModelListPresentation{
		FieldHeight:      6,
		VisibleRows:      4,
		HiddenCount:      0,
		ShowOverflowHint: false,
	})
	if got != "" {
		t.Fatalf("overflow hint = %q, want empty", got)
	}
}

func TestPrepareQuickSetupBacksUpProvidersMainSubAndNamedAgents(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}},
			{Name: "anthropic", BaseUrl: "https://api.anthropic.com", Models: []string{"claude-sonnet-4-6"}},
		},
		MainAgent: config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "anthropic/claude-sonnet-4-6"},
		SubAgent:  config.AgentModelConfig{Primary: "anthropic/claude-sonnet-4-6"},
		NamedAgents: []config.NamedAgentConfig{{
			ID:    "writer",
			Model: config.AgentModelConfig{Primary: "openai/gpt-5"},
		}},
	}

	snapshot := prepareQuickSetup(cfg)

	if !reflect.DeepEqual(snapshot.Providers, []config.ProviderConfig{
		{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}},
		{Name: "anthropic", BaseUrl: "https://api.anthropic.com", Models: []string{"claude-sonnet-4-6"}},
	}) {
		t.Fatalf("snapshot providers = %#v", snapshot.Providers)
	}
	if snapshot.MainAgent != (config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "anthropic/claude-sonnet-4-6"}) {
		t.Fatalf("snapshot main = %#v", snapshot.MainAgent)
	}
	if snapshot.SubAgent != (config.AgentModelConfig{Primary: "anthropic/claude-sonnet-4-6"}) {
		t.Fatalf("snapshot sub = %#v", snapshot.SubAgent)
	}
	if len(snapshot.NamedAgents) != 1 || snapshot.NamedAgents[0].ID != "writer" {
		t.Fatalf("snapshot named agents = %#v", snapshot.NamedAgents)
	}

	if cfg.SubAgent != (config.AgentModelConfig{}) {
		t.Fatalf("sub agent = %#v, want empty", cfg.SubAgent)
	}
	if cfg.NamedAgents != nil {
		t.Fatalf("named agents = %#v, want nil", cfg.NamedAgents)
	}
}

func TestRestoreQuickSetupSnapshotRestoresProvidersMainSubAndNamedAgents(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{{Name: "replacement", Models: []string{"replacement-model"}}},
		MainAgent: config.AgentModelConfig{Primary: "replacement/replacement-model"},
	}
	snapshot := quickSetupSnapshot{
		Providers: []config.ProviderConfig{{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}}},
		MainAgent: config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "openai/gpt-4.1"},
		SubAgent:  config.AgentModelConfig{Primary: "openai/gpt-5"},
		NamedAgents: []config.NamedAgentConfig{{
			ID:    "reviewer",
			Model: config.AgentModelConfig{Primary: "openai/gpt-5"},
		}},
	}

	restoreQuickSetupSnapshot(cfg, snapshot)

	if !reflect.DeepEqual(cfg.Providers, snapshot.Providers) {
		t.Fatalf("providers = %#v, want %#v", cfg.Providers, snapshot.Providers)
	}
	if cfg.MainAgent != snapshot.MainAgent {
		t.Fatalf("main = %#v, want %#v", cfg.MainAgent, snapshot.MainAgent)
	}
	if cfg.SubAgent != snapshot.SubAgent {
		t.Fatalf("sub = %#v, want %#v", cfg.SubAgent, snapshot.SubAgent)
	}
	if !reflect.DeepEqual(cfg.NamedAgents, snapshot.NamedAgents) {
		t.Fatalf("named agents = %#v, want %#v", cfg.NamedAgents, snapshot.NamedAgents)
	}
}
func TestApplyQuickProviderSelectionReplacesProvidersWithSingleProvider(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{
			{Name: "old-one", Models: []string{"a"}},
			{Name: "old-two", Models: []string{"b"}},
		},
	}
	selected := config.ProviderConfig{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5", "gpt-4.1"}}

	applyQuickProviderSelection(cfg, selected)

	want := []config.ProviderConfig{selected}
	if !reflect.DeepEqual(cfg.Providers, want) {
		t.Fatalf("providers = %#v, want %#v", cfg.Providers, want)
	}
}

func TestApplyQuickPrimaryModelSetsProviderQualifiedPrimaryAndClearsAllQuickOverrides(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent: config.AgentModelConfig{Primary: "old/model", Fallback: "old/fallback"},
		SubAgent:  config.AgentModelConfig{Primary: "custom/sub", Fallback: "custom/sub-fallback"},
		NamedAgents: []config.NamedAgentConfig{{
			ID:    "writer",
			Model: config.AgentModelConfig{Primary: "custom/writer"},
		}},
	}

	applyQuickPrimaryModel(cfg, "openai", "gpt-5")

	if cfg.MainAgent.Primary != "openai/gpt-5" {
		t.Fatalf("main primary = %q, want %q", cfg.MainAgent.Primary, "openai/gpt-5")
	}
	if cfg.MainAgent.Fallback != "" {
		t.Fatalf("main fallback = %q, want empty", cfg.MainAgent.Fallback)
	}
	if cfg.SubAgent != (config.AgentModelConfig{}) {
		t.Fatalf("sub = %#v, want empty", cfg.SubAgent)
	}
	if cfg.NamedAgents != nil {
		t.Fatalf("named agents = %#v, want nil", cfg.NamedAgents)
	}
}

func TestQuickPrimaryModelOptionsUseOnlySelectedProviderModels(t *testing.T) {
	got := buildQuickPrimaryModelOptions(config.ProviderConfig{
		Name:   "openai",
		Models: []string{"gpt-5", "gpt-4.1"},
	})
	want := []huh.Option[string]{
		huh.NewOption("gpt-5", "gpt-5"),
		huh.NewOption("gpt-4.1", "gpt-4.1"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %#v, want %#v", got, want)
	}
}

func TestQuickPrimaryModelOptionsEmptyWhenProviderHasNoModels(t *testing.T) {
	got := buildQuickPrimaryModelOptions(config.ProviderConfig{Name: "openai"})
	if len(got) != 0 {
		t.Fatalf("len(options) = %d, want 0", len(got))
	}
}

func TestProviderEditorHelpFooterUsesExpandedTextWhenModelListFocused(t *testing.T) {
	got := ui.ProviderEditorHelpFooter(true)
	want := ui.RenderHelpFooter("ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认")
	if got != want {
		t.Fatalf("help footer = %q, want %q", got, want)
	}
}

func TestProviderModelListFieldViewAppendsOverflowHintBelowOptions(t *testing.T) {
	var selected []string
	field := ui.NewProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(ui.ProviderModelListBaseDescription).
			Options(
				huh.NewOption("a", "a"),
				huh.NewOption("b", "b"),
				huh.NewOption("c", "c"),
				huh.NewOption("d", "d"),
				huh.NewOption("e", "e"),
				huh.NewOption("f", "f"),
				huh.NewOption("g", "g"),
				huh.NewOption("h", "h"),
				huh.NewOption("i", "i"),
				huh.NewOption("j", "j"),
			).
			Value(&selected).
			WithTheme(ui.ProviderModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 8 },
		func() int { return 10 },
	)
	field.LastWindowHeight = 8

	got := field.View()
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if !strings.Contains(got, want) {
		t.Fatalf("view = %q, want substring %q", got, want)
	}
}

func TestProviderModelListFieldViewHidesOverflowHintAtBottom(t *testing.T) {
	var selected []string
	field := ui.NewProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(ui.ProviderModelListBaseDescription).
			Options(
				huh.NewOption("a", "a"),
				huh.NewOption("b", "b"),
				huh.NewOption("c", "c"),
				huh.NewOption("d", "d"),
				huh.NewOption("e", "e"),
				huh.NewOption("f", "f"),
				huh.NewOption("g", "g"),
				huh.NewOption("h", "h"),
				huh.NewOption("i", "i"),
				huh.NewOption("j", "j"),
			).
			Value(&selected).
			WithTheme(ui.ProviderModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 8 },
		func() int { return 10 },
	)
	field.WithKeyMap(huh.NewDefaultKeyMap())

	model, _ := field.Update(tea.WindowSizeMsg{Height: 8, Width: 80})
	field = model.(*ui.ProviderModelListField)
	_ = field.View()
	for range 9 {
		model, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		field = model.(*ui.ProviderModelListField)
		_ = field.View()
	}

	got := field.View()
	if strings.Contains(got, "↓ 更多模型") {
		t.Fatalf("view = %q, want no overflow hint at bottom", got)
	}
}

func TestProviderModelListFieldFocusStateTracksFocusAndBlur(t *testing.T) {
	var selected []string
	field := ui.NewProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(ui.ProviderModelListBaseDescription).
			Options(huh.NewOption("a", "a")).
			Value(&selected).
			WithTheme(ui.ProviderModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 6 },
		func() int { return 1 },
	)

	if field.IsFocused() {
		t.Fatalf("expected unfocused field")
	}
	field.Focus()
	if !field.IsFocused() {
		t.Fatalf("expected focused field")
	}
	field.Blur()
	if field.IsFocused() {
		t.Fatalf("expected blurred field")
	}
}

func TestProviderEditorHelpFooterSwitchesForFocusedModelList(t *testing.T) {
	defaultFooter := ui.ProviderEditorHelpFooter(false)
	focusedFooter := ui.ProviderEditorHelpFooter(true)

	if !strings.Contains(defaultFooter, "ctrl+c 取消") {
		t.Fatalf("default footer = %q, want cancel help", defaultFooter)
	}
	if strings.Contains(defaultFooter, "空格/x 切换选中") {
		t.Fatalf("default footer should not include model list help: %q", defaultFooter)
	}
	wantFocused := "ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认"
	if !strings.Contains(focusedFooter, wantFocused) {
		t.Fatalf("focused footer = %q, want content %q", focusedFooter, wantFocused)
	}
}

func TestFormModelViewUsesCustomHelpFooterWhenProvided(t *testing.T) {
	form := ui.PrepareFormForRun(ui.NewForm(huh.NewGroup(
		huh.NewNote().
			Title("提示").
			Description("测试表单").
			Next(true),
	)))
	model := &ui.FormModel{
		Form: form,
		HelpFooterView: func() string {
			return "custom footer"
		},
	}

	view := model.View()
	if !strings.Contains(view, "custom footer") {
		t.Fatalf("View() = %q, want custom footer", view)
	}
	if strings.Contains(view, ui.HelpFooter) {
		t.Fatalf("View() should not include default footer when custom footer is provided: %q", view)
	}
}

func TestProviderModelRegisterAddsNewCustomModelsToRegistryAndSelection(t *testing.T) {
	selected, registry, added := registerProviderCustomModels(
		[]string{"claude-opus-4-6"},
		[]string{"custom-alpha"},
		"custom-beta, custom-gamma, custom-alpha",
	)

	if !reflect.DeepEqual(selected, []string{"claude-opus-4-6", "custom-beta", "custom-gamma", "custom-alpha"}) {
		t.Fatalf("selected = %v, want %v", selected, []string{"claude-opus-4-6", "custom-beta", "custom-gamma", "custom-alpha"})
	}
	if !reflect.DeepEqual(registry, []string{"custom-alpha", "custom-beta", "custom-gamma"}) {
		t.Fatalf("registry = %v, want %v", registry, []string{"custom-alpha", "custom-beta", "custom-gamma"})
	}
	if !reflect.DeepEqual(added, []string{"custom-beta", "custom-gamma"}) {
		t.Fatalf("added = %v, want %v", added, []string{"custom-beta", "custom-gamma"})
	}
}

func TestProviderModelRegisterTreatsPresetNamedInputAsSelectionOnly(t *testing.T) {
	selected, registry, added := registerProviderCustomModels(
		nil,
		nil,
		"gpt-5.2, custom-alpha",
	)

	if !reflect.DeepEqual(selected, []string{"gpt-5.2", "custom-alpha"}) {
		t.Fatalf("selected = %v, want %v", selected, []string{"gpt-5.2", "custom-alpha"})
	}
	if !reflect.DeepEqual(registry, []string{"custom-alpha"}) {
		t.Fatalf("registry = %v, want %v", registry, []string{"custom-alpha"})
	}
	if !reflect.DeepEqual(added, []string{"custom-alpha"}) {
		t.Fatalf("added = %v, want %v", added, []string{"custom-alpha"})
	}
}

func TestProviderCustomInputHandledEnterClearsValueAndMutatesSelection(t *testing.T) {
	selectedModels := []string{"claude-opus-4-6"}
	customRegistry := []string{"custom-alpha"}
	customInput := "custom-beta"
	optionsVersion := 0

	field := ui.NewProviderCustomModelInput(
		&customInput,
		func(raw string) ui.ProviderCustomRegistration {
			var added []string
			selectedModels, customRegistry, added = registerProviderCustomModels(selectedModels, customRegistry, raw)
			result := ui.ProviderCustomRegistration{
				Handled: len(parseCustomModelInput(raw)) > 0,
				Added:   len(added) > 0,
			}
			if result.Added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(ui.ChineseKeyMap())

	_, cmd := field.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected navigation command")
	}
	if customInput != "" {
		t.Fatalf("customInput = %q, want empty", customInput)
	}
	if optionsVersion != 1 {
		t.Fatalf("optionsVersion = %d, want 1", optionsVersion)
	}
	if !reflect.DeepEqual(customRegistry, []string{"custom-alpha", "custom-beta"}) {
		t.Fatalf("customRegistry = %v", customRegistry)
	}
	if !reflect.DeepEqual(selectedModels, []string{"claude-opus-4-6", "custom-beta"}) {
		t.Fatalf("selectedModels = %v", selectedModels)
	}
}

func TestProviderCustomInputHandledDuplicateStillClearsValue(t *testing.T) {
	selectedModels := []string{"custom-alpha"}
	customRegistry := []string{"custom-alpha"}
	customInput := "custom-alpha"
	optionsVersion := 0

	field := ui.NewProviderCustomModelInput(
		&customInput,
		func(raw string) ui.ProviderCustomRegistration {
			var added []string
			selectedModels, customRegistry, added = registerProviderCustomModels(selectedModels, customRegistry, raw)
			result := ui.ProviderCustomRegistration{
				Handled: len(parseCustomModelInput(raw)) > 0,
				Added:   len(added) > 0,
			}
			if result.Added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(ui.ChineseKeyMap())

	_, cmd := field.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatalf("expected navigation command")
	}
	if customInput != "" {
		t.Fatalf("customInput = %q, want empty", customInput)
	}
	if optionsVersion != 0 {
		t.Fatalf("optionsVersion = %d, want 0", optionsVersion)
	}
}

func TestProviderModelListAvailableFieldHeightReservesOtherViewsGapsAndFooter(t *testing.T) {
	got := ui.ProviderModelListAvailableFieldHeight(
		30,
		"Provider 标识名\n说明\n> demo",
		"Base URL\n说明\n> https://example.com",
		"API Key\n> sk-...",
		"自定义模型名称（可选，多个可用逗号/换行分隔）\n> custom-alpha",
	)

	if got != 11 {
		t.Fatalf("available height = %d, want 11", got)
	}
}

func TestProviderModelListAvailableFieldHeightClampsToMinimum(t *testing.T) {
	got := ui.ProviderModelListAvailableFieldHeight(6, "a", "b", "c", "d")
	want := ui.ProviderModelListTitleLines + ui.ProviderModelListBaseDescriptionLines + ui.ProviderModelListOverflowLines + 1
	if got != want {
		t.Fatalf("available height = %d, want %d", got, want)
	}
}

func TestHasAdvancedConfigDetectsSubOrNamedOverrides(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.FullConfig
		want bool
	}{
		{
			name: "仅主模型不算高级配置",
			cfg: config.FullConfig{
				MainAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2"},
			},
			want: false,
		},
		{
			name: "子 Agent 单独配置算高级配置",
			cfg: config.FullConfig{
				MainAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2"},
				SubAgent:  config.AgentModelConfig{Primary: "demo/gpt-5.2-mini"},
			},
			want: true,
		},
		{
			name: "命名 Agent 单独配置算高级配置",
			cfg: config.FullConfig{
				MainAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2"},
				NamedAgents: []config.NamedAgentConfig{
					{ID: "coder", Model: config.AgentModelConfig{Primary: "demo/gpt-5.2-mini"}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAdvancedConfig(&tt.cfg); got != tt.want {
				t.Fatalf("hasAdvancedConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrepareQuickSetupBacksUpAndClearsAdvancedConfig(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent: config.AgentModelConfig{Primary: "demo/gpt-5.2", Fallback: "demo/gpt-4.1"},
		SubAgent:  config.AgentModelConfig{Primary: "demo/gpt-5.2-mini", Fallback: "demo/gpt-4.1-mini"},
		NamedAgents: []config.NamedAgentConfig{
			{ID: "coder", Model: config.AgentModelConfig{Primary: "demo/gpt-5.2-codex"}},
		},
	}

	snapshot := prepareQuickSetup(cfg)

	if !reflect.DeepEqual(snapshot.MainAgent, config.AgentModelConfig{Primary: "demo/gpt-5.2", Fallback: "demo/gpt-4.1"}) {
		t.Fatalf("snapshot.MainAgent = %+v", snapshot.MainAgent)
	}
	if !reflect.DeepEqual(snapshot.SubAgent, config.AgentModelConfig{Primary: "demo/gpt-5.2-mini", Fallback: "demo/gpt-4.1-mini"}) {
		t.Fatalf("snapshot.SubAgent = %+v", snapshot.SubAgent)
	}
	if len(snapshot.NamedAgents) != 1 || snapshot.NamedAgents[0].ID != "coder" {
		t.Fatalf("snapshot.NamedAgents = %+v", snapshot.NamedAgents)
	}
	if !reflect.DeepEqual(cfg.SubAgent, config.AgentModelConfig{}) {
		t.Fatalf("cfg.SubAgent = %+v, want empty", cfg.SubAgent)
	}
	if len(cfg.NamedAgents) != 0 {
		t.Fatalf("cfg.NamedAgents = %+v, want empty", cfg.NamedAgents)
	}
	if cfg.MainAgent.Primary != "demo/gpt-5.2" {
		t.Fatalf("cfg.MainAgent.Primary = %q, want unchanged", cfg.MainAgent.Primary)
	}
}

func TestRestoreQuickSetupSnapshotRestoresMainSubAndNamedAgents(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent: config.AgentModelConfig{Primary: "demo/new-main"},
	}
	snapshot := quickSetupSnapshot{
		MainAgent: config.AgentModelConfig{Primary: "demo/original-main", Fallback: "demo/original-fallback"},
		SubAgent:  config.AgentModelConfig{Primary: "demo/original-sub"},
		NamedAgents: []config.NamedAgentConfig{
			{ID: "coder", Model: config.AgentModelConfig{Primary: "demo/original-coder"}},
		},
	}

	restoreQuickSetupSnapshot(cfg, snapshot)

	if !reflect.DeepEqual(cfg.MainAgent, snapshot.MainAgent) {
		t.Fatalf("cfg.MainAgent = %+v, want %+v", cfg.MainAgent, snapshot.MainAgent)
	}
	if !reflect.DeepEqual(cfg.SubAgent, snapshot.SubAgent) {
		t.Fatalf("cfg.SubAgent = %+v, want %+v", cfg.SubAgent, snapshot.SubAgent)
	}
	if !reflect.DeepEqual(cfg.NamedAgents, snapshot.NamedAgents) {
		t.Fatalf("cfg.NamedAgents = %+v, want %+v", cfg.NamedAgents, snapshot.NamedAgents)
	}
}

func TestQuickSetupSummaryDescriptionMentionsInheritanceAndRestore(t *testing.T) {
	got := quickSetupSummaryDescription(true)
	wants := []string{
		"已临时备份",
		"只保留 1 个 Provider",
		"默认继承主 Agent",
		"完成配置",
		"恢复原样",
	}

	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("quickSetupSummaryDescription() = %q, want contain %q", got, want)
		}
	}
}


