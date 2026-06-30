package main

import (
	"fmt"
	"strings"

	"openclaw_config/internal/config"
)

const (
	boxDTL = "╔"
	boxDTR = "╗"
	boxDBL = "╚"
	boxDBR = "╝"
	boxDH  = "═"
	boxDV  = "║"
	boxDML = "╠"
	boxDMR = "╣"
)

func printLogo() {
	logo := []string{
		"██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗",
		"██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║",
		"██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║",
		"██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║",
		"██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║",
		"╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝",
	}

	// 金黄→琥珀渐变
	grad := []string{
		"\033[38;2;255;215;0m",
		"\033[38;2;255;185;0m",
		"\033[38;2;255;160;0m",
		"\033[38;2;255;140;0m",
		"\033[38;2;235;130;30m",
		"\033[38;2;205;110;10m",
	}

	fmt.Println()
	for i, line := range logo {
		fmt.Printf("  %s%s%s%s\n", grad[i], cBold, line, cReset)
	}
	fmt.Println()
	fmt.Printf("  %s%sDMXAPI · OpenClaw 配置工具  ·  让 AI 触手可及%s\n", cDim, cGray, cReset)
	fmt.Printf("  %sv%s%s\n\n", cDim, Version, cReset)
}

func printConfigSummaryDouble(cfg *config.FullConfig) {
	type row struct {
		label string
		value string
		style string
	}

	var rows []row
	for _, p := range cfg.Providers {
		rows = append(rows, row{"Provider", fmt.Sprintf("%s  (%s)  [%s]", p.Name, p.BaseUrl, p.ApiFormat), cGreen})
		rows = append(rows, row{"模型", strings.Join(p.Models, ", "), cCyan})
	}
	rows = append(rows, row{"主 Agent", cfg.MainAgent.Primary, cCyan})
	if cfg.SubAgent.Primary != "" {
		rows = append(rows, row{"子 Agent", cfg.SubAgent.Primary, cMagenta})
	}
	for _, na := range cfg.NamedAgents {
		rows = append(rows, row{fmt.Sprintf("Named [%s]", na.ID), na.Model.Primary, cMagenta})
	}

	// 计算宽度
	labelW := 0
	for _, r := range rows {
		if w := visibleLength(r.label); w > labelW {
			labelW = w
		}
	}

	title := "配置摘要"
	contentW := visibleLength(title)
	lines := make([]string, len(rows))
	for i, r := range rows {
		pad := labelW - visibleLength(r.label)
		if pad < 0 {
			pad = 0
		}
		lines[i] = cBold + r.label + cReset + strings.Repeat(" ", pad) + "  " + r.style + r.value + cReset
		if w := visibleLength(lines[i]); w > contentW {
			contentW = w
		}
	}

	inner := contentW + 2
	bar := strings.Repeat(boxDH, inner)

	fmt.Println()
	fmt.Printf("%s%s%s%s%s\n", cCyan, boxDTL, bar, boxDTR, cReset)

	tpad := inner - visibleLength(title)
	if tpad < 0 {
		tpad = 0
	}
	tl := tpad / 2
	fmt.Printf("%s%s %s %s%s\n", cCyan, boxDV, cBold+title+cReset, strings.Repeat(" ", tpad-tl), cCyan+boxDV+cReset)

	fmt.Printf("%s%s%s%s\n", cCyan, boxDML, bar, cCyan+boxDMR+cReset)
	for _, l := range lines {
		rpad := inner - visibleLength(l) - 1
		if rpad < 0 {
			rpad = 0
		}
		fmt.Printf("%s%s %s%s%s\n", cCyan, boxDV, l, strings.Repeat(" ", rpad), cCyan+boxDV+cReset)
	}
	fmt.Printf("%s%s%s%s\n", cCyan, boxDBL, bar, cCyan+boxDBR+cReset)
}
