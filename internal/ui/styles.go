package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── ANSI 颜色常量（对齐 hermes 风格）──

const (
	CReset   = "\033[0m"
	CRed     = "\033[91m"
	CGreen   = "\033[92m"
	CYellow  = "\033[93m"
	CCyan    = "\033[96m"
	CCyanMid = "\033[36m"
	CBlue    = "\033[94m"
	CMagenta = "\033[95m"
	CGray    = "\033[90m"
	CBold    = "\033[1m"
	CDim     = "\033[2m"
)

// ── 图标（对齐 hermes 风格）──

const (
	IconOK    = "✔"
	IconErr   = "✘"
	IconWarn  = "⚠"
	IconInfo  = "→"
	IconTip   = "◆"
	IconArrow = "❯"
)

// ── lipgloss 便捷样式 ──

var (
	Bold     = lipgloss.NewStyle().Bold(true)
	Green    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	Yellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	Cyan     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	Magenta  = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	GrayDim  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	BoldCyan = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
)

// ── 帮助栏 ──

const DefaultHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认"

const ProviderModelListFocusedHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认"

// RenderHelpFooter 渲染带样式的帮助栏文本
func RenderHelpFooter(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("  " + text)
}

// HelpFooter 是统一显示在所有表单底部的导航提示栏
var HelpFooter = RenderHelpFooter(DefaultHelpFooterText)

// ProviderModelListTheme 返回模型列表专用的 huh 主题（ASCII checkbox 前缀）
func ProviderModelListTheme() *huh.Theme {
	theme := huh.ThemeCharm()
	theme.Focused.SelectedPrefix = lipgloss.NewStyle().SetString("[✓] ")
	theme.Focused.UnselectedPrefix = lipgloss.NewStyle().SetString("[ ] ")
	theme.Blurred.SelectedPrefix = lipgloss.NewStyle().SetString("[✓] ")
	theme.Blurred.UnselectedPrefix = lipgloss.NewStyle().SetString("[ ] ")
	return theme
}

// ProviderEditorHelpFooter 根据模型列表是否获得焦点返回对应的帮助栏
func ProviderEditorHelpFooter(modelListFocused bool) string {
	if modelListFocused {
		return RenderHelpFooter(ProviderModelListFocusedHelpFooterText)
	}
	return RenderHelpFooter(DefaultHelpFooterText)
}

// SuccessBoxStyle 成功提示框样式
func SuccessBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Padding(1, 2)
}

// YellowBold 黄色粗体样式（用于警告标题）
func YellowBold() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
}

// DimStyle 灰色样式（用于辅助说明）
func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
}

// FormatWarning 格式化警告行
func FormatWarning(title string, items []string) string {
	yellow := YellowBold()
	dim := DimStyle()
	result := yellow.Render(fmt.Sprintf("  ⚠ %s", title))
	for _, item := range items {
		result += "\n" + dim.Render("    · "+item)
	}
	return result
}
