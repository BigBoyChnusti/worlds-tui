package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

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
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		tm, cmd = tm.Update(msg)
	}
	return tm.(Model), cmd
}

func plain(m Model) string { return ansi.Strip(m.View()) }

func TestMenu(t *testing.T) {
	m := fixtureModel(t)
	if !m.inMenu {
		t.Fatal("app should start on the menu")
	}
	v := plain(m)
	for _, w := range []string{"██╗    ██╗", trophyGem, "Live", "Play-In", "Swiss Stage", "Knockouts", "Schedule", "Quit", "complete"} {
		if !strings.Contains(v, w) {
			t.Errorf("menu missing %q in\n%s", w, v)
		}
	}

	m, _ = press(t, m, "down", "down", "enter")
	if m.inMenu || m.tab != tabSwiss {
		t.Fatalf("enter on Swiss Stage: inMenu=%v tab=%d", m.inMenu, m.tab)
	}
	m, _ = press(t, m, "esc")
	if !m.inMenu || m.menuCursor != tabSwiss {
		t.Fatalf("esc did not return to the menu on the current view: inMenu=%v cursor=%d", m.inMenu, m.menuCursor)
	}
	m, _ = press(t, m, "4")
	if m.inMenu || m.tab != tabKnockout {
		t.Fatalf("4 from the menu: inMenu=%v tab=%d", m.inMenu, m.tab)
	}

	m, _ = press(t, m, "esc", "down", "down") // Knockouts -> Schedule -> Quit
	if menuItems[m.menuCursor].tab != -1 {
		t.Fatalf("cursor on %q, want Quit", menuItems[m.menuCursor].label)
	}
	_, cmd := press(t, m, "enter")
	if cmd == nil {
		t.Fatal("Quit did not return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("Quit did not quit")
	}

	// A narrow, short terminal drops the trophy and the block logo.
	m.width, m.height = 70, 22
	v = plain(m)
	if strings.Contains(v, trophyGem) || !strings.Contains(v, "W O R L D S") {
		t.Errorf("small menu layout wrong:\n%s", v)
	}
}

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

// liveSeries is a Bo5 with game 1 done and game 2 reported as running.
func liveSeries(matchID string) *esports.EventDetails {
	return &esports.EventDetails{ID: matchID, Match: esports.Match{
		State:    esports.StateInProgress,
		Strategy: esports.Strategy{Count: 5},
		Teams: []esports.Team{
			{ID: "98926509892121852", Name: "FlyQuest", Code: "FLY", Result: &esports.Result{GameWins: 0}},
			{ID: "98767991853197861", Name: "T1", Code: "T1", Result: &esports.Result{GameWins: 1}},
		},
		Games: []esports.Game{
			{ID: "g1", Number: 1, State: esports.StateCompleted},
			{ID: "113475798006664579", Number: 2, State: esports.StateInProgress},
			{ID: "g3", Number: 3, State: esports.StateUnstarted},
		},
	}}
}

// Game 2 is live in the gateway but the feed has no frames yet (draft,
// break, or a lagging feed). The view explains the wait and switches to stats once frames
// arrive.
func TestLiveGameWaitsForFeed(t *testing.T) {
	m := fixtureModel(t)
	m.inMenu = false
	var tm tea.Model = m
	tm, _ = tm.Update(liveMsg{events: []esports.Event{{
		League: esports.League{Slug: "worlds"},
		Match:  esports.Match{ID: "live-match"},
	}}})
	m = tm.(Model)
	if m.matchID != "live-match" || m.pinned {
		t.Fatalf("did not follow the live match: %q pinned=%v", m.matchID, m.pinned)
	}

	tm, cmd := tm.Update(detailsMsg{matchID: "live-match", details: liveSeries("live-match")})
	m = tm.(Model)
	if gm, _ := m.currentGame(); gm.Number != 2 {
		t.Fatalf("selected game %d, want the running game 2", gm.Number)
	}
	if cmd == nil {
		t.Fatal("no request for the running game")
	}

	tm, _ = tm.Update(gameStartMsg{gameID: "113475798006664579", err: livestats.ErrNoData})
	m = tm.(Model)
	m.tab = tabLive
	v := plain(m)
	for _, w := range []string{"waiting for the game to load", "draft", "next check in", "previous game"} {
		if !strings.Contains(v, w) {
			t.Errorf("waiting view missing %q in\n%s", w, v)
		}
	}

	// The next start attempt waits for the retry interval.
	if c := m.ensureGame(); c != nil {
		t.Error("retried before the retry interval")
	}
	m.startAttempt["113475798006664579"] = m.startAttempt["113475798006664579"].Add(-startRetryEvery)
	if c := m.ensureGame(); c == nil {
		t.Fatal("no retry after the interval")
	}

	var start, end livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	readJSON(t, "../livestats/testdata/window_end.json", &end)
	tm = m
	tm, _ = tm.Update(gameStartMsg{gameID: "113475798006664579", window: &start})
	tm, _ = tm.Update(gamePollMsg{gameID: "113475798006664579", window: &end})
	v = plain(tm.(Model))
	if strings.Contains(v, "waiting") || !strings.Contains(v, "Bwipo") {
		t.Errorf("stats not shown once the feed has frames:\n%s", v)
	}
}

// The selected game's label must not wrap already styled text: lipgloss
// underlines rune by rune and would print the inner escape codes.
func TestGameStripHasNoBrokenEscapes(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := fixtureModel(t)
	m.inMenu, m.tab = false, tabLive
	m.matchID = "live-match"
	m.details = liveSeries("live-match")
	m.gameIdx = 1

	raw := m.View()
	if !strings.Contains(raw, "\x1b[") {
		t.Fatal("test needs colour output")
	}
	v := ansi.Strip(raw)
	for _, bad := range []string{"[0m", ";2;", "[1;"} {
		if strings.Contains(v, bad) {
			t.Errorf("escape code fragment %q printed as text:\n%s", bad, v)
		}
	}
	if !strings.Contains(v, "G2 ●") {
		t.Errorf("game strip missing live marker:\n%s", v)
	}
}

// A running game replays its newest window in real time: the clock moves
// every second between feed updates and holds at the newest frame.
func TestLiveClockTicks(t *testing.T) {
	m := fixtureModel(t)
	m.inMenu, m.tab = false, tabLive
	m.matchID, m.details, m.gameIdx = "live-match", liveSeries("live-match"), 1

	var start, mid livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	readJSON(t, "../livestats/testdata/window_mid.json", &mid) // 10s window, in game
	const id = "113475798006664579"
	var tm tea.Model = m
	tm, _ = tm.Update(gameStartMsg{gameID: id, window: &start})
	tm, _ = tm.Update(gamePollMsg{gameID: id, window: &mid})
	m = tm.(Model)

	g := m.games[id]
	if _, ok := m.play[id]; !ok {
		t.Fatal("no replay for a running game")
	}
	clockLine := func(m Model) string {
		for _, l := range strings.Split(plain(m), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), "Game 2") {
				return l
			}
		}
		t.Fatalf("no game line in\n%s", plain(m))
		return ""
	}

	// Starts one window behind the newest frame...
	first := m.playhead(id, g)
	if d := g.Latest.Timestamp.Sub(first); d < 9*time.Second || d > 11*time.Second {
		t.Errorf("playhead %v behind the newest frame, want about 10s", d)
	}
	// ...and advances one second per second.
	var lines []string
	base := m.now
	for i := 0; i < 4; i++ {
		m.now = base.Add(time.Duration(i) * time.Second)
		lines = append(lines, clockLine(m))
		if got := m.playhead(id, g).Sub(first); got != time.Duration(i)*time.Second {
			t.Errorf("after %ds playhead moved %v", i, got)
		}
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == lines[i-1] {
			t.Errorf("clock did not change between seconds %d and %d: %q", i-1, i, lines[i])
		}
	}
	if !strings.Contains(lines[0], "LIVE") {
		t.Errorf("running game not marked live: %q", lines[0])
	}

	// With no new data it stops at the newest frame instead of running on.
	m.now = base.Add(time.Minute)
	m.advancePlay(id, g, false)
	if got := m.playhead(id, g); !got.Equal(g.Latest.Timestamp) {
		t.Errorf("playhead %v past the newest frame %v", got, g.Latest.Timestamp)
	}
	// And picks up from there, not with a jump, when the next window lands.
	m.now = m.now.Add(time.Second)
	if got := m.playhead(id, g); !got.Equal(g.Latest.Timestamp) {
		t.Errorf("held playhead moved without data: %v", got)
	}
}

