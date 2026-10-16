package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

func TestGlyphRowsLineUp(t *testing.T) {
	for r, g := range bigGlyphs {
		for i, row := range g {
			if lipgloss.Width(row) != lipgloss.Width(g[0]) {
				t.Errorf("big %q row %d is %d wide, row 0 is %d", r, i, lipgloss.Width(row), lipgloss.Width(g[0]))
			}
		}
	}
	for r, g := range midGlyphs {
		for i, row := range g {
			if len(row) != len(g[0]) || strings.Trim(row, ".#") != "" {
				t.Errorf("mid %q row %d is %q", r, i, row)
			}
		}
	}
}

func TestBigAndMidText(t *testing.T) {
	big := bigText("12:05", stAccent)
	if len(big) != 6 {
		t.Fatalf("bigText has %d rows", len(big))
	}
	w := lipgloss.Width(big[0])
	for i, r := range big {
		if lipgloss.Width(r) != w {
			t.Errorf("bigText row %d is %d wide, want %d", i, lipgloss.Width(r), w)
		}
	}
	mid := midText("60.3k", stBold)
	want := []string{
		"█▀▀ █▀█   ▀▀█ █ ▄",
		"█▀█ █ █   ▀▀█ █▀▄",
		"▀▀▀ ▀▀▀ ▀ ▀▀▀ ▀ ▀",
	}
	for i := range want {
		if got := ansi.Strip(mid[i]); got != want[i] {
			t.Errorf("midText row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestScoreboard(t *testing.T) {
	bt := livestats.TeamFrame{TotalKills: 12, TotalGold: 60300, Towers: 1, Dragons: []string{"mountain"}}
	rt := livestats.TeamFrame{TotalKills: 29, TotalGold: 72000, Towers: 8, Inhibitors: 1, Barons: 2,
		Dragons: []string{"hextech", "chemtech", "chemtech"}}
	clock := 34*time.Minute + 31*time.Second

	if s := scoreboard("FLY", "T1", bt, rt, clock, 80); s != "" {
		t.Errorf("scoreboard drawn in 80 columns:\n%s", ansi.Strip(s))
	}
	s := scoreboard("FLY", "T1", bt, rt, clock, 160)
	lines := strings.Split(s, "\n")
	if len(lines) != 11 {
		t.Errorf("scoreboard has %d lines, want 11", len(lines))
	}
	for i, l := range lines {
		if lipgloss.Width(l) > 160 {
			t.Errorf("line %d is %d wide", i, lipgloss.Width(l))
		}
	}
	p := ansi.Strip(s)
	for _, w := range []string{"FLY", "T1", "1   TOWERS    8", "0   BARONS    2", "◆M   DRAGONS   ◆H ◆T ◆T", "GOLD +11.7k"} {
		if !strings.Contains(p, w) {
			t.Errorf("scoreboard missing %q in\n%s", w, p)
		}
	}
}
