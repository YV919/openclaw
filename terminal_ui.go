package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ── Key event types (console_windows.go / console_other.go depend on these) ──

// KeyType represents the type of keyboard event.
type KeyType int

const (
	KeyOther KeyType = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
)

// rawModeState saves the terminal state while in Unix raw mode, for Ctrl+C
// path restoration. Also read by console_windows.go.
var rawModeState *term.State

// ── Theme: colors and icons (applyLegacyTheme degrades to ASCII on old consoles) ──

var (
	cReset   = "\033[0m"
	cRed     = "\033[91m"
	cGreen   = "\033[92m"
	cYellow  = "\033[93m"
	cCyan    = "\033[96m"
	cCyanMid = "\033[36m"
	cBlue    = "\033[94m"
	cMagenta = "\033[95m"
	cGray    = "\033[90m"
	cBold    = "\033[1m"
	cDim     = "\033[2m"
)

var (
	iconOK    = "✔"
	iconErr   = "✘"
	iconWarn  = "⚠"
	iconInfo  = "→"
	iconTip   = "◆"
	iconArrow = "❯"
	arrowUp   = "↑"
	arrowDown = "↓"
	// Single-line rounded box (for menus)
	boxTL = "╭"
	boxTR = "╮"
	boxBL = "╰"
	boxBR = "╯"
	boxH  = "─"
	boxV  = "│"
	boxML = "├"
	boxMR = "┤"
	// Double-line box (for config summaries) — defined as constants in output.go
	// boxDTL, boxDTR, boxDBL, boxDBR, boxDH, boxDV, boxDML, boxDMR
)

// sectionStart is the section header prefix; degraded by applyLegacyTheme.
var sectionStart = "┌─"

// applyLegacyTheme clears colors and downgrades icons to ASCII for consoles
// that don't support ANSI/VT.
func applyLegacyTheme() {
	cReset, cRed, cGreen, cYellow, cCyan, cCyanMid, cBlue, cMagenta, cGray, cBold, cDim = "", "", "", "", "", "", "", "", "", "", ""
	iconOK, iconErr, iconWarn, iconInfo, iconTip, iconArrow = "[OK]", "[X]", "[!]", "->", "*", ">"
	arrowUp, arrowDown = "^", "v"
	boxTL, boxTR, boxBL, boxBR = "+", "+", "+", "+"
	boxH, boxV = "-", "|"
	boxML, boxMR = "+", "+"

	sectionStart = ">>"
}

// ── CJK width awareness ──

var cjkAmbiguous bool

// detectCJKLocale detects CJK locales and decides whether East Asian Ambiguous
// characters should be rendered at width 2.
func detectCJKLocale() {
	for _, env := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := strings.ToLower(os.Getenv(env))
		if strings.Contains(v, "zh") || strings.Contains(v, "ja") || strings.Contains(v, "ko") {
			cjkAmbiguous = true
			return
		}
	}
	switch getWindowsACP() {
	case 936, 950, 932, 949: // GBK / Big5 / Japanese / Korean
		cjkAmbiguous = true
	}
}

func isAmbiguous(r rune) bool {
	switch r {
	// Note: '❯' is NOT here — Windows Terminal renders it as width 1 text.
	// Treating it as ambiguous width 2 would break box alignment.
	case '◆', '✔', '✘', '⚠', '↑', '↓', '→', '★', '●', '○':
		return true
	}
	return false
}

// wideEmoji determines whether a rune below 0x1F300 is an emoji-presentation
// character (displayed at width 2). Text presentation symbols like ❯(276F)/✔(2714)
// are not included.
func wideEmoji(r rune) bool {
	switch {
	case r >= 0x231A && r <= 0x231B,
		r >= 0x23E9 && r <= 0x23EC,
		r == 0x23F0, r == 0x23F3,
		r >= 0x25FD && r <= 0x25FE,
		r >= 0x2614 && r <= 0x2615,
		r >= 0x2648 && r <= 0x2653,
		r == 0x267F, r == 0x2693, r == 0x26A1,
		r >= 0x26AA && r <= 0x26AB,
		r >= 0x26BD && r <= 0x26BE,
		r >= 0x26C4 && r <= 0x26C5,
		r == 0x26CE, r == 0x26D4, r == 0x26EA,
		r >= 0x26F2 && r <= 0x26F3,
		r == 0x26F5, r == 0x26FA, r == 0x26FD,
		r == 0x2705,
		r >= 0x270A && r <= 0x270B,
		r == 0x2728, r == 0x274C, r == 0x274E,
		r >= 0x2753 && r <= 0x2755,
		r == 0x2757,
		r >= 0x2795 && r <= 0x2797,
		r == 0x27B0, r == 0x27BF,
		r >= 0x2B1B && r <= 0x2B1C,
		r == 0x2B50, r == 0x2B55:
		return true
	}
	return false
}

