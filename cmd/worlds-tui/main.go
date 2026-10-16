// Command worlds-tui follows the League of Legends World Championship in
// the terminal: live game stats and the Play-In, Swiss and Knockout stages.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
	"github.com/BigBoyChnusti/worlds-tui/internal/ui"
)

// Set by the release build through -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var cfg ui.Config
	flag.StringVar(&cfg.TournamentSlug, "tournament", "", "tournament slug, e.g. worlds_2025 (default: the current one)")
	flag.StringVar(&cfg.GameID, "game", "", "open this game id on start, e.g. to replay a finished game")
	flag.DurationVar(&cfg.Refresh, "refresh", 10*time.Second, "live game poll interval")
	flag.DurationVar(&cfg.DataRefresh, "data-refresh", time.Minute, "standings and schedule refresh interval")
	snapshot := flag.Bool("snapshot", false, "print every view once and exit instead of starting the TUI")
	width := flag.Int("width", 120, "snapshot width")
	height := flag.Int("height", 50, "snapshot height")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("worlds-tui %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	cfg.LeagueID = esports.WorldsLeagueID
	cfg.LeagueSlug = "worlds"

	if *snapshot {
		fmt.Println(ui.Snapshot(cfg, *width, *height))
		return
	}

	p := tea.NewProgram(ui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "worlds-tui:", err)
		os.Exit(1)
	}
}
