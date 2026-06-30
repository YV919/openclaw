package ui

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── SuccessDismissField ──────────────────────────────────────────────────

// SuccessDismissField 自定义 Note 字段，只接受 Enter/Tab 前进，Shift+Tab 返回
type SuccessDismissField struct {
	note        *huh.Note
	title       string
	description string
	nextLabel   string
}

// NewSuccessDismissField 创建 SuccessDismissField
func NewSuccessDismissField(description string) *SuccessDismissField {
	const title = "提示"
	const nextLabel = "按 Enter 退出"
	return &SuccessDismissField{
		title:       title,
		description: description,
		nextLabel:   nextLabel,
		note: huh.NewNote().
			Title(title).
			Description(description).
			Next(true).
			NextLabel(nextLabel),
	}
}

func (f *SuccessDismissField) Init() tea.Cmd { return f.note.Init() }

func (f *SuccessDismissField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "shift+tab":
			return f, huh.PrevField
		case "enter", "tab":
			return f, huh.NextField
		default:
			return f, nil
		}
	}

	model, cmd := f.note.Update(msg)
	f.note = model.(*huh.Note)
	return f, cmd
}

func (f *SuccessDismissField) View() string            { return f.note.View() }
func (f *SuccessDismissField) Blur() tea.Cmd           { return f.note.Blur() }
func (f *SuccessDismissField) Focus() tea.Cmd          { return f.note.Focus() }
func (f *SuccessDismissField) Error() error            { return f.note.Error() }
func (f *SuccessDismissField) Run() error              { return f.note.Run() }
func (f *SuccessDismissField) Skip() bool              { return false }
func (f *SuccessDismissField) Zoom() bool              { return f.note.Zoom() }
func (f *SuccessDismissField) KeyBinds() []key.Binding { return f.note.KeyBinds() }

func (f *SuccessDismissField) RunAccessible(w io.Writer, r io.Reader) error {
	if _, err := fmt.Fprintln(w, f.title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, f.description); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, f.nextLabel); err != nil {
		return err
	}
	if r == nil {
		return nil
	}

	buf := make([]byte, 1)
	for {
		if _, err := r.Read(buf); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if buf[0] == '\n' || buf[0] == '\r' {
			return nil
		}
	}
}

func (f *SuccessDismissField) WithTheme(theme *huh.Theme) huh.Field {
	f.note = f.note.WithTheme(theme).(*huh.Note)
	return f
}

func (f *SuccessDismissField) WithAccessible(accessible bool) huh.Field {
	f.note = f.note.WithAccessible(accessible).(*huh.Note)
	return f
}

func (f *SuccessDismissField) WithKeyMap(k *huh.KeyMap) huh.Field {
	f.note = f.note.WithKeyMap(k).(*huh.Note)
	return f
}

func (f *SuccessDismissField) WithWidth(width int) huh.Field {
	f.note = f.note.WithWidth(width).(*huh.Note)
	return f
}

func (f *SuccessDismissField) WithHeight(height int) huh.Field {
	f.note = f.note.WithHeight(height).(*huh.Note)
	return f
}

func (f *SuccessDismissField) WithPosition(position huh.FieldPosition) huh.Field {
	f.note = f.note.WithPosition(position).(*huh.Note)
	return f
}

func (f *SuccessDismissField) GetKey() string { return f.note.GetKey() }
func (f *SuccessDismissField) GetValue() any  { return f.note.GetValue() }

// ── ProviderModelListPresentation ────────────────────────────────────────

const (
	ProviderModelListTitleLines           = 1
	ProviderModelListBaseDescriptionLines = 1
	ProviderModelListOverflowLines        = 1
)

// ProviderModelListBaseDescription 模型列表的默认描述文案
const ProviderModelListBaseDescription = "选择此 provider 支持的预设模型；如需自定义模型，请在下方输入框填写，可一次填写多个"

// providerFieldGapLines Provider 编辑器中各字段之间的间隔行数
const providerFieldGapLines = 2

// ProviderModelListPresentation 模型列表的展示参数
type ProviderModelListPresentation struct {
	FieldHeight      int
	VisibleRows      int
	HiddenCount      int
	ShowOverflowHint bool
}