// utf8Size returns the total byte count (1~4) for a UTF-8 character given its
// leading byte. Invalid leading bytes return 1.
// Used by Unix raw-mode byte-by-byte reading to fetch continuation bytes.
// Defined in a platform-independent file for cross-platform compilation and testing.
func utf8Size(b byte) int {
	switch {
	case b&0x80 == 0:
		return 1
	case b&0xE0 == 0xC0:
		return 2
	case b&0xF0 == 0xE0:
		return 3
	case b&0xF8 == 0xF0:
		return 4
	default:
		return 1
	}
}

// runeWidth returns the display width of a single rune in the terminal (0, 1, or 2).
func runeWidth(r rune) int {
	if r == 0 {
		return 0
	}
	// Variant selectors / zero-width joiner: no columns (VS16 promotion handled in visibleLength)
	if r == 0xFE0F || r == 0xFE0E || r == 0x200D {
		return 0
	}
	if (r >= 0x1100 && r <= 0x115F) || // Hangul Jamo
		(r >= 0x2E80 && r <= 0xA4CF) || // CJK Radicals ~ Yi
		(r >= 0xAC00 && r <= 0xD7A3) || // Hangul Syllables
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0xFE30 && r <= 0xFE4F) || // CJK Compatibility Forms
		(r >= 0xFF00 && r <= 0xFF60) || // Fullwidth Forms
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x1F300 && r <= 0x1FAFF) || // Emoji
		(r >= 0x20000 && r <= 0x3FFFD) || // CJK Extension B+
		wideEmoji(r) { // Emoji presentation below 0x1F300
		return 2
	}
	if cjkAmbiguous && isAmbiguous(r) {
		return 2
	}
	return 1
}

// visibleLength calculates the visible width of a string, ignoring ANSI escape codes.
// Handles VS16 (U+FE0F) promotion: if the preceding base character was width 1 in
// text presentation, it gets promoted to emoji width 2.
func visibleLength(s string) int {
	w := 0
	inEsc := false
	prev := 0 // width of the last visible character
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		if r == 0xFE0F {
			if prev == 1 { // text-presentation base promoted to emoji, 1→2
				w++
				prev = 2
			}
			continue
		}
		rw := runeWidth(r)
		w += rw
		prev = rw
	}
	return w
}

// ── Color print helpers ──

func printColor(color, text string) { fmt.Printf("%s%s%s\n", color, text, cReset) }
func printSuccessMsg(t string) { fmt.Printf("%s%s%s %s\n", cGreen, iconOK, cReset, t) }
func printError(t string)           { fmt.Printf("%s%s%s %s\n", cRed, iconErr, cReset, t) }
func printWarning(t string)         { fmt.Printf("%s%s%s %s\n", cYellow, iconWarn, cReset, t) }
func printInfo(t string)            { fmt.Printf("%s%s%s %s\n", cCyan, iconInfo, cReset, t) }
func printTip(t string)             { fmt.Printf("%s%s%s %s\n", cBlue, iconTip, cReset, t) }

// clearScreen clears the terminal and moves the cursor to the top-left corner,
// implementing "page-style" navigation.
// Non-terminal (pipe/test) returns immediately to avoid ANSI garbage.
// Legacy console falls back to newline scrolling.
func clearScreen() {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return
	}
	if legacyConsoleMode {
		fmt.Print(strings.Repeat("\n", 50))
		return
	}
	fmt.Print("\033[2J\033[3J\033[H")
}

// waitKey shows a prompt and waits for the user to press any key (for action result
// pause; the caller will then clear screen and return).
// Non-terminal returns immediately to avoid blocking.
func waitKey(prompt string) {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return
	}
	fmt.Printf("\n%s%s%s", cDim, prompt, cReset)
	readMenuKey()
	fmt.Println()
}

// waitReturn is the unified pause after an action completes: view the result,
// then press a key to return (caller clears screen to go back).
func waitReturn() { waitKey("按任意键返回…") }

// printSectionHeader prints a formatted section header with a prefix.
func printSectionHeader(t string) {
	fmt.Println()
	fmt.Printf("%s%s%s %s%s%s\n", cBlue, sectionStart, cReset, cBold, t, cReset)
}

// maskKey masks a secret key: ≤8 chars returns fixed 8 asterisks; otherwise
// first 4 + "..." + last 4.
func maskKey(s string) string {
	r := []rune(s)
	if len(r) <= 8 {
		return "********"
	}
	return string(r[:4]) + "..." + string(r[len(r)-4:])
}

// ── Input helpers ──
// All text input (including secrets) uses a single bufio reader to avoid mixing
// with ReadConsoleInputW or term.ReadPassword, which could cause pre-read byte
// loss or empty reads on some Windows consoles.

