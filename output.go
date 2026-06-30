package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

func printSuccess(cfg *config.FullConfig) {
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

	box := ui.SuccessBoxStyle().Render("✓ 配置已保存！\n\n" + strings.Join(lines, "\n"))

	fmt.Println()
	fmt.Println(box)

	f := ui.NewForm(huh.NewGroup(ui.NewSuccessDismissField(ui.SavedConfigNoticeDescription())))
	ui.RunForm(f) //nolint:errcheck
}

func printBanner(updateStatus releaseUpdateStatus) {
	purple := lipgloss.Color("63")

	art := "  ██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗\n" +
		"  ██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║\n" +
		"  ██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║\n" +
		"  ██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║\n" +
		"  ██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║\n" +
		"  ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝"

	logo := lipgloss.NewStyle().Foreground(purple).Render(art)
	subtitle := lipgloss.NewStyle().Bold(true).
		Render("  OpenClaw 配置工具  ·  openclaw-config " + displayVersion(Version))
	sep := ui.DimStyle().Render("  ────────────────────────────────────────────────")
	note := ui.DimStyle().Render("  多 Provider · 多模型 · 主/子/命名 Agent 独立配置")

	fmt.Println(logo)
	fmt.Println()
	fmt.Println(subtitle)
	fmt.Println(sep)
	fmt.Println(note)
	if updateLine := buildUpdateStatusLine(updateStatus); updateLine != "" {
		style := ui.DimStyle()
		if updateStatus.HasUpdate {
			style = ui.YellowBold()
		}
		fmt.Println(style.Render("  " + updateLine))
	}
	fmt.Println()
}
