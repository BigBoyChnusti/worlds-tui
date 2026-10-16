package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
)

var (
	colBlue    = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#60a5fa"}
	colRed     = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f87171"}
	colAccent  = lipgloss.AdaptiveColor{Light: "#a16207", Dark: "#facc15"}
	colMuted   = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#6b7280"}
	colGood    = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"}
	colBorder  = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#374151"}
	colLiveDot = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ef4444"}

	stBold    = lipgloss.NewStyle().Bold(true)
	stMuted   = lipgloss.NewStyle().Foreground(colMuted)
	stBlue    = lipgloss.NewStyle().Foreground(colBlue).Bold(true)
	stRed     = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	stAccent  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	stGood    = lipgloss.NewStyle().Foreground(colGood)
	stLive    = lipgloss.NewStyle().Foreground(colLiveDot).Bold(true)
	stTitle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	stSection = lipgloss.NewStyle().Bold(true).Underline(true)

	stTab       = lipgloss.NewStyle().Padding(0, 1).Foreground(colMuted)
	stTabActive = lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(colAccent).Underline(true)

	stBox         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colBorder).Padding(0, 1)
	stBoxSelected = stBox.BorderForeground(colAccent)
	stBoxLive     = stBox.BorderForeground(colLiveDot)
)

// dragonStyle colours a dragon type and gives it a one-letter tag.
func dragonTag(kind string) string {
	var c lipgloss.TerminalColor
	var tag string
	switch kind {
	case "ocean":
		c, tag = lipgloss.Color("#38bdf8"), "O"
	case "mountain":
		c, tag = lipgloss.Color("#a8794c"), "M"
	case "infernal":
		c, tag = lipgloss.Color("#f97316"), "I"
	case "cloud":
		c, tag = lipgloss.Color("#e5e7eb"), "C"
	case "hextech":
		c, tag = lipgloss.Color("#22d3ee"), "H"
	case "chemtech":
		c, tag = lipgloss.Color("#84cc16"), "T"
	case "elder":
		c, tag = lipgloss.Color("#c084fc"), "E"
	default:
		c, tag = colMuted, "?"
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render("◆" + tag)
}

// fmtGold renders gold as 12.3k.
func fmtGold(g int) string {
	if g >= 1000 || g <= -1000 {
		return fmt.Sprintf("%.1fk", float64(g)/1000)
	}
	return fmt.Sprintf("%d", g)
}

// fmtDiff renders a signed gold difference, coloured by the leading side.
func fmtDiff(d int) string {
	switch {
	case d > 0:
		return stBlue.Render("+" + fmtGold(d))
	case d < 0:
		return stRed.Render("+" + fmtGold(-d))
	}
	return stMuted.Render("even")
}

// pad fits s into exactly w cells, truncating with an ellipsis.
func pad(s string, w int) string {
	if lipgloss.Width(s) > w {
		r := []rune(s)
		for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
			r = r[:len(r)-1]
		}
		s = string(r) + "…"
	}
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}

// padLeft right-aligns s in w cells.
func padLeft(s string, w int) string {
	return strings.Repeat(" ", max(0, w-lipgloss.Width(s))) + s
}

func bestOf(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("Bo%d", n)
}

// slotLine renders "CODE  wins" for a bracket slot, bold for the winner and
// dimmed for the loser.
func slotLine(s bracket.Slot, width int, state string) string {
	code := s.Code
	if !s.Known() {
		code = "TBD"
	}
	score := ""
	if state != esports.StateUnstarted && s.Known() {
		score = fmt.Sprintf("%d", s.Wins)
	}
	line := pad(code, width-2) + padLeft(score, 2)
	switch s.Outcome {
	case "win":
		return stBold.Render(line)
	case "loss":
		return stMuted.Render(line)
	}
	if !s.Known() {
		return stMuted.Render(line)
	}
	return line
}

// matchBox draws a bracket match as a bordered two-line box.
func matchBox(m *bracket.Match, label string, selected bool) string {
	const w = 10
	if m == nil {
		return stBox.Render(stMuted.Render(pad("TBD", w)) + "\n" + stMuted.Render(pad("TBD", w)))
	}
	body := slotLine(m.Teams[0], w, m.State) + "\n" + slotLine(m.Teams[1], w, m.State)
	style := stBox
	switch {
	case selected:
		style = stBoxSelected
	case m.State == esports.StateInProgress:
		style = stBoxLive
	}
	box := style.Render(body)
	if label != "" {
		box = stMuted.Render(pad(label, w+4)) + "\n" + box
	}
	return box
}

// matchStateTag is a short marker for schedule lists.
func matchStateTag(state string) string {
	switch state {
	case esports.StateInProgress:
		return stLive.Render("● LIVE")
	case esports.StateCompleted:
		return stMuted.Render("final ")
	}
	return "      "
}