var stdinReader = bufio.NewReader(os.Stdin)

func readLine() string {
	s, err := stdinReader.ReadString('\n')
	if err != nil && s == "" {
		return ""
	}
	return strings.TrimRight(s, "\r\n")
}

// readLineEsc reads a line interactively with ESC cancellation support
// (returns escaped=true). masked=true echoes * for each character.
// Non-terminal (pipe/test) falls back to full-line bufio reading and never
// reports ESC, preserving existing test and CI behavior.
func readLineEsc(masked bool) (string, bool) {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return readLine(), false
	}
	return readLineRaw(masked)
}

// styledInput reads ordinary text input. The second return value is true if
// the user pressed ESC to cancel.
func styledInput(label string) (string, bool) {
	fmt.Printf("%s%s%s %s: ", cCyan, iconArrow, cReset, label)
	v, esc := readLineEsc(false)
	if esc {
		return "", true
	}
	return strings.TrimSpace(v), false
}

// styledInputDefault reads input with a default value. Pressing Enter uses the
// default. ESC cancellation returns (def, true).
func styledInputDefault(label, def string) (string, bool) {
	if def != "" {
		fmt.Printf("%s%s%s %s [%s%s%s]: ", cCyan, iconArrow, cReset, label, cGray, def, cReset)
	} else {
		fmt.Printf("%s%s%s %s: ", cCyan, iconArrow, cReset, label)
	}
	v, esc := readLineEsc(false)
	if esc {
		return def, true
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return def, false
	}
	return v, false
}

// readPassword reads a secret: in interactive mode reads character by character
// echoing * (truly hidden), supports ESC cancellation and backspace.
// Non-interactive (pipe/test) falls back to full-line reading.
// The second return value is true if the user pressed ESC to cancel.
func readPassword(label string) (string, bool) {
	if !term.IsTerminal(int(syscall.Stdin)) {
		fmt.Printf("%s%s%s %s: ", cCyan, iconArrow, cReset, label)
		return strings.TrimSpace(readLine()), false
	}
	fmt.Printf("%s%s%s %s: ", cCyan, iconArrow, cReset, label)
	v, esc := readLineEsc(true)
	if esc {
		return "", true
	}
	return strings.TrimSpace(v), false
}

// ── Menu: arrow key navigation (legacy console falls back to number selection) ──

type menuItem struct {
	Label string
	Desc  string
}

// selectMenu displays a menu with up/down arrow selection, returning the selected
// index. ESC returns -1. Default highlight is on item 0.
func selectMenu(title string, items []menuItem) int {
	return selectMenuFrom(title, items, 0)
}

// selectMenuFrom is like selectMenu but allows specifying the initial highlight
// index (e.g., for confirm menus defaulting to "No").
func selectMenuFrom(title string, items []menuItem, defaultIdx int) int {
	if len(items) == 0 {
		return -1
	}
	if defaultIdx < 0 || defaultIdx >= len(items) {
		defaultIdx = 0
	}
	if legacyConsoleMode {
		return selectMenuNumbered(title, items)
	}
	idx := defaultIdx
	drawMenu(title, items, idx, false)
	for {
		switch readMenuKey() {
		case KeyUp:
			idx = (idx - 1 + len(items)) % len(items)
			drawMenu(title, items, idx, true)
		case KeyDown:
			idx = (idx + 1) % len(items)
			drawMenu(title, items, idx, true)
		case KeyEnter:
			return idx
		case KeyEsc:
			return -1
		}
	}
}

// styledConfirm pops a "Yes/No" confirm menu. Returns (yes, escaped).
// ESC→(false,true): caller goes back; No→(false,false); Yes→(true,false).
func styledConfirm(label string, defaultYes bool) (yes, escaped bool) {
	items := []menuItem{
		{Label: "是", Desc: "确认"},
		{Label: "否", Desc: "取消 / 保持不变"},
	}
	def := 1
	if defaultYes {
		def = 0
	}
	idx := selectMenuFrom(label, items, def)
	if idx < 0 {
		return false, true
	}
	return idx == 0, false
}

