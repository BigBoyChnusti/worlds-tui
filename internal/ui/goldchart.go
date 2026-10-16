package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

// goldStep is the bucket size used to read the gold history. The backfill
// fetches one window per minute and live polling adds frames seconds
// apart, so a small step keeps every distinct sample; the chart
// interpolates between them.
const goldStep = time.Second

// goldSeries is the gold difference up to the game clock at the playhead
// at, ending with the value current at the playhead. full.At shares the
// history with full, so later samples are cut off here.
func goldSeries(full *livestats.Game, at time.Time) ([]livestats.GoldPoint, time.Duration) {
	end := full.ClockAt(at)
	var out []livestats.GoldPoint
	for _, p := range full.History(goldStep) {
		if p.Clock > end {
			break
		}
		out = append(out, p)
	}
	// Close the series at the playhead. At falls back to the oldest kept
	// frame when the playhead is before it, so only a frame at or before
	// the playhead counts; otherwise the last sample holds.
	if v := full.At(at); v.Latest != nil && !v.Latest.Timestamp.After(at) {
		out = append(out, livestats.GoldPoint{Clock: end, Diff: v.GoldDiff()})
	} else if n := len(out); n > 0 && out[n-1].Clock < end {
		out = append(out, livestats.GoldPoint{Clock: end, Diff: out[n-1].Diff})
	}
	return out, end
}

// chartScale rounds the largest gold difference up to a value whose half
// is a round number too.
func chartScale(maxAbs int) int {
	switch {
	case maxAbs <= 2000:
		return 2000
	case maxAbs <= 20000:
		return (maxAbs + 1999) / 2000 * 2000
	}
	return (maxAbs + 9999) / 10000 * 10000
}

func axisGold(v int) string {
	switch {
	case v == 0:
		return "0"
	case v%1000 == 0 && v > 0:
		return fmt.Sprintf("+%dk", v/1000)
	case v%1000 == 0:
		return fmt.Sprintf("-%dk", -v/1000)
	case v > 0:
		return "+" + fmtGold(v)
	}
	return "-" + fmtGold(-v)
}

// diffAt interpolates the gold difference at clock t between the samples.
func diffAt(pts []livestats.GoldPoint, t time.Duration) float64 {
	i := sort.Search(len(pts), func(i int) bool { return pts[i].Clock >= t })
	switch {
	case i == len(pts):
		return float64(pts[len(pts)-1].Diff)
	case pts[i].Clock == t || i == 0:
		return float64(pts[i].Diff)
	}
	a, b := pts[i-1], pts[i]
	f := float64(t-a.Clock) / float64(b.Clock-a.Clock)
	return float64(a.Diff) + f*float64(b.Diff-a.Diff)
}

