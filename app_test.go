package main

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
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

func TestProviderManagementDescriptionUsesFormatReminderCopy(t *testing.T) {
	got := providerManagementDescription()
	want := "同一 Provider 只配置一种模型格式。\nOpenAI 兼容、GPT-5 系列、Anthropic、Gemini 请分开配置。"

	if got != want {
		t.Fatalf("providerManagementDescription() = %q, want %q", got, want)
	}
}

func TestSavedConfigNoticeDescriptionMatchesHybridReloadGuidance(t *testing.T) {
	got := savedConfigNoticeDescription()
	want := "✓ 配置已保存。默认在 OpenClaw 的 hybrid reload 下，大多数 agent/model 变更会自动生效。\n若修改 gateway/plugins/discovery/canvasHost，请执行 openclaw gateway restart。"

	if got != want {
		t.Fatalf("savedConfigNoticeDescription() = %q, want %q", got, want)
	}
}

func TestSuccessDismissFieldDoesNotSkip(t *testing.T) {
	field := newSuccessDismissField("测试说明")

	if field.Skip() {
		t.Fatal("success dismiss field should block until Enter")
	}
}

func TestSuccessDismissFieldIgnoresNonSubmitKeys(t *testing.T) {
	field := newSuccessDismissField("测试说明")

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
	form := prepareFormForRun(newForm(huh.NewGroup(
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
	theme := providerModelListTheme()
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
	got := computeProviderModelListPresentation(12, 4)
	want := providerModelListPresentation{
		fieldHeight:      6,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestComputeProviderModelListPresentationReservesOverflowHintLine(t *testing.T) {
	got := computeProviderModelListPresentation(8, 10)
	want := providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      5,
		hiddenCount:      5,
		showOverflowHint: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestProviderModelListOverflowHintWithOverflowShowsRemainingCount(t *testing.T) {
	got := providerModelListOverflowHint(providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      5,
		hiddenCount:      5,
		showOverflowHint: true,
	})
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if got != want {
		t.Fatalf("overflow hint = %q, want %q", got, want)
	}
}

func TestProviderModelListOverflowHintWithoutOverflowReturnsEmpty(t *testing.T) {
	got := providerModelListOverflowHint(providerModelListPresentation{
		fieldHeight:      6,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	})
	if got != "" {
		t.Fatalf("overflow hint = %q, want empty", got)
	}
}

func TestProviderEditorHelpFooterUsesDefaultTextWhenNotFocused(t *testing.T) {
	got := providerEditorHelpFooter(false)
	want := renderHelpFooter(defaultHelpFooterText)
	if got != want {
		t.Fatalf("help footer = %q, want %q", got, want)
	}
}

func TestProviderEditorHelpFooterUsesExpandedTextWhenModelListFocused(t *testing.T) {
	got := providerEditorHelpFooter(true)
	want := renderHelpFooter("ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认")
	if got != want {
		t.Fatalf("help footer = %q, want %q", got, want)
	}
}

func TestProviderModelListFieldViewAppendsOverflowHintBelowOptions(t *testing.T) {
	var selected []string
	field := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
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
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 8 },
		func() int { return 10 },
	)
	field.lastWindowHeight = 8

	got := field.View()
	want := "↓ 更多模型（还有 5 项，继续向下查看）"
	if !strings.Contains(got, want) {
		t.Fatalf("view = %q, want substring %q", got, want)
	}
}

func TestProviderModelListFieldViewHidesOverflowHintAtBottom(t *testing.T) {
	var selected []string
	field := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
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
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
		func(int) int { return 8 },
		func() int { return 10 },
	)
	field.WithKeyMap(huh.NewDefaultKeyMap())

	model, _ := field.Update(tea.WindowSizeMsg{Height: 8, Width: 80})
	field = model.(*providerModelListField)
	_ = field.View()
	for range 9 {
		model, _ = field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		field = model.(*providerModelListField)
		_ = field.View()
	}

	got := field.View()
	if strings.Contains(got, "↓ 更多模型") {
		t.Fatalf("view = %q, want no overflow hint at bottom", got)
	}
}

func TestProviderModelListFieldFocusStateTracksFocusAndBlur(t *testing.T) {
	var selected []string
	field := newProviderModelListField(
		huh.NewMultiSelect[string]().
			Title("模型列表").
			Description(providerModelListBaseDescription).
			Options(huh.NewOption("a", "a")).
			Value(&selected).
			WithTheme(providerModelListTheme()).(*huh.MultiSelect[string]),
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
	defaultFooter := providerEditorHelpFooter(false)
	focusedFooter := providerEditorHelpFooter(true)

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
	form := prepareFormForRun(newForm(huh.NewGroup(
		huh.NewNote().
			Title("提示").
			Description("测试表单").
			Next(true),
	)))
	model := &formModel{
		form: form,
		helpFooterView: func() string {
			return "custom footer"
		},
	}

	view := model.View()
	if !strings.Contains(view, "custom footer") {
		t.Fatalf("View() = %q, want custom footer", view)
	}
	if strings.Contains(view, helpFooter) {
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

	field := newProviderCustomModelInput(
		&customInput,
		func(raw string) providerCustomRegistration {
			var added []string
			selectedModels, customRegistry, added = registerProviderCustomModels(selectedModels, customRegistry, raw)
			result := providerCustomRegistration{
				handled: len(parseCustomModelInput(raw)) > 0,
				added:   len(added) > 0,
			}
			if result.added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(chineseKeyMap())

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

	field := newProviderCustomModelInput(
		&customInput,
		func(raw string) providerCustomRegistration {
			var added []string
			selectedModels, customRegistry, added = registerProviderCustomModels(selectedModels, customRegistry, raw)
			result := providerCustomRegistration{
				handled: len(parseCustomModelInput(raw)) > 0,
				added:   len(added) > 0,
			}
			if result.added {
				optionsVersion++
			}
			return result
		},
	)
	field.WithKeyMap(chineseKeyMap())

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
	got := providerModelListAvailableFieldHeight(
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
	got := providerModelListAvailableFieldHeight(6, "a", "b", "c", "d")
	want := providerModelListTitleLines + providerModelListBaseDescriptionLines + providerModelListOverflowLines + 1
	if got != want {
		t.Fatalf("available height = %d, want %d", got, want)
	}
}

func optionValues(opts []huh.Option[string]) []string {
	values := make([]string, 0, len(opts))
	for _, opt := range opts {
		values = append(values, opt.Value)
	}
	return values
}
