package ui

// SavedConfigNoticeDescription 保存配置后的提示文案
func SavedConfigNoticeDescription() string {
	return "✓ 配置已保存。默认在 OpenClaw 的 hybrid reload 下，大多数 agent/model 变更会自动生效。\n若修改 gateway/plugins/discovery/canvasHost，请执行 openclaw gateway restart。"
}

// ProviderManagementDescription Provider 管理页面的描述文案
func ProviderManagementDescription() string {
	return "同一 Provider 只配置一种模型格式。\nOpenAI 兼容、GPT-5 系列、Anthropic、Gemini 请分开配置。"
}
