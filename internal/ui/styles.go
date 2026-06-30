package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

const DefaultHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  enter 确认"

const ProviderModelListFocusedHelpFooterText = "ctrl+c 取消  ·  shift+tab 上一项  ·  空格/x 切换选中  ·  ↑↓ 移动  ·  enter 确认"

const ProviderFieldGapLines = 2

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

// savedConfigNoticeDescription 保存配置后的提示文案（已导出供 output.go 使用）
func SavedConfigNoticeDescription() string {
	return "✓ 配置已保存。默认在 OpenClaw 的 hybrid reload 下，大多数 agent/model 变更会自动生效。\n若修改 gateway/plugins/discovery/canvasHost，请执行 openclaw gateway restart。"
}

// ProviderManagementDescription Provider 管理页面的描述文案
func ProviderManagementDescription() string {
	return "同一 Provider 只配置一种模型格式。\nOpenAI 兼容、GPT-5 系列、Anthropic、Gemini 请分开配置。"
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
