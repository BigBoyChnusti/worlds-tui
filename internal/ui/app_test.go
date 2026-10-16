package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

func readJSON(t *testing.T, file string, v any) {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// fixtureModel is a model fed with the saved Worlds 2025 standings and
// schedule, sized like a typical terminal.
func fixtureModel(t *testing.T) Model {
	t.Helper()
	var st struct {
		Data struct {
			Standings []struct {
				Stages []esports.Stage `json:"stages"`
			} `json:"standings"`
		} `json:"data"`
	}
	readJSON(t, "../bracket/testdata/standings_2025.json", &st)
	var events []esports.Event
	for _, f := range []string{"schedule_older.json", "schedule_page.json"} {
		var sc struct {
			Data struct {
				Schedule struct {
					Events []esports.Event `json:"events"`
				} `json:"schedule"`
			} `json:"data"`
		}
		readJSON(t, "../esports/testdata/"+f, &sc)
		// The pages span several years; keep Worlds 2025 like loadData does.
		for _, e := range sc.Data.Schedule.Events {
			if e.StartTime.Year() == 2025 && e.StartTime.Month() >= time.October {
				events = append(events, e)
			}
		}
	}

	m := New(Config{LeagueID: esports.WorldsLeagueID, LeagueSlug: "worlds"})
	m.tournament = esports.Tournament{ID: "113475452383887518", Slug: "worlds_2025"}
	var tm tea.Model = m
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	tm, _ = tm.Update(dataMsg{stages: st.Data.Standings[0].Stages, events: events})
	return tm.(Model)
}

func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var tm tea.Model = m
	var cmd tea.Cmd
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		tm, cmd = tm.Update(msg)
	}
	return tm.(Model), cmd
}

func plain(m Model) string { return ansi.Strip(m.View()) }

func TestTabsRenderStages(t *testing.T) {
	m := fixtureModel(t)
	cases := map[string][]string{
		"3": {"Round 1", "Round 5", "3–0", "AL  KT", "eliminated"},
		"4": {"Quarterfinals", "Semifinals", "Final", "T1 3–2 KT"},
		"5": {"VKS 0–1 TSW", "Quarterfinals"},
	}
	for key, wants := range cases {
		m, _ = press(t, m, key)
		v := plain(m)
		for _, w := range wants {
			if !strings.Contains(v, w) {
				t.Errorf("tab %s: missing %q in\n%s", key, w, v)
			}
		}
		if n := strings.Count(v, "\n") + 1; n != 60 {
			t.Errorf("tab %s: view has %d lines, want 60", key, n)
		}
	}
}

func TestOpenMatchShowsGame(t *testing.T) {
	m := fixtureModel(t)

	// Swiss round 1, fifth match: T1 vs FLY.
	m, _ = press(t, m, "3", "j", "j", "j", "j")
	if id := m.selectedID(); id != "113475798006664578" {
		t.Fatalf("selected %s, want the T1-FLY match", id)
	}
	m, cmd := press(t, m, "enter")
	if m.tab != tabLive || m.matchID != "113475798006664578" || !m.pinned || cmd == nil {
		t.Fatalf("enter did not open match: tab=%d match=%s pinned=%v", m.tab, m.matchID, m.pinned)
	}

	// The gateway lists T1 as blue here, but the feed has FlyQuest on blue.
	details := &esports.EventDetails{ID: m.matchID, Match: esports.Match{
		State:    esports.StateCompleted,
		Strategy: esports.Strategy{Count: 1},
		Teams: []esports.Team{
			{ID: "98767991853197861", Name: "T1", Code: "T1", Result: &esports.Result{GameWins: 1}},
			{ID: "98926509892121852", Name: "FlyQuest", Code: "FLY", Result: &esports.Result{GameWins: 0}},
		},
		Games: []esports.Game{{ID: "113475798006664579", Number: 1, State: esports.StateCompleted,
			Teams: []esports.GameTeam{{ID: "98767991853197861", Side: "blue"}, {ID: "98926509892121852", Side: "red"}}}},
	}}
	var tm tea.Model = m
	tm, cmd = tm.Update(detailsMsg{matchID: m.matchID, details: details})
	if cmd == nil {
		t.Fatal("details did not start loading the game")
	}

	var start, end livestats.Window
	var det livestats.Details
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	readJSON(t, "../livestats/testdata/window_end.json", &end)
	readJSON(t, "../livestats/testdata/details_end.json", &det)
	tm, _ = tm.Update(gameStartMsg{gameID: "113475798006664579", window: &start})
	tm, _ = tm.Update(gamePollMsg{gameID: "113475798006664579", window: &end, details: &det})
	m = tm.(Model)

	v := plain(m)
	for _, w := range []string{"T1  1 – 0  FlyQuest", "FINAL", "34:3", "FLY BLUE", "T1 RED", "Bwipo", "Faker", "Gold lead T1"} {
		if !strings.Contains(v, w) {
			t.Errorf("live view missing %q in\n%s", w, v)
		}
	}
	if strings.Contains(v, "FLY Bwipo") {
		t.Error("team prefix not stripped from player names")
	}

	// Following live again unpins the match.
	m, _ = press(t, m, "f")
	if m.pinned {
		t.Error("f did not unpin")
	}
}
