package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

// bigGlyphs are 6-row block digits in the style of the menu logo.
var bigGlyphs = map[rune][6]string{
	'0': {" ██████╗ ", "██╔═████╗", "██║██╔██║", "████╔╝██║", "╚██████╔╝", " ╚═════╝ "},
	'1': {" ██╗", "███║", "╚██║", " ██║", " ██║", " ╚═╝"},
	'2': {"██████╗ ", "╚════██╗", " █████╔╝", "██╔═══╝ ", "███████╗", "╚══════╝"},
	'3': {"██████╗ ", "╚════██╗", " █████╔╝", " ╚═══██╗", "██████╔╝", "╚═════╝ "},
	'4': {"██╗  ██╗", "██║  ██║", "███████║", "╚════██║", "     ██║", "     ╚═╝"},
	'5': {"███████╗", "██╔════╝", "███████╗", "╚════██║", "███████║", "╚══════╝"},
	'6': {" ██████╗ ", "██╔════╝ ", "███████╗ ", "██╔═══██╗", "╚██████╔╝", " ╚═════╝ "},
	'7': {"███████╗", "╚════██║", "    ██╔╝", "   ██╔╝ ", "   ██║  ", "   ╚═╝  "},
	'8': {" █████╗ ", "██╔══██╗", "╚█████╔╝", "██╔══██╗", "╚█████╔╝", " ╚════╝ "},
	'9': {" █████╗ ", "██╔══██╗", "╚██████║", " ╚═══██║", " █████╔╝", " ╚════╝ "},
	':': {"   ", "██╗", "╚═╝", "██╗", "╚═╝", "   "},
}

// midGlyphs are 3x5 pixel bitmaps drawn with half blocks into 3 rows.
var midGlyphs = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'.': {".", ".", ".", ".", "#"},
	'k': {"#..", "#.#", "##.", "#.#", "#.#"},
}

// bigText renders s in the 6-row font, one space between glyphs. The
// blocks take st's colour and the shadow lines are muted, like the logo.
func bigText(s string, st lipgloss.Style) []string {
	rows := make([]string, 6)
	shade := lipgloss.NewStyle().Foreground(colMuted)
	for i := range rows {
		var parts []string
		for _, r := range s {
			if g, ok := bigGlyphs[r]; ok {
				parts = append(parts, g[i])
			}
		}
		rows[i] = styleRuns(strings.Join(parts, " "), '█', st, shade)
	}
	return rows
}

// styleRuns renders the runs of on in st and everything else in off, one
// Render call per run.
func styleRuns(s string, on rune, st, off lipgloss.Style) string {
	var b, run strings.Builder
	cur := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		if cur {
			b.WriteString(st.Render(run.String()))
		} else if strings.TrimSpace(run.String()) == "" {
			b.WriteString(run.String())
		} else {
			b.WriteString(off.Render(run.String()))
		}
		run.Reset()
	}
	for _, r := range s {
		if (r == on) != cur {
			flush()
			cur = r == on
		}
		run.WriteRune(r)
	}
	flush()
	return b.String()
}

// midText renders s in the 3-row half-block font, one space between glyphs.
func midText(s string, st lipgloss.Style) []string {
	px := make([]string, 6) // pixel rows; the sixth stays empty
	first := true
	for _, r := range s {
		g, ok := midGlyphs[r]
		if !ok {
			continue
		}
		for i := range px {
			if !first {
				px[i] += "."
			}
			if i < 5 {
				px[i] += g[i]
			} else {
				px[i] += strings.Repeat(".", len(g[0]))
			}
		}
		first = false
	}
	rows := make([]string, 3)
	for i := range rows {
		top, bot := px[2*i], px[2*i+1]
		var b strings.Builder
		for j := range top {
			switch {
			case top[j] == '#' && bot[j] == '#':
				b.WriteRune('█')
			case top[j] == '#':
				b.WriteRune('▀')
			case bot[j] == '#':
				b.WriteRune('▄')
			default:
				b.WriteRune(' ')
			}
		}
		rows[i] = st.Render(b.String())
	}
	return rows
}

// blockWidth is the widest of rows.
func blockWidth(rows []string) int {
	w := 0
	for _, r := range rows {
		w = max(w, lipgloss.Width(r))
	}
	return w
}

