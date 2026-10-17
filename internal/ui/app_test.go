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
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
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
		"5": {"TSW 1–0 VKS", "Quarterfinals"},
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

	// Big terminals get the scoreboard, small ones the compact table; the
	// gold chart shows in both and no line is wider than the terminal.
	for _, sz := range []struct {
		w, h int
		want []string
	}{
		{200, 60, []string{"TOWERS", "DRAGONS", "╚════██╗", "FLY ▲", "T1 ▼", "+12k ┤", "0 ┼───"}},
		{120, 60, []string{"TOWERS", "╚════██╗", "0 ┼───"}},
		{80, 24, []string{"Kills     Gold  Towers", "0 ┼───", "FLY ▲"}},
	} {
		tm, _ := m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
		v := tm.(Model).View()
		for i, l := range strings.Split(v, "\n") {
			if w := lipgloss.Width(l); w > sz.w {
				t.Errorf("%dx%d: line %d is %d cells wide: %q", sz.w, sz.h, i, w, ansi.Strip(l))
			}
		}
		p := ansi.Strip(v)
		for _, w := range sz.want {
			if !strings.Contains(p, w) {
				t.Errorf("%dx%d: live view missing %q in\n%s", sz.w, sz.h, w, p)
			}
		}
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

// clickZone renders the view, waits for the zone's position to settle and
// clicks its top-left cell. bubblezone registers zones on a background
// goroutine, so right after a view change a zone id that also appeared in
// the previous view can still report its old position.
func clickZone(t *testing.T, m Model, id string) Model {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var z *zone.ZoneInfo
	for stable := 0; stable < 3; {
		if time.Now().After(deadline) {
			t.Fatalf("zone %q never settled", id)
		}
		m.View()
		time.Sleep(10 * time.Millisecond)
		next := m.zones.Get(id)
		if !next.IsZero() && z != nil && next.StartX == z.StartX && next.StartY == z.StartY {
			stable++
		} else {
			stable = 0
		}
		z = next
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

func TestWinnerFirst(t *testing.T) {
	slots := func(w0, w1 int, o0, o1 string) [2]bracket.Slot {
		return [2]bracket.Slot{{Code: "GEN", Wins: w0, Outcome: o0}, {Code: "T1", Wins: w1, Outcome: o1}}
	}
	cases := []struct {
		name      string
		m         bracket.Match
		top, line string
	}{
		{"second won", bracket.Match{State: esports.StateCompleted, Teams: slots(1, 3, "loss", "win")}, "T1", "T1 3–1 GEN"},
		{"first won", bracket.Match{State: esports.StateCompleted, Teams: slots(3, 2, "win", "loss")}, "GEN", "GEN 3–2 T1"},
		{"completed without outcomes", bracket.Match{State: esports.StateCompleted, Teams: slots(0, 2, "", "")}, "T1", "T1 2–0 GEN"},
		{"outcome before state flips", bracket.Match{State: esports.StateInProgress, Teams: slots(1, 3, "loss", "win")}, "T1", "T1 3–1 GEN ●"},
		{"in progress keeps order", bracket.Match{State: esports.StateInProgress, Teams: slots(0, 1, "", "")}, "GEN", "GEN 0–1 T1 ●"},
		{"unstarted keeps order", bracket.Match{State: esports.StateUnstarted, Teams: slots(0, 0, "", "")}, "GEN", "GEN vs T1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orig := c.m.Teams
			box := strings.Split(ansi.Strip(matchBox(&c.m, "", false)), "\n")
			if len(box) != 4 {
				t.Fatalf("box has %d lines, want 4:\n%s", len(box), strings.Join(box, "\n"))
			}
			if got := strings.Fields(strings.Trim(box[1], "│ "))[0]; got != c.top {
				t.Errorf("box top = %q, want %q", got, c.top)
			}
			if got := strings.Join(strings.Fields(ansi.Strip(matchLine(&c.m, false))), " "); got != c.line {
				t.Errorf("matchLine = %q, want %q", got, c.line)
			}
			if c.m.Teams != orig {
				t.Errorf("Teams reordered in data: %+v", c.m.Teams)
			}
		})
	}
}

func TestEventTitleWinnerFirst(t *testing.T) {
	ev := func(state string, r0, r1 *esports.Result) esports.Event {
		var e esports.Event
		e.State = state
		e.Match.Teams = []esports.Team{{Code: "GEN", Result: r0}, {Code: "T1", Result: r1}}
		return e
	}
	cases := []struct {
		e    esports.Event
		want string
	}{
		{ev(esports.StateCompleted, &esports.Result{Outcome: "loss", GameWins: 1}, &esports.Result{Outcome: "win", GameWins: 3}), "T1 vs GEN"},
		{ev(esports.StateCompleted, &esports.Result{Outcome: "win", GameWins: 3}, &esports.Result{Outcome: "loss", GameWins: 1}), "GEN vs T1"},
		{ev(esports.StateUnstarted, nil, nil), "GEN vs T1"},
		{ev(esports.StateInProgress, &esports.Result{GameWins: 0}, &esports.Result{GameWins: 2}), "GEN vs T1"},
	}
	for _, c := range cases {
		if got := eventTitle(c.e); got != c.want {
			t.Errorf("eventTitle(%s) = %q, want %q", c.e.State, got, c.want)
		}
	}
	var tbd esports.Event
	tbd.State = esports.StateUnstarted
	if got := eventTitle(tbd); got != "TBD vs TBD" {
		t.Errorf("eventTitle(no teams) = %q", got)
	}
}

// With -game, a live match listed before the game's first window must not
// take over the Live tab.
func TestGameFlagBeatsLiveMatch(t *testing.T) {
	m := New(Config{LeagueID: esports.WorldsLeagueID, LeagueSlug: "worlds", GameID: "113475798006664579"})
	var tm tea.Model = m
	live := esports.Event{League: esports.League{Slug: "worlds"}, Match: esports.Match{ID: "other-live-match"}}
	tm, _ = tm.Update(liveMsg{events: []esports.Event{live}})
	if id := tm.(Model).matchID; id != "" {
		t.Fatalf("live match %s opened before the requested game", id)
	}
	var start livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	tm, _ = tm.Update(gameStartMsg{gameID: "113475798006664579", window: &start})
	if id := tm.(Model).matchID; id != "113475798006664578" {
		t.Fatalf("opened match %s, want the -game match", id)
	}
}

// selected returns the teams of the selected match, e.g. "KC-C9".
func selected(m Model) string {
	id := m.selectedID()
	find := func(ms ...*bracket.Match) string {
		for _, x := range ms {
			if x != nil && x.ID == id {
				return x.Teams[0].Code + "-" + x.Teams[1].Code
			}
		}
		return ""
	}
	b := m.bracket
	if s := find(b.PlayIn.All...); s != "" {
		return s
	}
	for _, r := range b.Swiss.Rounds {
		if s := find(r...); s != "" {
			return s
		}
	}
	return find(append(append(append([]*bracket.Match{}, b.Knockout.Quarterfinals...), b.Knockout.Semifinals...), b.Knockout.Final)...)
}

func TestKnockoutArrows(t *testing.T) {
	m := fixtureModel(t)
	m, _ = press(t, m, "4")
	steps := []struct{ key, want string }{
		{"", "HLE-GEN"},     // first quarterfinal
		{"right", "KT-GEN"}, // its semifinal
		{"right", "T1-KT"},  // the final
		{"right", "T1-KT"},  // nothing further right
		{"left", "KT-GEN"},  // tie between the semis goes to the upper one
		{"down", "TES-T1"},  // the other semifinal
		{"left", "G2-TES"},  // nearest quarterfinal (rows 10 and 15 vs 12)
		{"down", "T1-AL"},   // last quarterfinal
		{"down", "T1-AL"},   // stays at the bottom of the column
		{"up", "G2-TES"},    // back up
		{"right", "TES-T1"}, // its semifinal
		{"up", "KT-GEN"},    // upper semifinal
		{"left", "HLE-GEN"}, // nearest quarterfinal on a tie: the upper one
	}
	for i, s := range steps {
		if s.key != "" {
			m, _ = press(t, m, s.key)
		}
		if got := selected(m); got != s.want {
			t.Fatalf("step %d (%s): selected %s, want %s", i, s.key, got, s.want)
		}
	}
}

func TestPlayInArrows(t *testing.T) {
	m := fixtureModel(t)
	var st struct {
		Data struct {
			Standings []struct {
				Stages []esports.Stage `json:"stages"`
			} `json:"standings"`
		} `json:"data"`
	}
	readJSON(t, "../bracket/testdata/standings_2026.json", &st)
	var events []esports.Event
	for _, f := range []string{"schedule_page.json", "schedule_newer.json"} {
		var sc struct {
			Data struct {
				Schedule struct {
					Events []esports.Event `json:"events"`
				} `json:"schedule"`
			} `json:"data"`
		}
		readJSON(t, "../esports/testdata/"+f, &sc)
		events = append(events, sc.Data.Schedule.Events...)
	}
	m.bracket = bracket.Build(st.Data.Standings[0].Stages, events)
	m, _ = press(t, m, "2")

	steps := []struct{ key, want string }{
		{"", "KC-C9"},        // first opener
		{"right", "KC-MVK"},  // upper final
		{"right", "TBD-TBD"}, // final
		{"left", "KC-MVK"},   // tie between upper and lower final: upper
		{"down", "TBD-TBD"},  // lower final
		{"left", "C9-FUR"},   // lower round 1, same row
		{"up", "MVK-FUR"},    // second opener
		{"up", "KC-C9"},      // first opener
		{"up", "KC-C9"},      // stays at the top
	}
	for i, s := range steps {
		if s.key != "" {
			m, _ = press(t, m, s.key)
		}
		if got := selected(m); got != s.want {
			t.Fatalf("step %d (%s): selected %s, want %s", i, s.key, got, s.want)
		}
	}
	if m.tab != tabPlayIn {
		t.Errorf("arrows left the Play-In tab (tab %d)", m.tab)
	}
}

func TestSwissArrows(t *testing.T) {
	m := fixtureModel(t)
	m, _ = press(t, m, "3")
	r := m.bracket.Swiss.Rounds
	id := func() string { return m.selectedID() }

	if id() != r[0][0].ID {
		t.Fatal("Swiss does not start on the first match")
	}
	// Down stays inside round 1 and stops at its last match.
	for range 20 {
		m, _ = press(t, m, "down")
	}
	if id() != r[0][len(r[0])-1].ID {
		t.Errorf("down left round 1 or stopped early")
	}
	// Right moves one round at a time, landing on the nearest row.
	for want := 1; want < len(r); want++ {
		m, _ = press(t, m, "right")
		if !containsID(r[want], id()) {
			t.Fatalf("right %d: selection not in round %d", want, want+1)
		}
	}
	m, _ = press(t, m, "right")
	if !containsID(r[len(r)-1], id()) {
		t.Error("right past the last round moved the selection")
	}
	m, _ = press(t, m, "left")
	if !containsID(r[len(r)-2], id()) {
		t.Error("left did not go back a round")
	}
}

// Opening a finished game must not flash its opening frame as a live 00:00
// before the final frames arrive.
func TestFinishedGameNoLiveFlash(t *testing.T) {
	m := fixtureModel(t)
	m.inMenu, m.tab = false, tabLive
	m.matchID = "113475798006664578"
	m.details = &esports.EventDetails{ID: m.matchID, Match: esports.Match{
		State: esports.StateCompleted,
		Teams: []esports.Team{
			{ID: "98767991853197861", Name: "T1", Code: "T1", Result: &esports.Result{GameWins: 1}},
			{ID: "98926509892121852", Name: "FlyQuest", Code: "FLY", Result: &esports.Result{GameWins: 0}},
		},
		Games: []esports.Game{{ID: "113475798006664579", Number: 1, State: esports.StateCompleted}},
	}}
	m.gameIdx = 0

	var start, end livestats.Window
	readJSON(t, "../livestats/testdata/window_start.json", &start)
	readJSON(t, "../livestats/testdata/window_end.json", &end)
	var tm tea.Model = m
	tm, _ = tm.Update(gameStartMsg{gameID: "113475798006664579", window: &start})
	v := plain(tm.(Model))
	if strings.Contains(v, "LIVE") || strings.Contains(v, "00:00") || !strings.Contains(v, "loading") {
		t.Errorf("finished game shown from its opening frame:\n%s", v)
	}
	tm, _ = tm.Update(gamePollMsg{gameID: "113475798006664579", window: &end})
	v = plain(tm.(Model))
	if !strings.Contains(v, "FINAL") || strings.Contains(v, "loading") {
		t.Errorf("final state not shown after the poll:\n%s", v)
	}
}