// ComputeProviderModelListPresentation 根据可用高度和选项数量计算展示参数
func ComputeProviderModelListPresentation(availableFieldHeight int, optionCount int) ProviderModelListPresentation {
	minFieldHeight := ProviderModelListTitleLines + ProviderModelListBaseDescriptionLines + ProviderModelListOverflowLines + 1
	if optionCount <= 0 {
		return ProviderModelListPresentation{
			FieldHeight:      minFieldHeight - ProviderModelListOverflowLines,
			VisibleRows:      1,
			HiddenCount:      0,
			ShowOverflowHint: false,
		}
	}

	fullFieldHeight := ProviderModelListTitleLines + ProviderModelListBaseDescriptionLines + optionCount
	if availableFieldHeight >= fullFieldHeight {
		return ProviderModelListPresentation{
			FieldHeight:      fullFieldHeight,
			VisibleRows:      optionCount,
			HiddenCount:      0,
			ShowOverflowHint: false,
		}
	}

	budgetWithHint := max(minFieldHeight, availableFieldHeight)
	visibleRows := max(1, budgetWithHint-ProviderModelListTitleLines-ProviderModelListBaseDescriptionLines-ProviderModelListOverflowLines)
	return ProviderModelListPresentation{
		FieldHeight:      ProviderModelListTitleLines + ProviderModelListBaseDescriptionLines + visibleRows,
		VisibleRows:      visibleRows,
		HiddenCount:      max(0, optionCount-visibleRows),
		ShowOverflowHint: true,
	}
}

// ProviderModelListOverflowHint 生成溢出提示文本
func ProviderModelListOverflowHint(p ProviderModelListPresentation) string {
	if !p.ShowOverflowHint || p.HiddenCount <= 0 {
		return ""
	}
	return fmt.Sprintf("↓ 更多模型（还有 %d 项，继续向下查看）", p.HiddenCount)
}

// ProviderModelListRemainingBelow 获取 MultiSelect 中当前不可见的剩余选项数。
//
// 注意：此函数通过 reflect 访问 huh.MultiSelect 的未导出字段（filteredOptions、viewport），
// 非常脆弱——上游 huh 库任何字段重命名都会导致静默返回 (0, false)。
// 当前兼容 huh v0.6.0；升级 huh 时需验证此处是否仍然有效。
func ProviderModelListRemainingBelow(field *huh.MultiSelect[string]) (int, bool) {
	if field == nil {
		return 0, false
	}

	value := reflect.ValueOf(field)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return 0, false
	}

	elem := value.Elem()
	filteredOptions := elem.FieldByName("filteredOptions")
	viewport := elem.FieldByName("viewport")
	if !filteredOptions.IsValid() || !viewport.IsValid() {
		return 0, false
	}

	yOffset := viewport.FieldByName("YOffset")
	height := viewport.FieldByName("Height")
	if !yOffset.IsValid() || !height.IsValid() {
		return 0, false
	}

	bottom := int(yOffset.Int()) + int(height.Int())
	if bottom < 0 {
		bottom = 0
	}
	if bottom > filteredOptions.Len() {
		bottom = filteredOptions.Len()
	}

	return max(0, filteredOptions.Len()-bottom), true
}

// ProviderModelListAvailableFieldHeight 计算模型列表字段可用的终端高度
func ProviderModelListAvailableFieldHeight(windowHeight int, otherViews ...string) int {
	reserved := lipgloss.Height(HelpFooter)
	for _, view := range otherViews {
		reserved += lipgloss.Height(view)
	}
	reserved += len(otherViews) * providerFieldGapLines

	minFieldHeight := ProviderModelListTitleLines + ProviderModelListBaseDescriptionLines + ProviderModelListOverflowLines + 1
	return max(minFieldHeight, windowHeight-reserved)
}

// ── ProviderModelListField ──────────────────────────────────────────────

// ProviderModelListField 自适应高度的 MultiSelect 包装
type ProviderModelListField struct {
	Field            *huh.MultiSelect[string]
	MeasureAvailable func(int) int
	OptionCount      func() int
	LastWindowHeight int
	presentation     ProviderModelListPresentation
	focused          bool
}

// NewProviderModelListField 创建 ProviderModelListField
func NewProviderModelListField(
	field *huh.MultiSelect[string],
	measureAvailable func(int) int,
	optionCount func() int,
) *ProviderModelListField {
	return &ProviderModelListField{
		Field:            field,
		MeasureAvailable: measureAvailable,
		OptionCount:      optionCount,
	}
}

func (f *ProviderModelListField) syncPresentation(windowHeight int) {
	available := f.MeasureAvailable(windowHeight)
	presentation := ComputeProviderModelListPresentation(available, f.OptionCount())
	if presentation == f.presentation {
		return
	}
	f.presentation = presentation
	f.Field.Description(ProviderModelListBaseDescription)
	f.Field.Height(presentation.FieldHeight)
}

func (f *ProviderModelListField) Init() tea.Cmd { return f.Field.Init() }

func (f *ProviderModelListField) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.LastWindowHeight = windowMsg.Height
	}
	if f.LastWindowHeight > 0 {
		f.syncPresentation(f.LastWindowHeight)
	}
	m, cmd := f.Field.Update(msg)
	f.Field = m.(*huh.MultiSelect[string])
	return f, cmd
}

func (f *ProviderModelListField) View() string {
	if f.LastWindowHeight > 0 {
		f.syncPresentation(f.LastWindowHeight)
	}
	view := f.Field.View()
	presentation := f.presentation
	if hiddenCount, ok := ProviderModelListRemainingBelow(f.Field); ok {
		presentation.HiddenCount = hiddenCount
		presentation.ShowOverflowHint = presentation.ShowOverflowHint && hiddenCount > 0
	}
	if hint := ProviderModelListOverflowHint(presentation); hint != "" {
		return view + "\n" + hint
	}
	return view
}

