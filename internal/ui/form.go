package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// FormModel 将 huh.Form 包裹为 tea.Model，在 View() 底部追加固定帮助栏。
type FormModel struct {
	Form           *huh.Form
	HelpFooterView func() string
}

func (m *FormModel) Init() tea.Cmd {
	return m.Form.Init()
}

func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.Form.Update(msg)
	m.Form = form.(*huh.Form)
	return m, cmd
}

func (m *FormModel) View() string {
	v := m.Form.View()
	if v == "" {
		return "" // form.go:610：quitting 时 View() 确定返回空字符串
	}
	if m.HelpFooterView != nil {
		if footer := m.HelpFooterView(); footer != "" {
			return v + "\n" + footer
		}
	}
	return v + "\n" + HelpFooter
}

// PrepareFormForRun 预处理表单：隐藏内置帮助、设置提交/取消命令为 Quit
func PrepareFormForRun(form *huh.Form) *huh.Form {
	form.WithShowHelp(false)
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Quit
	return form
}

// RunForm 替代 form.Run()，通过 bubbletea 程序展示带固定帮助栏的表单。
func RunForm(form *huh.Form) error {
	return RunFormWithFooter(form, nil)
}

// RunFormWithFooter 带自定义底部帮助栏的表单运行
func RunFormWithFooter(form *huh.Form, helpFooterView func() string) error {
	form = PrepareFormForRun(form)
	m := &FormModel{Form: form, HelpFooterView: helpFooterView}
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if result.(*FormModel).Form.State == huh.StateAborted {
		return huh.ErrUserAborted
	}
	return nil
}

// NewForm 创建带中文 KeyMap 的 huh.Form（统一入口，避免重复调用 WithKeyMap）
func NewForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithKeyMap(ChineseKeyMap()).WithShowHelp(false)
}
