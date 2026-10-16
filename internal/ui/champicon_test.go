package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/BigBoyChnusti/worlds-tui/internal/ddragon"
	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

// quadrants is an 8x8 image: red, green, blue and white quarters, with one
// odd pixel in the red quarter to check the box averaging.
func quadrants() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			c := color.RGBA{255, 255, 255, 255}
			switch {
			case x < 4 && y < 4:
				c = color.RGBA{255, 0, 0, 255}
			case y < 4:
				c = color.RGBA{0, 255, 0, 255}
			case x < 4:
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	img.Set(0, 0, color.RGBA{0, 0, 0, 255})
	return img
}

func TestDownscaleBoxFilter(t *testing.T) {
	px := downscale(quadrants(), 4, 4)
	if len(px) != 4 || len(px[0]) != 4 {
		t.Fatalf("size %dx%d, want 4x4", len(px[0]), len(px))
	}
	want := map[[2]int]rgb{
		{0, 0}: {191, 0, 0}, // three red pixels and one black
		{1, 1}: {255, 0, 0},
		{3, 0}: {0, 255, 0},
		{0, 3}: {0, 0, 255},
		{3, 3}: {255, 255, 255},
	}
	for pos, c := range want {
		if got := px[pos[1]][pos[0]]; got != c {
			t.Errorf("pixel %v = %v, want %v", pos, got, c)
		}
	}

	// Upscaling a tiny image repeats its pixels instead of reading nothing.
	tiny := image.NewRGBA(image.Rect(0, 0, 2, 2))
	tiny.Set(1, 1, color.RGBA{10, 20, 30, 255})
	if got := downscale(tiny, 4, 4)[3][3]; got != (rgb{10, 20, 30}) {
		t.Errorf("upscaled pixel = %v", got)
	}

	// Bounds that do not start at the origin.
	sub := quadrants().SubImage(image.Rect(4, 4, 8, 8))
	if got := downscale(sub, 1, 1)[0][0]; got != (rgb{255, 255, 255}) {
		t.Errorf("sub image pixel = %v", got)
	}
}

func TestIconLineHalfBlocks(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	ic := newChampIcon(quadrants())
	top, bottom := iconLine(ic, 0), iconLine(ic, 1)
	for _, l := range []string{top, bottom} {
		if w := lipgloss.Width(l); w != iconCols {
			t.Errorf("line width %d, want %d: %q", w, iconCols, l)
		}
		if got := ansi.Strip(l); got != strings.Repeat("▀", iconCols) {
			t.Errorf("cells %q, want half blocks", got)
		}
	}
	// Line 0 holds pixel rows 0 (foreground) and 1 (background); its last
	// cell is green over green, its first dark red over red.
	for _, w := range []string{"38;2;191;0;0", "48;2;255;0;0", "38;2;0;255;0", "48;2;0;255;0"} {
		if !strings.Contains(top, w) {
			t.Errorf("top line missing %q: %q", w, top)
		}
	}
	for _, w := range []string{"38;2;0;0;255", "48;2;255;255;255"} {
		if !strings.Contains(bottom, w) {
			t.Errorf("bottom line missing %q: %q", w, bottom)
		}
	}

	if got := iconLine(nil, 0); got != strings.Repeat(" ", iconCols) {
		t.Errorf("missing icon = %q, want blanks", got)
	}
	lipgloss.SetColorProfile(termenv.Ascii)
	if got := iconLine(ic, 0); got != strings.Repeat(" ", iconCols) {
		t.Errorf("icon without colour = %q, want blanks", got)
	}
}

// Every player takes two lines whether or not item details and icons are
// there, and a row still fits 80 cells.
func TestPlayersKeepIconColumn(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	var start, end livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	readJSON(t, "../livestats/testdata/window_end.json", &end)
	g := livestats.NewGame("g")
	g.SetStart(&start)
	g.ApplyWindow(&end)

	m := New(Config{})
	players := g.Players("blue")
	m.icons[players[0].Meta.ChampionID] = newChampIcon(quadrants())

	out := strings.TrimSuffix(m.renderPlayers(g, "blue", "FLY"), "\n")
	lines := strings.Split(out, "\n")
	if want := 2 + 2*len(players); len(lines) != want {
		t.Fatalf("%d lines, want %d:\n%s", len(lines), want, out)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line %d is %d cells wide: %q", i, w, ansi.Strip(l))
		}
	}
	if !strings.Contains(lines[2], "▀") || !strings.Contains(lines[3], "▀") {
		t.Errorf("first player has no icon:\n%s\n%s", lines[2], lines[3])
	}
	// A missing icon leaves blanks of the same width, so roles line up.
	for i := 2; i < len(lines); i += 2 {
		plainLine := ansi.Strip(lines[i])
		idx := strings.Index(plainLine, roleShort[players[(i-2)/2].Meta.Role])
		if col := lipgloss.Width(plainLine[:max(0, idx)]); idx < 0 || col != iconCols+1 {
			t.Errorf("line %d: role at column %d, want %d: %q", i, col, iconCols+1, plainLine)
		}
	}
}

func TestIconsMsg(t *testing.T) {
	m := New(Config{})
	if m.loadIcons() != nil {
		t.Error("icons requested before Data Dragon loaded")
	}

	var start livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	g := livestats.NewGame("g")
	g.SetStart(&start)
	m.games["g"] = g
	m.ddData = &ddragon.Data{Version: "15.20.1"}
	if m.loadIcons() == nil {
		t.Fatal("no icon load for a game with champions")
	}
	if len(m.iconsAsked) != 10 {
		t.Errorf("asked for %d icons, want 10", len(m.iconsAsked))
	}
	if m.loadIcons() != nil {
		t.Error("icons requested twice")
	}

	key := g.Meta.BlueTeamMetadata.ParticipantMetadata[0].ChampionID
	other := g.Meta.RedTeamMetadata.ParticipantMetadata[0].ChampionID
	var tm tea.Model = m
	tm, _ = tm.Update(iconsMsg{version: "old", icons: map[string]*champIcon{key: {}}})
	if tm.(Model).icons[key] != nil {
		t.Error("icon from another version kept")
	}
	tm, _ = tm.Update(iconsMsg{version: "15.20.1", icons: map[string]*champIcon{key: {}}, failed: []string{other}})
	m = tm.(Model)
	if m.icons[key] == nil {
		t.Error("icon not stored")
	}
	if m.iconsAsked[other] {
		t.Error("failed icon not cleared for a retry")
	}
}
