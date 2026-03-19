package main

import (
	"io"
	"os"
	"reflect"
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
		fieldHeight:      7,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestComputeProviderModelListPresentationShowsOverflowWhenSpaceIsTight(t *testing.T) {
	got := computeProviderModelListPresentation(8, 10)
	want := providerModelListPresentation{
		fieldHeight:      8,
		visibleRows:      4,
		hiddenCount:      6,
		showOverflowHint: true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation = %+v, want %+v", got, want)
	}
}

func TestProviderModelListDescriptionWithoutOverflowUsesBaseText(t *testing.T) {
	got := providerModelListDescription(providerModelListPresentation{
		fieldHeight:      7,
		visibleRows:      4,
		hiddenCount:      0,
		showOverflowHint: false,
	})
	want := providerModelListBaseDescription
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
	}
}

func TestProviderModelListDescriptionWithOverflowAppendsHiddenCount(t *testing.T) {
	got := providerModelListDescription(providerModelListPresentation{
		fieldHeight:      8,
		visibleRows:      4,
		hiddenCount:      6,
		showOverflowHint: true,
	})
	want := providerModelListBaseDescription + "\n当前仅显示前 4 项，还有 6 项可继续向下查看"
	if got != want {
		t.Fatalf("description = %q, want %q", got, want)
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
