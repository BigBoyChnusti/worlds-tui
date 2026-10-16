package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
)

// trophyArt is the Summoner's Cup. The gem marker is coloured separately.
const trophyArt = `        .-~~~~~~~~~~~~~~~~~~~~~~~~-.
       (____________________________)
        \\                        //
  .-.    \\                      //    .-.
 / .-\    \\        .--.        //    /-. \
| |   \    \\      / /\ \      //    /   | |
| |    \    \\    | <()> |    //    /    | |
 \ \    \    \\    \ \/ /    //    /    / /
  \ \    \    \\    '--'    //    /    / /
   '.'.   \    \\          //    /   .'.'
     '.'.  \    \\        //    /  .'.'
       '-.'-.\   \\      //   /.-'.-'
          '-.'-\  \\    //  /-'.-'
              '-\  \\  //  /-'
                 '. \\// .'
                   ) || (
                  (  ||  )
                   '.||.'
                    _||_
                 .-'    '-.
               _/__________\_
              |______________|
             /________________\`

const trophyGem = "<()>"

// logoGlyphs are the rows of each letter of the block logo.
var logoGlyphs = map[rune][6]string{
	'W': {"██╗    ██╗", "██║    ██║", "██║ █╗ ██║", "██║███╗██║", "╚███╔███╔╝", " ╚══╝╚══╝ "},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'R': {"██████╗ ", "██╔══██╗", "██████╔╝", "██╔══██╗", "██║  ██║", "╚═╝  ╚═╝"},
	'L': {"██╗     ", "██║     ", "██║     ", "██║     ", "███████╗", "╚══════╝"},
	'D': {"██████╗ ", "██╔══██╗", "██║  ██║", "██║  ██║", "██████╔╝", "╚═════╝ "},
	'S': {"███████╗", "██╔════╝", "███████╗", "╚════██║", "███████║", "╚══════╝"},
	'-': {"      ", "      ", "█████╗", "╚════╝", "      ", "      "},
	'T': {"████████╗", "╚══██╔══╝", "   ██║   ", "   ██║   ", "   ██║   ", "   ╚═╝   "},
	'U': {"██╗   ██╗", "██║   ██║", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'I': {"██╗", "██║", "██║", "██║", "██║", "╚═╝"},
}

// Gold rows for WORLDS, blue rows for -TUI, light to dark.
var (
	logoGold = []string{"#fef3c7", "#fde68a", "#fcd34d", "#fbbf24", "#f59e0b", "#b45309"}
	logoBlue = []string{"#dbeafe", "#bfdbfe", "#93c5fd", "#60a5fa", "#3b82f6", "#1d4ed8"}
	stSilver = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#d1d5db"})
	stGem    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Bold(true)
)

func renderWord(word string, row int, palette []string) string {
	var b strings.Builder
	for _, r := range word {
		b.WriteString(logoGlyphs[r][row])
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(palette[row])).Render(b.String())
}

// bigLogo renders WORLDS-TUI in block letters, 78 cells wide.
func bigLogo() string {
	rows := make([]string, 6)
	for i := range rows {
		rows[i] = renderWord("WORLDS", i, logoGold) + renderWord("-TUI", i, logoBlue)
	}
	return strings.Join(rows, "\n")
}

// smallLogo is the fallback for narrow terminals.
func smallLogo() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(logoGold[3])).Bold(true).Render("W O R L D S") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(logoBlue[3])).Bold(true).Render(" - T U I")
}

func trophy() string {
	lines := strings.Split(trophyArt, "\n")
	for i, l := range lines {
		if before, after, ok := strings.Cut(l, trophyGem); ok {
			lines[i] = stSilver.Render(before) + stGem.Render(trophyGem) + stSilver.Render(after)
		} else {
			lines[i] = stSilver.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

type menuItem struct {
	label string
	tab   int // -1 quits
}

var menuItems = []menuItem{
	{"Live", tabLive},
	{"Play-In", tabPlayIn},
	{"Swiss Stage", tabSwiss},
	{"Knockouts", tabKnockout},
	{"Schedule", tabSchedule},
	{"Quit", -1},
}

// stageStatus summarises a stage's progress for the menu.
func (m Model) stageStatus(ms []*bracket.Match) string {
	var total, done, live int
	var first *bracket.Match
	for _, x := range ms {
		if x == nil {
			continue
		}
		total++
		switch x.State {
		case esports.StateCompleted:
			done++
		case esports.StateInProgress:
			live++
		}
		if first == nil || (!x.Start.IsZero() && x.Start.Before(first.Start)) {
			first = x
		}
	}
	switch {
	case total == 0:
		return stMuted.Render("no data yet")
	case live > 0:
		return stLive.Render("● live") + stMuted.Render(fmt.Sprintf("  %d/%d played", done, total))
	case done == total:
		return stGood.Render("complete")
	case done == 0 && first != nil && first.Start.After(m.now):
		return stMuted.Render("starts " + first.Start.Local().Format("Mon 02 Jan"))
	}
	return stMuted.Render(fmt.Sprintf("%d/%d played", done, total))
}

func (m Model) liveStatus() string {
	if len(m.live) > 0 {
		s := stLive.Render("● " + eventTitle(m.live[0]) + " live")
		if len(m.live) > 1 {
			s += stMuted.Render(fmt.Sprintf(" +%d", len(m.live)-1))
		}
		return s
	}
	for _, e := range m.events {
		if e.State == esports.StateUnstarted && e.StartTime.After(m.now) {
			return stMuted.Render(fmt.Sprintf("next %s in %s", eventTitle(e), fmtCountdown(e.StartTime.Sub(m.now))))
		}
	}
	if m.lastData.IsZero() {
		return stMuted.Render("loading…")
	}
	return stMuted.Render("no match live")
}

func (m Model) itemStatus(it menuItem) string {
	b := m.bracket
	switch it.tab {
	case tabLive:
		return m.liveStatus()
	case tabPlayIn:
		return m.stageStatus(b.PlayIn.All)
	case tabSwiss:
		var all []*bracket.Match
		for _, r := range b.Swiss.Rounds {
			all = append(all, r...)
		}
		return m.stageStatus(all)
	case tabKnockout:
		all := append(append([]*bracket.Match{}, b.Knockout.Quarterfinals...), b.Knockout.Semifinals...)
		if b.Knockout.Final != nil {
			all = append(all, b.Knockout.Final)
		}
		return m.stageStatus(all)
	case tabSchedule:
		if len(m.events) == 0 {
			return ""
		}
		return stMuted.Render(fmt.Sprintf("%d matches", len(m.events)))
	}
	return ""
}

func (m Model) renderMenuList() string {
	var lines []string
	for i, it := range menuItems {
		key := " "
		if it.tab >= 0 {
			key = fmt.Sprint(it.tab + 1)
		} else {
			key = "q"
		}
		label := pad(it.label, 13)
		marker := "  "
		if i == m.menuCursor {
			marker = stAccent.Render("▸ ")
			label = stAccent.Render(label)
		} else {
			label = stBold.Render(label)
		}
		lines = append(lines, m.zones.Mark(menuZone(i), marker+stMuted.Render(key+"  ")+label+" "+m.itemStatus(it)))
		if it.tab == tabSchedule {
			lines = append(lines, "")
		}
	}
	title := stTitle.Render("World Championship")
	if m.tournament.Slug != "" {
		title += stMuted.Render("  " + strings.ReplaceAll(m.tournament.Slug, "_", " "))
	}
	return stBox.Padding(1, 2).Render(title + "\n\n" + strings.Join(lines, "\n"))
}

func (m Model) renderMenu() string {
	help := stMuted.Render("↑/↓ or mouse select · enter or click open · 1-5 jump · esc back to menu from any view · q quit")
	list := m.renderMenuList()

	var logo string
	if m.width >= 82 {
		logo = bigLogo()
	} else {
		logo = smallLogo()
	}
	tagline := stMuted.Render("League of Legends World Championship, live in your terminal")

	art := trophy()
	gap := "    "
	row := lipgloss.JoinHorizontal(lipgloss.Center, art, gap, list)
	// Drop the trophy when it doesn't fit.
	full := lipgloss.JoinVertical(lipgloss.Center, logo, "", tagline, "", row, "", help)
	if lipgloss.Width(row) > m.width || lipgloss.Height(full) > m.height {
		full = lipgloss.JoinVertical(lipgloss.Center, logo, "", tagline, "", list, "", help)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, full)
}

func (m Model) handleMenuKey(k string) (Model, bool) {
	switch k {
	case "down", "j", "tab":
		m.menuCursor = (m.menuCursor + 1) % len(menuItems)
	case "up", "k", "shift+tab":
		m.menuCursor = (m.menuCursor + len(menuItems) - 1) % len(menuItems)
	case "enter", " ":
		it := menuItems[m.menuCursor]
		if it.tab < 0 {
			return m, true
		}
		m.tab, m.inMenu = it.tab, false
	case "1", "2", "3", "4", "5":
		m.tab, m.inMenu = int(k[0]-'1'), false
		m.menuCursor = m.tab
	}
	return m, false
}
