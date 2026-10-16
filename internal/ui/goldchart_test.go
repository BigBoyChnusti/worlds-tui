package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

func pts(diffs ...int) []livestats.GoldPoint {
	var out []livestats.GoldPoint
	for i, d := range diffs {
		out = append(out, livestats.GoldPoint{Clock: time.Duration(i) * time.Minute, Diff: d})
	}
	return out
}

func chartLines(t *testing.T, s string, width, rows int) []string {
	t.Helper()
	lines := strings.Split(s, "\n")
	if len(lines) != rows+2 {
		t.Fatalf("chart has %d lines, want %d:\n%s", len(lines), rows+2, ansi.Strip(s))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > width {
			t.Errorf("line %d is %d cells wide, limit %d: %q", i, w, width, ansi.Strip(l))
		}
		lines[i] = ansi.Strip(l)
	}
	return lines
}

func TestGoldChartBlueAboveRedBelow(t *testing.T) {
	// Blue leads by 4k at minute 10, red by 4k at minute 20.
	hist := pts(0, 400, 800, 1200, 1600, 2000, 2400, 2800, 3200, 3600, 4000,
		3200, 2400, 1600, 800, 0, -800, -1600, -2400, -3200, -4000)
	const width, rows = 90, 13
	lines := chartLines(t, goldChart(hist, 20*time.Minute, width, rows, "AAA", "BBB"), width, rows)

	if !strings.HasPrefix(lines[0], "  +4k ┤") || !strings.HasPrefix(lines[12], "  -4k ┤") ||
		!strings.HasPrefix(lines[6], "    0 ┼──") || !strings.HasPrefix(lines[3], "  +2k ┤") {
		t.Errorf("y axis labels wrong:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[1], "AAA ▲ │") || !strings.HasPrefix(lines[11], "BBB ▼ │") {
		t.Errorf("team labels missing:\n%s", strings.Join(lines, "\n"))
	}
	col := func(r, c int) rune {
		rr := []rune(lines[r])
		if c >= len(rr) {
			return ' '
		}
		return rr[c]
	}
	plot := len([]rune("  +4k ┤")) // first plot column
	cols := width - plot
	mid, last := plot+cols/2, plot+cols-1
	// At minute 10 blue fills the whole upper half, red is empty.
	if col(0, mid) != '█' || col(5, mid) != '█' || col(7, mid) != ' ' {
		t.Errorf("blue peak not drawn at column %d:\n%s", mid, strings.Join(lines, "\n"))
	}
	// At the end red fills the lower half and blue nothing.
	if col(12, last) != '█' || col(7, last) != '█' || col(5, last) != ' ' {
		t.Errorf("red peak not drawn at column %d:\n%s", last, strings.Join(lines, "\n"))
	}
	// Minute ticks along the bottom, 5 minutes apart at this width.
	if !strings.Contains(lines[14], "0") || !strings.Contains(lines[14], "5") ||
		!strings.Contains(lines[14], "10") || !strings.Contains(lines[14], "20") {
		t.Errorf("x labels wrong: %q", lines[14])
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[13]), "└┴") {
		t.Errorf("x axis wrong: %q", lines[13])
	}
}

func TestGoldChartHalfBlocks(t *testing.T) {
	// A constant lead of a quarter of the 4k scale: 3 of 12 half cells on
	// 13 rows, so a full row and a lower half block above the baseline.
	hist := pts(1000, 1000, 1000, 4000)
	lines := chartLines(t, goldChart(hist, 3*time.Minute, 60, 13, "A", "B"), 60, 13)
	c := len([]rune(" +4k ┤")) + 1
	got := string([]rune(lines[5])[c]) + string([]rune(lines[4])[c]) + string([]rune(lines[3])[c])
	if got != "█▄ " {
		t.Errorf("rows above the baseline = %q, want %q\n%s", got, "█▄ ", strings.Join(lines, "\n"))
	}
}

func TestGoldChartEdgeCases(t *testing.T) {
	if s := goldChart(nil, time.Minute, 80, 9, "A", "B"); s != "" {
		t.Errorf("empty history drew %q", s)
	}
	if s := goldChart(pts(100), 0, 80, 9, "A", "B"); s != "" {
		t.Errorf("zero clock drew %q", s)
	}
	if s := goldChart(pts(0, 0), time.Minute, 4, 9, "A", "B"); s != "" {
		t.Errorf("no room drew %q", s)
	}
	// All even: only the baseline, no fill.
	lines := chartLines(t, goldChart(pts(0, 0, 0), 2*time.Minute, 40, 9, "A", "B"), 40, 9)
	for i, l := range lines[:9] {
		if i != 4 && strings.ContainsAny(l, "█▀▄") {
			t.Errorf("even game filled row %d: %q", i, l)
		}
	}
	// A single sample late in the game, an even row count and a tiny
	// width all still draw within bounds.
	chartLines(t, goldChart([]livestats.GoldPoint{{Clock: 90 * time.Second, Diff: -25000}}, 2*time.Minute, 20, 6, "LONGCODE", "B"), 20, 5)
	chartLines(t, goldChart(pts(0, 500), 30*time.Second, 30, 1, "A", "B"), 30, 3)
}

func TestChartScale(t *testing.T) {
	for in, want := range map[int]int{0: 2000, 1500: 2000, 2001: 4000, 11700: 12000, 20000: 20000, 20001: 30000} {
		if got := chartScale(in); got != want {
			t.Errorf("chartScale(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestGoldSeriesStopsAtPlayhead(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 17, 0, 0, 0, time.UTC)
	f := func(sec, blue, red int) livestats.WindowFrame {
		return livestats.WindowFrame{Timestamp: t0.Add(time.Duration(sec) * time.Second), GameState: livestats.GameInGame,
			BlueTeam: livestats.TeamFrame{TotalGold: blue}, RedTeam: livestats.TeamFrame{TotalGold: red}}
	}
	g := livestats.NewGame("x")
	g.SetStart(&livestats.Window{Frames: []livestats.WindowFrame{f(0, 2500, 2500)}})
	var frames []livestats.WindowFrame
	for m := 1; m <= 20; m++ {
		frames = append(frames, f(m*60, 2500+m*500, 2500))
	}
	g.ApplyWindow(&livestats.Window{Frames: frames})

	at := t0.Add(10*time.Minute + 30*time.Second)
	series, end := goldSeries(g, at)
	if end != 10*time.Minute+30*time.Second {
		t.Fatalf("end = %v", end)
	}
	for _, p := range series {
		if p.Clock > end {
			t.Errorf("point after the playhead: %+v", p)
		}
	}
	lastP := series[len(series)-1]
	if lastP.Clock != end || lastP.Diff != 5000 {
		t.Errorf("series ends with %+v, want the minute 10 value at the playhead", lastP)
	}
	if len(series) != 12 { // minutes 0 to 10 plus the playhead
		t.Errorf("got %d points, want 12: %+v", len(series), series)
	}
}
