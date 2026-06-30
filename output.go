package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"openclaw_config/internal/config"
	"openclaw_config/internal/ui"
)

// ── 配置摘要（双线框，对齐 hermes 风格）──

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

func printConfigSummary(cfg *config.FullConfig) {
	cyan := lipgloss.Color("6")
	bold := lipgloss.NewStyle().Bold(true)
	cyanStyle := lipgloss.NewStyle().Foreground(cyan)

	type row struct{ label, value, color lipgloss.Style }
	var rows []struct {
		label, value string
		labelStyle   lipgloss.Style
		valueStyle   lipgloss.Style
	}

	for _, p := range cfg.Providers {
		rows = append(rows,
			struct {
				label, value string
				labelStyle   lipgloss.Style
				valueStyle   lipgloss.Style
			}{"Provider", fmt.Sprintf("%s  (%s)  [%s]", p.Name, p.BaseUrl, p.ApiFormat), bold, lipgloss.NewStyle().Foreground(lipgloss.Color("10"))},
		)
		rows = append(rows,
			struct {
				label, value string
				labelStyle   lipgloss.Style
				valueStyle   lipgloss.Style
			}{"模型", strings.Join(p.Models, ", "), bold, lipgloss.NewStyle().Foreground(lipgloss.Color("14"))},
		)
	}
	rows = append(rows,
		struct {
			label, value string
			labelStyle   lipgloss.Style
			valueStyle   lipgloss.Style
		}{"主 Agent", cfg.MainAgent.Primary, bold, lipgloss.NewStyle().Foreground(lipgloss.Color("14"))},
	)
	if cfg.SubAgent.Primary != "" {
		rows = append(rows,
			struct {
				label, value string
				labelStyle   lipgloss.Style
				valueStyle   lipgloss.Style
			}{"子 Agent", cfg.SubAgent.Primary, bold, lipgloss.NewStyle().Foreground(lipgloss.Color("13"))},
		)
	}
	for _, na := range cfg.NamedAgents {
		rows = append(rows,
			struct {
				label, value string
				labelStyle   lipgloss.Style
				valueStyle   lipgloss.Style
			}{fmt.Sprintf("Named [%s]", na.ID), na.Model.Primary, bold, lipgloss.NewStyle().Foreground(lipgloss.Color("13"))},
		)
	}

	// 计算列宽
	labelW := 0
	for _, r := range rows {
		if w := lipgloss.Width(r.label); w > labelW {
			labelW = w
		}
	}

	// 渲染行
	title := "配置摘要"
	lines := make([]string, len(rows))
	contentW := lipgloss.Width(title)
	for i, r := range rows {
		pad := labelW - lipgloss.Width(r.label)
		if pad < 0 {
			pad = 0
		}
		lines[i] = fmt.Sprintf("%s%s  %s",
			r.labelStyle.Render(r.label+strings.Repeat(" ", pad)),
			r.valueStyle.Render(r.value), "")
		if w := lipgloss.Width(lines[i]); w > contentW {
			contentW = w
		}
	}

	inner := contentW + 2
	bar := strings.Repeat(boxDH, inner)

	fmt.Println()
	// 顶
	fmt.Printf("%s%s%s%s%s\n", cyanStyle.Render(boxDTL), cyanStyle.Render(bar), cyanStyle.Render(boxDTR), "", "")
	// 标题居中
	tpad := inner - lipgloss.Width(title)
	if tpad < 0 {
		tpad = 0
	}
	tl := tpad / 2
	fmt.Printf("%s%s %s %s%s\n", cyanStyle.Render(boxDV), strings.Repeat(" ", tl), bold.Render(title), strings.Repeat(" ", tpad-tl), cyanStyle.Render(boxDV))
	// 分隔
	fmt.Printf("%s%s%s%s\n", cyanStyle.Render(boxDML), cyanStyle.Render(bar), cyanStyle.Render(boxDMR), "")
	// 行
	for _, l := range lines {
		rpad := inner - lipgloss.Width(l) - 1
		if rpad < 0 {
			rpad = 0
		}
		fmt.Printf("%s%s %s%s%s\n", cyanStyle.Render(boxDV), l, strings.Repeat(" ", rpad), cyanStyle.Render(boxDV), "")
	}
	// 底
	fmt.Printf("%s%s%s%s\n", cyanStyle.Render(boxDBL), cyanStyle.Render(bar), cyanStyle.Render(boxDBR), "")
}

func printSuccess(cfg *config.FullConfig) {
	printConfigSummary(cfg)

	fmt.Println()
	fmt.Printf("  %s✔%s %s\n", ui.CGreen, ui.CReset, ui.Bold.Render("配置已保存！"))
	fmt.Println()

	f := ui.NewForm(huh.NewGroup(ui.NewSuccessDismissField(ui.SavedConfigNoticeDescription())))
	ui.RunForm(f) //nolint:errcheck
}

func printBanner(updateStatus releaseUpdateStatus) {
	logo := []string{
		"██████╗ ███╗   ███╗██╗  ██╗ █████╗ ██████╗ ██╗",
		"██╔══██╗████╗ ████║╚██╗██╔╝██╔══██╗██╔══██╗██║",
		"██║  ██║██╔████╔██║ ╚███╔╝ ███████║██████╔╝██║",
		"██║  ██║██║╚██╔╝██║ ██╔██╗ ██╔══██║██╔═══╝ ██║",
		"██████╔╝██║ ╚═╝ ██║██╔╝ ██╗██║  ██║██║     ██║",
		"╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝",
	}

	// 金黄→橙→琥珀 渐变（对齐 hermes 风格）
	grad := []string{
		"\033[38;2;255;215;0m",
		"\033[38;2;255;185;0m",
		"\033[38;2;255;160;0m",
		"\033[38;2;255;140;0m",
		"\033[38;2;235;130;30m",
		"\033[38;2;205;110;10m",
	}
	bold := "\033[1m"
	reset := "\033[0m"
	dim := "\033[2m"
	magenta := "\033[95m"

	fmt.Println()
	for i, line := range logo {
		fmt.Printf("  %s%s%s%s%s\n", grad[i], bold, line, reset, "")
	}
	fmt.Println()
	fmt.Printf("  %s%sDMXAPI · OpenClaw 配置工具  ·  让 AI 触手可及%s\n", dim, "\033[90m", reset)
	fmt.Printf("  %sv%s%s  %s/%s/%s%s\n", dim, displayVersion(Version), reset, magenta, runtime.GOOS, runtime.GOARCH, reset)
	fmt.Println()

	if updateLine := buildUpdateStatusLine(updateStatus); updateLine != "" {
		if updateStatus.HasUpdate {
			fmt.Printf("  %s⚠ %s%s\n", "\033[93m\033[1m", updateLine, reset)
		} else {
			fmt.Printf("  %s%s%s\n", dim, updateLine, reset)
		}
		fmt.Println()
	}
}
