package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Snapshot loads the data once and renders the menu and every tab, for
// non-interactive use and debugging. It drives the model with the same
// messages the TUI uses, so the output matches what the interface shows.
func Snapshot(cfg Config, width, height int) string {
	m := New(cfg)
	m.width, m.height = width, height

	queue := []tea.Cmd{m.loadTournament(), m.loadLive()}
	if cfg.GameID != "" {
		queue = append(queue, m.startGame(cfg.GameID))
	}
	var model tea.Model = m
	// The cap only guards against a command loop. Loading the game, then
	// Data Dragon, then the champion icons takes more steps than the
	// tournament data alone.
	for steps := 0; len(queue) > 0 && steps < 100; steps++ {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		var next tea.Cmd
		model, next = model.Update(msg)
		queue = append(queue, next)
	}

	m = model.(Model)
	m.inMenu = true
	out := []string{m.View()}
	m.inMenu = false
	for t := 0; t < tabCount; t++ {
		m.tab = t
		out = append(out, m.View())
	}
	return strings.Join(out, "\n\n")
}
