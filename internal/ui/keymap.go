package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

// ChineseKeyMap 返回带中文说明的 KeyMap，覆盖所有字段类型（Input/Confirm/Note/Select/MultiSelect）的导航提示
func ChineseKeyMap() *huh.KeyMap {
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