func (f *ProviderModelListField) Blur() tea.Cmd {
	f.focused = false
	return f.Field.Blur()
}

func (f *ProviderModelListField) Focus() tea.Cmd {
	f.focused = true
	return f.Field.Focus()
}

func (f *ProviderModelListField) IsFocused() bool         { return f.focused }
func (f *ProviderModelListField) Error() error            { return f.Field.Error() }
func (f *ProviderModelListField) Run() error              { return f.Field.Run() }
func (f *ProviderModelListField) Skip() bool              { return f.Field.Skip() }
func (f *ProviderModelListField) Zoom() bool              { return f.Field.Zoom() }
func (f *ProviderModelListField) KeyBinds() []key.Binding { return f.Field.KeyBinds() }

func (f *ProviderModelListField) WithTheme(theme *huh.Theme) huh.Field {
	f.Field = f.Field.WithTheme(theme).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) WithAccessible(accessible bool) huh.Field {
	f.Field = f.Field.WithAccessible(accessible).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) WithKeyMap(k *huh.KeyMap) huh.Field {
	f.Field = f.Field.WithKeyMap(k).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) WithWidth(width int) huh.Field {
	f.Field = f.Field.WithWidth(width).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) WithHeight(height int) huh.Field {
	f.Field = f.Field.WithHeight(height).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) WithPosition(position huh.FieldPosition) huh.Field {
	f.Field = f.Field.WithPosition(position).(*huh.MultiSelect[string])
	return f
}

func (f *ProviderModelListField) GetKey() string { return f.Field.GetKey() }
func (f *ProviderModelListField) GetValue() any  { return f.Field.GetValue() }

// ── ProviderCustomModelInput ─────────────────────────────────────────────

// ProviderCustomRegistration 自定义模型注册结果
type ProviderCustomRegistration struct {
	Handled bool
	Added   bool
}

// ProviderCustomModelInput 带 onSubmit 回调的自定义模型输入框
type ProviderCustomModelInput struct {
	input    *huh.Input
	accessor *huh.PointerAccessor[string]
	keymap   *huh.KeyMap
	OnSubmit func(string) ProviderCustomRegistration
}

// NewProviderCustomModelInput 创建 ProviderCustomModelInput
func NewProviderCustomModelInput(value *string, onSubmit func(string) ProviderCustomRegistration) *ProviderCustomModelInput {
	accessor := huh.NewPointerAccessor(value)
	return &ProviderCustomModelInput{
		input: huh.NewInput().
			Title("自定义模型名称（可选，多个可用逗号/换行分隔）").
			Placeholder("my-custom-model-a, my-custom-model-b").
			Accessor(accessor),
		accessor: accessor,
		OnSubmit: onSubmit,
	}
}

func (f *ProviderCustomModelInput) Init() tea.Cmd { return f.input.Init() }

func (f *ProviderCustomModelInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && f.keymap != nil {
		if key.Matches(keyMsg, f.keymap.Input.Next, f.keymap.Input.Submit) {
			result := f.OnSubmit(f.accessor.Get())
			if result.Handled {
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

func (f *ProviderCustomModelInput) View() string            { return f.input.View() }
func (f *ProviderCustomModelInput) Blur() tea.Cmd           { return f.input.Blur() }
func (f *ProviderCustomModelInput) Focus() tea.Cmd          { return f.input.Focus() }
func (f *ProviderCustomModelInput) Error() error            { return f.input.Error() }
func (f *ProviderCustomModelInput) Run() error              { return f.input.Run() }
func (f *ProviderCustomModelInput) Skip() bool              { return f.input.Skip() }
func (f *ProviderCustomModelInput) Zoom() bool              { return f.input.Zoom() }
func (f *ProviderCustomModelInput) KeyBinds() []key.Binding { return f.input.KeyBinds() }

func (f *ProviderCustomModelInput) WithTheme(theme *huh.Theme) huh.Field {
	f.input = f.input.WithTheme(theme).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) WithAccessible(accessible bool) huh.Field {
	f.input = f.input.WithAccessible(accessible).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) WithKeyMap(k *huh.KeyMap) huh.Field {
	f.keymap = k
	f.input = f.input.WithKeyMap(k).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) WithWidth(width int) huh.Field {
	f.input = f.input.WithWidth(width).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) WithHeight(height int) huh.Field {
	f.input = f.input.WithHeight(height).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) WithPosition(position huh.FieldPosition) huh.Field {
	f.input = f.input.WithPosition(position).(*huh.Input)
	return f
}

func (f *ProviderCustomModelInput) GetKey() string { return f.input.GetKey() }
func (f *ProviderCustomModelInput) GetValue() any  { return f.input.GetValue() }