// center places s in the middle of w cells.
func center(s string, w int) string {
	gap := max(0, w-lipgloss.Width(s))
	return strings.Repeat(" ", gap/2) + s + strings.Repeat(" ", gap-gap/2)
}

// scoreboardMinHeight is the terminal height below which the live view
// keeps the compact team table.
const scoreboardMinHeight = 40

// scoreboard renders the team summary as a broadcast-style scoreboard:
// big kills either side of the big game clock, then gold in a medium font
// and towers, inhibitors, barons and dragons mirrored around their labels.
// It returns "" when it doesn't fit in width.
func scoreboard(blueCode, redCode string, bt, rt livestats.TeamFrame, clock time.Duration, width int) string {
	blueKills := bigText(fmt.Sprint(bt.TotalKills), stBlue)
	redKills := bigText(fmt.Sprint(rt.TotalKills), stRed)
	clk := bigText(fmtClock(clock), stAccent)

	const gap = 5
	killW := max(blockWidth(blueKills), blockWidth(redKills))
	codeW := max(lipgloss.Width(blueCode), lipgloss.Width(redCode), 5)

	// The mirrored table under the clock.
	drakes := func(t livestats.TeamFrame) string {
		var d []string
		for _, k := range t.Dragons {
			d = append(d, dragonTag(k))
		}
		if len(d) == 0 {
			return stMuted.Render("–")
		}
		return strings.Join(d, " ")
	}
	type stat struct{ label, blue, red string }
	stats := []stat{
		{"TOWERS", stBlue.Render(fmt.Sprint(bt.Towers)), stRed.Render(fmt.Sprint(rt.Towers))},
		{"INHIBS", stBlue.Render(fmt.Sprint(bt.Inhibitors)), stRed.Render(fmt.Sprint(rt.Inhibitors))},
		{"BARONS", stBlue.Render(fmt.Sprint(bt.Barons)), stRed.Render(fmt.Sprint(rt.Barons))},
		{"DRAGONS", drakes(bt), drakes(rt)},
	}
	sideW := 0
	for _, s := range stats {
		sideW = max(sideW, lipgloss.Width(s.blue), lipgloss.Width(s.red))
	}
	var table []string
	for _, s := range stats {
		table = append(table, padLeft(s.blue, sideW)+"  "+stMuted.Render(center(s.label, 9))+"  "+pad(s.red, sideW))
	}

	sideCol := codeW + 2 + killW
	midCol := max(blockWidth(clk), blockWidth(table))
	total := 2*sideCol + 2*gap + midCol
	if total > width {
		return ""
	}
	indent := strings.Repeat(" ", (width-total)/2)
	sp := strings.Repeat(" ", gap)

	var b strings.Builder
	for i := range clk {
		left, right := "", ""
		if i == 2 {
			left, right = stBlue.Render(blueCode), stRed.Render(redCode)
		} else if i == 3 {
			left, right = stMuted.Render("kills"), stMuted.Render("kills")
		}
		b.WriteString(indent + pad(left, codeW) + "  " + padLeft(blueKills[i], killW) + sp +
			center(clk[i], midCol) + sp + pad(redKills[i], killW) + "  " + padLeft(right, codeW) + "\n")
	}
	b.WriteString("\n")

	// Gold in the medium font, the leader's gold bold-coloured and the
	// trailing side muted.
	blueSt, redSt := lipgloss.NewStyle().Foreground(colBlue), lipgloss.NewStyle().Foreground(colRed)
	blueGold, redGold := midText(fmtGold(bt.TotalGold), blueSt), midText(fmtGold(rt.TotalGold), redSt)
	goldLabel := func(diff int) string {
		if diff > 0 {
			return stMuted.Render("GOLD ") + stGood.Render("+"+fmtGold(diff))
		}
		return stMuted.Render("GOLD")
	}
	for i := range table {
		left, right := "", ""
		if i < 3 {
			left, right = blueGold[i], redGold[i]
		} else {
			left = goldLabel(bt.TotalGold - rt.TotalGold)
			right = goldLabel(rt.TotalGold - bt.TotalGold)
		}
		b.WriteString(indent + center(left, sideCol) + sp + center(table[i], midCol) + sp + center(right, sideCol) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