// drawMenu renders the menu in a single-line rounded box: top centered title +
// separator + items (label/description two-column aligned), with the selected
// item highlighted in cyan with ❯. Below the box is a dim operation hint line.
// Box width does not change with selection to enable in-place redraw.
func drawMenu(title string, items []menuItem, sel int, redraw bool) {
	// Label column fixed width (by visible width), description in separate column
	labelW := 0
	for _, it := range items {
		if w := visibleLength(it.Label); w > labelW {
			labelW = w
		}
	}
	// Pre-render each item content (with colors), track max visible width
	lines := make([]string, len(items))
	contentW := visibleLength(title)
	for i, it := range items {
		prefix := "  "
		label := it.Label
		if i == sel {
			prefix = cCyan + iconArrow + cReset + " "
			label = cCyan + cBold + it.Label + cReset
		}
		pad := labelW - visibleLength(it.Label)
		if pad < 0 {
			pad = 0
		}
		desc := ""
		if it.Desc != "" {
			desc = "  " + cDim + it.Desc + cReset
		}
		lines[i] = prefix + label + strings.Repeat(" ", pad) + desc
		if w := visibleLength(lines[i]); w > contentW {
			contentW = w
		}
	}

	inner := contentW + 2 // 1 space padding on each side
	bar := strings.Repeat(boxH, inner)

	total := len(items) + 5 // top + title + separator + bottom + hint + N items
	if redraw {
		fmt.Printf("\033[%dA", total)
	}

	// Top
	fmt.Printf("\r\033[K%s%s%s%s%s\n", cCyan, boxTL, bar, boxTR, cReset)
	// Title centered
	tpad := inner - visibleLength(title)
	if tpad < 0 {
		tpad = 0
	}
	tl := tpad / 2
	fmt.Printf("\r\033[K%s%s%s%s%s%s%s%s%s\n",
		cCyan, boxV, cReset, strings.Repeat(" ", tl), cBold+title+cReset,
		strings.Repeat(" ", tpad-tl), cCyan, boxV, cReset)
	// Separator
	fmt.Printf("\r\033[K%s%s%s%s%s\n", cCyan, boxML, bar, boxMR, cReset)
	// Items
	for _, l := range lines {
		rpad := inner - visibleLength(l) - 1
		if rpad < 0 {
			rpad = 0
		}
		fmt.Printf("\r\033[K%s%s%s %s%s%s%s%s\n",
			cCyan, boxV, cReset, l, strings.Repeat(" ", rpad), cCyan, boxV, cReset)
	}
	// Bottom
	fmt.Printf("\r\033[K%s%s%s%s%s\n", cCyan, boxBL, bar, boxBR, cReset)
	// Hint (outside box, dim)
	fmt.Printf("\r\033[K%s  %s/%s 选择 · Enter 确认 · ESC 返回%s\n", cDim, arrowUp, arrowDown, cReset)
}

// selectMenuNumbered is a fallback for legacy consoles: shows numbered items
// and reads a number from input.
func selectMenuNumbered(title string, items []menuItem) int {
	fmt.Printf("%s%s%s\n", cBold, title, cReset)
	for i, it := range items {
		desc := ""
		if it.Desc != "" {
			desc = "  " + cDim + it.Desc + cReset
		}
		fmt.Printf("  %d. %s%s\n", i+1, it.Label, desc)
	}
	for {
		v, esc := styledInput("输入序号 (0 返回)")
		if esc || v == "0" || v == "" {
			return -1
		}
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 && n <= len(items) {
			return n - 1
		}
		printError("无效序号，请重新输入")
	}
}

// readMenuKey reads a navigation key in a cross-platform way.
func readMenuKey() KeyType {
	if runtime.GOOS == "windows" {
		return readConsoleKey()
	}
	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		// Cannot enter raw mode (e.g., pipe input), fall back to line reading
		line := strings.ToLower(readLine())
		switch line {
		case "", "q":
			return KeyEsc
		case "k", "w":
			return KeyUp
		case "j", "s":
			return KeyDown
		default:
			return KeyEnter
		}
	}
	rawModeState = oldState
	defer func() {
		term.Restore(int(syscall.Stdin), oldState)
		rawModeState = nil
	}()

	buf := make([]byte, 3)
	n, _ := os.Stdin.Read(buf)
	if n == 0 {
		return KeyOther
	}
	switch buf[0] {
	case 3: // Ctrl+C
		term.Restore(int(syscall.Stdin), oldState)
		rawModeState = nil
		restoreConsole()
		fmt.Println()
		os.Exit(130)
	case '\r', '\n':
		return KeyEnter
	case 27: // ESC or arrow key escape sequence
		if n >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return KeyUp
			case 'B':
				return KeyDown
			}
		}
		return KeyEsc
	case 'q', 'Q':
		return KeyEsc
	case 'k', 'K', 'w':
		return KeyUp
	case 'j', 'J', 's':
		return KeyDown
	}
	return KeyOther
}

// ── 步进式流程导航 ──

type stepResult int

const (
	stepNext stepResult = iota
	stepBack
	stepStay
	stepExit
)

func runSteps(steps []func() stepResult) {
	i := 0
	for i < len(steps) {
		clearScreen()
		switch steps[i]() {
		case stepNext:
			i++
		case stepBack:
			i--
			if i < 0 {
				return
			}
		case stepStay:
			// 重显本步
		case stepExit:
			return
		}
	}
}