// goldChart draws the blue minus red gold difference from clock 0 to end
// as an area chart: blue fills up from a zero baseline in the middle, red
// fills down. Half blocks give two steps per row. rows is made odd (the
// baseline sits in the middle row) and at least 3; the result is rows
// lines of chart plus an x axis and its minute labels, each at most width
// cells wide. It returns "" when there is nothing to draw.
func goldChart(pts []livestats.GoldPoint, end time.Duration, width, rows int, blueCode, redCode string) string {
	if len(pts) == 0 || end <= 0 {
		return ""
	}
	half := max(1, (rows-1)/2)
	rows = 2*half + 1

	maxAbs := 0
	for _, p := range pts {
		maxAbs = max(maxAbs, p.Diff, -p.Diff)
	}
	scale := chartScale(maxAbs)

	// Left gutter: scale labels on the top, middle and bottom rows, plus
	// the team codes next to the half they lead in.
	labels := map[int]string{0: axisGold(scale), half: "0", rows - 1: axisGold(-scale)}
	if half%2 == 0 && half >= 4 {
		labels[half/2] = axisGold(scale / 2)
		labels[half+half/2] = axisGold(-scale / 2)
	}
	gutter := map[int]string{}
	gutterW := 0
	for r, l := range labels {
		gutter[r] = stMuted.Render(l)
		gutterW = max(gutterW, lipgloss.Width(l))
	}
	if half >= 2 {
		gutter[1] = stBlue.Render(blueCode + " ▲")
		gutter[rows-2] = stRed.Render(redCode + " ▼")
		gutterW = max(gutterW, lipgloss.Width(blueCode)+2, lipgloss.Width(redCode)+2)
	}
	gutterW++ // space before the axis
	cols := width - gutterW - 1
	if cols < 2 {
		return ""
	}

	if pts[0].Clock > 0 {
		pts = append([]livestats.GoldPoint{{}}, pts...)
	}
	// Half-cell heights per column, positive for blue.
	h := make([]int, cols)
	for c := range h {
		t := time.Duration(float64(end) * float64(c) / float64(cols-1))
		v := diffAt(pts, t) / float64(scale) * float64(2*half)
		h[c] = max(-2*half, min(2*half, int(math.Round(v))))
	}

	fill := func(n, k int, part rune) rune {
		switch {
		case n >= 2*k+2:
			return '█'
		case n == 2*k+1:
			return part
		}
		return ' '
	}
	blueSt := lipgloss.NewStyle().Foreground(colBlue)
	redSt := lipgloss.NewStyle().Foreground(colRed)

	var b strings.Builder
	for r := 0; r < rows; r++ {
		axis := "│"
		if _, ok := labels[r]; ok {
			axis = "┤"
		}
		var line []rune
		var st lipgloss.Style
		switch {
		case r < half:
			k := half - 1 - r // 0 next to the baseline
			for _, n := range h {
				line = append(line, fill(n, k, '▄'))
			}
			st = blueSt
		case r == half:
			axis = "┼"
			line = []rune(strings.Repeat("─", cols))
			st = stMuted
		default:
			k := r - half - 1
			for _, n := range h {
				line = append(line, fill(-n, k, '▀'))
			}
			st = redSt
		}
		b.WriteString(padLeft(gutter[r]+" ", gutterW) + stMuted.Render(axis))
		if s := string(line); strings.TrimSpace(s) != "" {
			b.WriteString(st.Render(strings.TrimRight(s, " ")))
		}
		b.WriteString("\n")
	}

	// X axis with a tick every few minutes, at least 8 columns apart.
	mins := end.Minutes()
	step := 1
	for _, s := range []int{1, 5, 10, 15, 30, 60} {
		step = s
		if float64(cols)*float64(s)/math.Max(mins, 1) >= 8 {
			break
		}
	}
	axis := []rune(strings.Repeat("─", cols))
	tickLine := []rune(strings.Repeat(" ", cols))
	for m := 0; float64(m) <= mins; m += step {
		c := int(math.Round(float64(m) / mins * float64(cols-1)))
		axis[c] = '┴'
		lbl := []rune(fmt.Sprint(m))
		if c+len(lbl) > cols {
			c = cols - len(lbl) // the last label ends at the right edge
		}
		if c > 0 && (tickLine[c-1] != ' ' || tickLine[c] != ' ') {
			continue
		}
		copy(tickLine[c:], lbl)
	}
	b.WriteString(strings.Repeat(" ", gutterW) + stMuted.Render("└"+string(axis)) + "\n")
	ticks := strings.TrimRight(string(tickLine), " ")
	if len([]rune(ticks))+4 <= cols {
		ticks += " min"
	}
	b.WriteString(strings.Repeat(" ", gutterW+1) + stMuted.Render(ticks))
	return b.String()
}

// chartLegend says which team leads above and below the baseline, in at
// most width cells.
func chartLegend(blueCode, redCode string, width int) string {
	long := stMuted.Render("   gold difference: ") + stBlue.Render(blueCode+" ▲") +
		stMuted.Render(" ahead above, ") + stRed.Render(redCode+" ▼") + stMuted.Render(" ahead below")
	if lipgloss.Width(long) <= width {
		return long
	}
	short := "   " + stBlue.Render(blueCode+" ▲") + " " + stRed.Render(redCode+" ▼")
	if lipgloss.Width(short) <= width {
		return short
	}
	return ""
}

// chartRows sizes the gold chart to what is left of the Live tab's body
// after the lines above it and the player tables below it: 11 to 13 rows
// with the big scoreboard, down to 5 on small terminals where the tab
// scrolls anyway.
func (m Model) chartRows(above, below string) int {
	body := m.height - lipgloss.Height(m.renderHeader()) - lipgloss.Height(m.renderFooter())
	used := lipgloss.Height(m.renderSeriesHeader()) + 1 + // series header and blank line
		lipgloss.Height(above) - 1 + // above ends with a newline
		3 + // x axis, its labels and the blank line after the chart
		lipgloss.Height(below)
	lo := 5
	if m.height >= scoreboardMinHeight {
		lo = 11
	}
	return max(lo, min(13, body-used))
}
