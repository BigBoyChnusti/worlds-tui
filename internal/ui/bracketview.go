package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
)

const (
	boxH    = 4 // rendered height of a match box
	boxW    = 14
	boxExit = 2 // row inside a box where connectors attach
)

// placed is a block of text positioned at a row of a column.
type placed struct {
	top  int
	text string
}

// column lays blocks out in a fixed-size column of lines.
func column(items []placed, width, height int) []string {
	lines := make([]string, height)
	for _, it := range items {
		for i, l := range strings.Split(it.text, "\n") {
			if r := it.top + i; r >= 0 && r < height {
				lines[r] = l
			}
		}
	}
	for i, l := range lines {
		lines[i] = l + strings.Repeat(" ", max(0, width-lipgloss.Width(l)))
	}
	return lines
}

// link joins two exit rows a < b into an entry row t between them.
type link struct{ a, b, t int }

// connectors draws a three-cell-wide column of bracket lines. A link with
// a == b is a straight line.
func connectors(height int, links []link) []string {
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = []rune("   ")
	}
	for _, l := range links {
		if l.a == l.b {
			if l.a < height {
				grid[l.a] = []rune("───")
			}
			continue
		}
		for r := l.a; r <= l.b && r < height; r++ {
			grid[r][1] = '│'
		}
		grid[l.a] = []rune("─┐ ")
		grid[l.b] = []rune("─┘ ")
		if l.t > l.a && l.t < l.b {
			grid[l.t] = []rune(" ├─")
		}
	}
	out := make([]string, height)
	for i, r := range grid {
		out[i] = stMuted.Render(string(r))
	}
	return out
}

