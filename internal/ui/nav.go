package ui

import (
	"sort"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
)

// Row of the top of each box in the bracket layouts, shared by rendering
// and arrow-key navigation.
var (
	playInRows = struct{ opener1, opener2, lower1, upperFinal, lowerFinal, final int }{
		opener1: 0, opener2: 5, lower1: 12,
		upperFinal: 2, // centred between the openers
		lowerFinal: 12,
		final:      7, // centred between the upper and lower finals
	}
	quarterRows = []int{0, 5, 10, 15}
	semiRows    = []int{(quarterRows[0] + quarterRows[1]) / 2, (quarterRows[2] + quarterRows[3]) / 2}
	finalRow    = (semiRows[0] + semiRows[1]) / 2
)

// navCell is where a selectable match sits on screen: its column (bracket
// round) and its row within the view.
type navCell struct {
	id       string
	col, row int
}

// navCells maps the selectable matches of a tab to their positions.
func (m Model) navCells(tab int) []navCell {
	var cells []navCell
	add := func(x *bracket.Match, col, row int) {
		if x != nil {
			cells = append(cells, navCell{x.ID, col, row})
		}
	}
	switch tab {
	case tabPlayIn:
		p, r := m.bracket.PlayIn, playInRows
		add(p.Openers[0], 0, r.opener1)
		add(p.Openers[1], 0, r.opener2)
		add(p.LowerRound1, 0, r.lower1)
		add(p.UpperFinal, 1, r.upperFinal)
		add(p.LowerFinal, 1, r.lowerFinal)
		add(p.Final, 2, r.final)
	case tabSwiss:
		s := m.bracket.Swiss
		for ri, round := range s.Rounds {
			row := 1 // below the "Round N" heading
			for _, g := range poolGroups(s, round) {
				row++ // pool label
				for _, x := range g.matches {
					add(x, ri, row)
					row++
				}
			}
		}
	case tabKnockout:
		k := m.bracket.Knockout
		for i, x := range k.Quarterfinals {
			if i < len(quarterRows) {
				add(x, 0, quarterRows[i])
			}
		}
		for i, x := range k.Semifinals {
			if i < len(semiRows) {
				add(x, 1, semiRows[i])
			}
		}
		add(k.Final, 2, finalRow)
	case tabSchedule:
		for i, e := range m.events {
			cells = append(cells, navCell{e.Match.ID, 0, i})
		}
	}
	return cells
}

// navigate moves the selection on a bracket tab. dy steps up or down
// within the current column; dx jumps to the nearest match in the next
// non-empty column to the left or right.
func (m *Model) navigate(dx, dy int) {
	cells := m.navCells(m.tab)
	if len(cells) == 0 {
		return
	}
	cur := cells[0]
	sel := m.selectedID()
	for _, c := range cells {
		if c.id == sel {
			cur = c
		}
	}

	target := cur
	switch {
	case dy != 0:
		var col []navCell
		for _, c := range cells {
			if c.col == cur.col {
				col = append(col, c)
			}
		}
		sort.Slice(col, func(i, j int) bool { return col[i].row < col[j].row })
		for i, c := range col {
			if c.id == cur.id {
				target = col[min(len(col)-1, max(0, i+dy))]
			}
		}
	case dx != 0:
		minCol, maxCol := cells[0].col, cells[0].col
		for _, c := range cells {
			minCol, maxCol = min(minCol, c.col), max(maxCol, c.col)
		}
		for col := cur.col + dx; col >= minCol && col <= maxCol; col += dx {
			found := false
			for _, c := range cells {
				if c.col != col {
					continue
				}
				// Closest row wins; on a tie, the upper one.
				d, best := abs(c.row-cur.row), abs(target.row-cur.row)
				if !found || d < best || (d == best && c.row < target.row) {
					target, found = c, true
				}
			}
			if found {
				break
			}
		}
	}

	for i, id := range m.tabMatchIDs(m.tab) {
		if id == target.id {
			m.cursors[m.tab] = i
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