func TestArrowsDoNotSwitchTabs(t *testing.T) {
	m := fixtureModel(t)
	m, _ = press(t, m, "3", "right", "right", "l", "left", "h")
	if m.tab != tabSwiss {
		t.Errorf("arrows changed the tab to %d", m.tab)
	}
	m, _ = press(t, m, "tab")
	if m.tab != tabKnockout {
		t.Errorf("tab went to %d, want Knockouts", m.tab)
	}
	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if tm.(Model).tab != tabSwiss {
		t.Errorf("shift+tab went to %d, want Swiss", tm.(Model).tab)
	}
}

// clickZone renders the view, waits for the zone to be registered and
// clicks its top-left cell.
func clickZone(t *testing.T, m Model, id string) Model {
	t.Helper()
	m.View()
	deadline := time.Now().Add(2 * time.Second)
	z := m.zones.Get(id)
	for z.IsZero() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		z = m.zones.Get(id)
	}
	if z.IsZero() {
		t.Fatalf("zone %q never rendered", id)
	}
	var tm tea.Model = m
	tm, _ = tm.Update(tea.MouseMsg{X: z.StartX, Y: z.StartY, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	return tm.(Model)
}

func TestMouse(t *testing.T) {
	m := fixtureModel(t)

	m = clickZone(t, m, menuZone(2))
	if m.inMenu || m.tab != tabSwiss {
		t.Fatalf("menu click: inMenu=%v tab=%d", m.inMenu, m.tab)
	}
	m = clickZone(t, m, tabZone(tabSchedule))
	if m.tab != tabSchedule {
		t.Fatalf("tab click went to %d", m.tab)
	}

	var tm tea.Model = m
	tm, _ = tm.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	tm, _ = tm.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if c := tm.(Model).cursors[tabSchedule]; c != 2 {
		t.Errorf("wheel moved the cursor to %d, want 2", c)
	}

	// Clicking a match opens it on the Live tab.
	m = clickZone(t, tm.(Model), lineZone("113475798006664578"))
	if m.tab != tabLive || m.matchID != "113475798006664578" {
		t.Fatalf("match click: tab=%d match=%s", m.tab, m.matchID)
	}

	// Clicking a game of the series switches to it.
	m.details = liveSeries(m.matchID)
	m.details.ID = m.matchID
	m.gameIdx = 1
	m = clickZone(t, m, gameZone(0))
	if m.gameIdx != 0 || !m.gamePinned {
		t.Errorf("game click: idx=%d pinned=%v", m.gameIdx, m.gamePinned)
	}

	// Knockout boxes are clickable too.
	m, _ = press(t, m, "4")
	m = clickZone(t, m, boxZone(m.bracket.Knockout.Final.ID))
	if m.matchID != m.bracket.Knockout.Final.ID {
		t.Errorf("box click opened %s", m.matchID)
	}
}