func joinColumns(cols ...[]string) string {
	if len(cols) == 0 {
		return ""
	}
	var b strings.Builder
	for r := range cols[0] {
		for _, c := range cols {
			if r < len(c) {
				b.WriteString(c[r])
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func headerRow(widths []int, labels []string) string {
	var b strings.Builder
	for i, w := range widths {
		l := ""
		if i < len(labels) {
			l = labels[i]
		}
		b.WriteString(stSection.Render(l) + strings.Repeat(" ", max(0, w-lipgloss.Width(l))))
	}
	return b.String()
}

func (m Model) selectedID() string {
	ids := m.tabMatchIDs(m.tab)
	if c := m.cursors[m.tab]; c < len(ids) {
		return ids[c]
	}
	return ""
}

func (m Model) box(x *bracket.Match) string {
	return matchBox(x, "", x != nil && x.ID == m.selectedID())
}

// playInOrder is the cursor order of Play-In matches.
func (m Model) playInOrder() []*bracket.Match {
	p := m.bracket.PlayIn
	return []*bracket.Match{p.Openers[0], p.Openers[1], p.LowerRound1, p.UpperFinal, p.LowerFinal, p.Final}
}

func (m Model) renderPlayIn() (string, int) {
	p := m.bracket.PlayIn
	if len(p.All) == 0 {
		return stMuted.Render("No Play-In data yet."), -1
	}
	const h = 17
	// Upper bracket rows 0-8, lower bracket from row 12.
	o1, o2, lr1 := 0, 5, 12
	uf := (o1 + o2) / 2 // centred between the openers
	lf := lr1
	gf := (uf + lf) / 2

	col0 := column([]placed{{o1, m.box(p.Openers[0])}, {o2, m.box(p.Openers[1])}, {lr1, m.box(p.LowerRound1)}}, boxW, h)
	con0 := connectors(h, []link{{o1 + boxExit, o2 + boxExit, uf + boxExit}, {lr1 + boxExit, lr1 + boxExit, 0}})
	col1 := column([]placed{{uf, m.box(p.UpperFinal)}, {lf, m.box(p.LowerFinal)}}, boxW, h)
	con1 := connectors(h, []link{{uf + boxExit, lf + boxExit, gf + boxExit}})
	col2 := column([]placed{{gf, m.box(p.Final)}}, boxW, h)

	header := headerRow([]int{boxW + 3, boxW + 3, boxW}, []string{"Round 1", "Round 2", "Final"})
	side := column([]placed{{o1 + 1, stMuted.Render("Upper")}, {lr1 + 1, stMuted.Render("Lower")}}, 7, h)
	body := joinColumns(side, col0, con0, col1, con1, col2)

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", 7) + header + "\n")
	b.WriteString(body)
	b.WriteString(stMuted.Render("Openers' winners meet in the upper final, losers in the lower round. The upper\n"+
		"final loser drops to the lower final; the final's winner advances to the Swiss Stage.") + "\n\n")
	b.WriteString(m.matchList(p.All))

	focus := -1
	sel := m.selectedID()
	tops := map[*bracket.Match]int{p.Openers[0]: o1, p.Openers[1]: o2, p.LowerRound1: lr1, p.UpperFinal: uf, p.LowerFinal: lf, p.Final: gf}
	for x, top := range tops {
		if x != nil && x.ID == sel {
			focus = top + boxH
		}
	}
	return b.String(), focus
}

// matchList prints matches with their times, for detail below a bracket.
func (m Model) matchList(ms []*bracket.Match) string {
	var b strings.Builder
	for _, x := range ms {
		if x == nil {
			continue
		}
		when := ""
		if !x.Start.IsZero() {
			when = x.Start.Local().Format("Mon 02 Jan 15:04")
		}
		fmt.Fprintf(&b, "  %s  %s  %s %s\n", stMuted.Render(pad(when, 16)), matchStateTag(x.State),
			matchLine(x, false), stMuted.Render(bestOf(x.BestOf)))
	}
	return b.String()
}

// matchLine renders "T1   2–1  GEN" with the winner bold.
func matchLine(x *bracket.Match, selected bool) string {
	code := func(i int) string {
		s := x.Teams[i]
		c := "TBD"
		if s.Known() {
			c = s.Code
		}
		st := lipgloss.NewStyle()
		switch {
		case s.Outcome == "win":
			st = stBold
		case s.Outcome == "loss" || !s.Known():
			st = stMuted
		}
		return st.Render(c)
	}
	score := " vs "
	if x.State != esports.StateUnstarted && x.Teams[0].Known() {
		score = fmt.Sprintf(" %d–%d", x.Teams[0].Wins, x.Teams[1].Wins)
		score += " "
	}
	marker := "  "
	if selected {
		marker = stAccent.Render("▸ ")
	}
	live := ""
	if x.State == esports.StateInProgress {
		live = " " + stLive.Render("●")
	}
	return marker + padLeft(code(0), 5) + score + pad(code(1), 5) + live
}

func (m Model) renderSwiss() (string, int) {
	s := m.bracket.Swiss
	if len(s.Rounds) == 0 {
		return stMuted.Render("No Swiss data yet."), -1
	}
	sel := m.selectedID()
	const colW = 24
	var cols [][]string
	focusRow := -1
	for ri, round := range s.Rounds {
		lines := []string{stSection.Render(fmt.Sprintf("Round %d", ri+1))}
		for _, g := range poolGroups(s, round) {
			lines = append(lines, stMuted.Render(g.label))
			for _, x := range g.matches {
				if x.ID == sel {
					focusRow = len(lines)
				}
				lines = append(lines, matchLine(x, x.ID == sel))
			}
		}
		cols = append(cols, lines)
	}

	var b strings.Builder
	focus := -1
	if len(cols)*colW <= m.width {
		height := 0
		for _, c := range cols {
			height = max(height, len(c))
		}
		var padded [][]string
		for _, c := range cols {
			var ps []placed
			for i, l := range c {
				ps = append(ps, placed{i, l})
			}
			padded = append(padded, column(ps, colW, height))
		}
		b.WriteString(joinColumns(padded...))
		focus = focusRow
	} else {
		row := 0
		for ri, c := range cols {
			for i, l := range c {
				if i == focusRow && containsID(s.Rounds[ri], sel) {
					focus = row
				}
				b.WriteString(l + "\n")
				row++
			}
			b.WriteString("\n")
			row++
		}
	}

	b.WriteString("\n" + stSection.Render("Standings") + "\n")
	b.WriteString(renderRecords(s))
	return b.String(), focus
}

func containsID(ms []*bracket.Match, id string) bool {
	for _, x := range ms {
		if x.ID == id {
			return true
		}
	}
	return false
}

type poolGroup struct {
	label   string
	matches []*bracket.Match
}

// poolGroups splits a round by the record teams carried into each match.
func poolGroups(s bracket.Swiss, round []*bracket.Match) []poolGroup {
	var out []poolGroup
	index := map[string]int{}
	for _, x := range round {
		label := "TBD"
		if r, ok := s.Pools[x.ID]; ok {
			label = fmt.Sprintf("%d–%d", r.Wins, r.Losses)
		}
		label += "  " + bestOf(x.BestOf)
		i, ok := index[label]
		if !ok {
			i = len(out)
			index[label] = i
			out = append(out, poolGroup{label: label})
		}
		out[i].matches = append(out[i].matches, x)
	}
	return out
}

// renderRecords lists teams grouped by their current Swiss record.
func renderRecords(s bracket.Swiss) string {
	if len(s.Teams) == 0 {
		return stMuted.Render("Teams are decided once the Play-In finishes.") + "\n"
	}
	type bucket struct {
		r     bracket.Record
		teams []string
	}
	var buckets []*bucket
	byRec := map[bracket.Record]*bucket{}
	for _, t := range s.Teams {
		bk := byRec[t.Record]
		if bk == nil {
			bk = &bucket{r: t.Record}
			byRec[t.Record] = bk
			buckets = append(buckets, bk)
		}
		bk.teams = append(bk.teams, t.Code)
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		a, b := buckets[i].r, buckets[j].r
		if a.Wins != b.Wins {
			return a.Wins > b.Wins
		}
		return a.Losses < b.Losses
	})
	var b strings.Builder
	for _, bk := range buckets {
		label := fmt.Sprintf("%d–%d", bk.r.Wins, bk.r.Losses)
		teams := strings.Join(bk.teams, "  ")
		switch {
		case bk.r.Advanced():
			b.WriteString(stGood.Render(pad(label, 6)+"✓ "+teams) + stMuted.Render("  advanced") + "\n")
		case bk.r.Eliminated():
			b.WriteString(stMuted.Render(pad(label, 6)+"✗ "+teams+"  eliminated") + "\n")
		default:
			b.WriteString(pad(label, 6) + "  " + teams + "\n")
		}
	}
	return b.String()
}

func (m Model) renderKnockout() (string, int) {
	k := m.bracket.Knockout
	if len(k.Quarterfinals) == 0 && len(k.Semifinals) == 0 && k.Final == nil {
		return stMuted.Render("No Knockout data yet."), -1
	}
	at := func(ms []*bracket.Match, i int) *bracket.Match {
		if i < len(ms) {
			return ms[i]
		}
		return nil
	}
	const h = 4*(boxH+1) - 1
	qfTop := []int{0, 5, 10, 15}
	sfTop := []int{(qfTop[0] + qfTop[1]) / 2, (qfTop[2] + qfTop[3]) / 2}
	fTop := (sfTop[0] + sfTop[1]) / 2

	var qf, sf []placed
	for i, t := range qfTop {
		qf = append(qf, placed{t, m.box(at(k.Quarterfinals, i))})
	}
	for i, t := range sfTop {
		sf = append(sf, placed{t, m.box(at(k.Semifinals, i))})
	}
	con0 := connectors(h, []link{
		{qfTop[0] + boxExit, qfTop[1] + boxExit, sfTop[0] + boxExit},
		{qfTop[2] + boxExit, qfTop[3] + boxExit, sfTop[1] + boxExit},
	})
	con1 := connectors(h, []link{{sfTop[0] + boxExit, sfTop[1] + boxExit, fTop + boxExit}})

	header := headerRow([]int{boxW + 3, boxW + 3, boxW}, []string{"Quarterfinals", "Semifinals", "Final"})
	body := joinColumns(column(qf, boxW, h), con0, column(sf, boxW, h), con1,
		column([]placed{{fTop, m.box(k.Final)}}, boxW, h))

	var all []*bracket.Match
	all = append(all, k.Quarterfinals...)
	all = append(all, k.Semifinals...)
	if k.Final != nil {
		all = append(all, k.Final)
	}

	focus := -1
	sel := m.selectedID()
	for i, x := range k.Quarterfinals {
		if x.ID == sel && i < len(qfTop) {
			focus = qfTop[i] + boxH
		}
	}
	for i, x := range k.Semifinals {
		if x.ID == sel && i < len(sfTop) {
			focus = sfTop[i] + boxH
		}
	}
	if k.Final != nil && k.Final.ID == sel {
		focus = fTop + boxH
	}
	return header + "\n" + body + "\n" + m.matchList(all), focus
}

func (m Model) renderSchedule() (string, int) {
	if len(m.events) == 0 {
		return stMuted.Render("No schedule yet."), -1
	}
	sel := m.cursors[tabSchedule]
	var b strings.Builder
	focus, row := -1, 0
	lastDay := ""
	for i, e := range m.events {
		day := e.StartTime.Local().Format("Mon 02 Jan")
		if day != lastDay {
			if lastDay != "" {
				b.WriteString("\n")
				row++
			}
			b.WriteString(stSection.Render(day) + "\n")
			row++
			lastDay = day
		}
		x := scheduleMatch(e)
		if i == sel {
			focus = row
		}
		fmt.Fprintf(&b, "%s  %s %s  %s\n", stMuted.Render(e.StartTime.Local().Format("15:04")),
			matchStateTag(e.State), matchLine(x, i == sel), stMuted.Render(pad(e.BlockName, 14)+" "+bestOf(x.BestOf)))
		row++
	}
	return b.String(), focus
}

// scheduleMatch adapts a schedule event to the bracket match type.
func scheduleMatch(e esports.Event) *bracket.Match {
	x := &bracket.Match{ID: e.Match.ID, State: e.State, Start: e.StartTime, Block: e.BlockName, BestOf: e.Match.Strategy.Count}
	for i := 0; i < 2 && i < len(e.Match.Teams); i++ {
		t := e.Match.Teams[i]
		x.Teams[i] = bracket.Slot{Code: t.Code, Name: t.Name}
		if t.Result != nil {
			x.Teams[i].Wins, x.Teams[i].Outcome = t.Result.GameWins, t.Result.Outcome
		}
	}
	return x
}
