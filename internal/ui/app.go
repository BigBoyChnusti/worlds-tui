// Package ui is the Bubble Tea interface.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/BigBoyChnusti/worlds-tui/internal/bracket"
	"github.com/BigBoyChnusti/worlds-tui/internal/ddragon"
	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

const (
	tabLive = iota
	tabPlayIn
	tabSwiss
	tabKnockout
	tabSchedule
	tabCount
)

var tabNames = [tabCount]string{"1 Live", "2 Play-In", "3 Swiss", "4 Knockouts", "5 Schedule"}

const (
	requestTimeout  = 20 * time.Second
	liveListEvery   = 30 * time.Second
	detailsEvery    = 30 * time.Second
	startRetryEvery = 15 * time.Second
	backfillStep    = time.Minute
)

// Config holds startup options.
type Config struct {
	LeagueID       string
	LeagueSlug     string
	TournamentSlug string        // empty picks the current tournament
	GameID         string        // open this game on start (replay)
	Refresh        time.Duration // minimum time between live stats requests
	DataRefresh    time.Duration // standings/schedule refresh interval
}

// playState anchors the replay of a live game: game wall time at was
// shown at real time wall, and the playhead advances in real time from
// there.
type playState struct{ at, wall time.Time }

type Model struct {
	cfg   Config
	zones *zone.Manager // clickable regions of the last rendered view
	es    *esports.Client
	ls    *livestats.Client
	dd    *ddragon.Client

	tournament esports.Tournament
	stages     []esports.Stage
	events     []esports.Event
	bracket    bracket.Tournament
	live       []esports.Event

	tab        int
	inMenu     bool
	menuCursor int
	width      int
	height     int
	cursors    [tabCount]int
	scrolls    [tabCount]int
	now        time.Time

	// The match shown on the Live tab. pinned means the user chose it;
	// otherwise the first live Worlds match is followed automatically.
	matchID    string
	pinned     bool
	details    *esports.EventDetails
	gameIdx    int
	gamePinned bool
	preferGame string

	games        map[string]*livestats.Game
	startAttempt map[string]time.Time
	backfilled   map[string]bool
	polling      bool
	lastPoll     time.Time
	play         map[string]playState
	ddData       *ddragon.Data
	ddLoading    bool
	// Champion icons by champion key, downscaled for the player table.
	// iconsAsked holds keys whose load is running or done, so each icon is
	// requested once per Data Dragon version.
	icons      map[string]*champIcon
	iconsAsked map[string]bool

	dataLoading bool
	lastData    time.Time
	lastLive    time.Time
	lastDetails time.Time
	err         error
	errAt       time.Time
}

func New(cfg Config) Model {
	if cfg.Refresh <= 0 {
		cfg.Refresh = 2 * time.Second
	}
	if cfg.DataRefresh <= 0 {
		cfg.DataRefresh = time.Minute
	}
	return Model{
		cfg:          cfg,
		zones:        zone.New(),
		es:           esports.NewClient(),
		ls:           livestats.NewClient(),
		dd:           ddragon.NewClient(),
		games:        map[string]*livestats.Game{},
		startAttempt: map[string]time.Time{},
		backfilled:   map[string]bool{},
		play:         map[string]playState{},
		icons:        map[string]*champIcon{},
		iconsAsked:   map[string]bool{},
		gameIdx:      -1,
		preferGame:   cfg.GameID,
		now:          time.Now(),
		inMenu:       cfg.GameID == "",
	}
}

// Messages.
type (
	tickMsg       time.Time
	tournamentMsg struct {
		t   esports.Tournament
		err error
	}
	dataMsg struct {
		stages []esports.Stage
		events []esports.Event
		err    error
	}
	liveMsg struct {
		events []esports.Event
		err    error
	}
	detailsMsg struct {
		matchID string
		details *esports.EventDetails
		err     error
	}
	gameStartMsg struct {
		gameID string
		window *livestats.Window
		err    error
	}
	gamePollMsg struct {
		gameID  string
		window  *livestats.Window
		details *livestats.Details
		err     error
	}
	backfillMsg struct {
		gameID  string
		windows []*livestats.Window
	}
	ddMsg struct {
		data *ddragon.Data
		err  error
	}
	iconsMsg struct {
		version string
		icons   map[string]*champIcon
		failed  []string
	}
)

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout)
}

func (m Model) loadTournament() tea.Cmd {
	es, cfg := m.es, m.cfg
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		ts, err := es.GetTournamentsForLeague(c, cfg.LeagueID)
		if err != nil {
			return tournamentMsg{err: err}
		}
		if cfg.TournamentSlug != "" {
			for _, t := range ts {
				if t.Slug == cfg.TournamentSlug {
					return tournamentMsg{t: t}
				}
			}
			return tournamentMsg{err: fmt.Errorf("tournament %q not found", cfg.TournamentSlug)}
		}
		t, ok := esports.CurrentTournament(ts, time.Now())
		if !ok {
			return tournamentMsg{err: errors.New("league has no tournaments")}
		}
		return tournamentMsg{t: t}
	}
}

func (m Model) loadData() tea.Cmd {
	es, t, leagueID := m.es, m.tournament, m.cfg.LeagueID
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		stages, err := es.GetStandings(c, t.ID)
		if err != nil {
			return dataMsg{err: err}
		}
		from, _ := time.Parse(time.DateOnly, t.StartDate)
		events, err := es.GetSchedule(c, leagueID, from)
		if err != nil {
			return dataMsg{err: err}
		}
		if end, err := time.Parse(time.DateOnly, t.EndDate); err == nil {
			end = end.Add(48 * time.Hour)
			kept := events[:0]
			for _, e := range events {
				if e.StartTime.Before(end) {
					kept = append(kept, e)
				}
			}
			events = kept
		}
		return dataMsg{stages: stages, events: events}
	}
}

func (m Model) loadLive() tea.Cmd {
	es := m.es
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		ev, err := es.GetLive(c)
		return liveMsg{events: ev, err: err}
	}
}

func (m Model) loadDetails(matchID string) tea.Cmd {
	es := m.es
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		d, err := es.GetEventDetails(c, matchID)
		return detailsMsg{matchID: matchID, details: d, err: err}
	}
}

func (m Model) startGame(gameID string) tea.Cmd {
	ls := m.ls
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		w, err := ls.Window(c, gameID, time.Time{})
		return gameStartMsg{gameID: gameID, window: w, err: err}
	}
}

func (m Model) pollGame(gameID string, at time.Time) tea.Cmd {
	ls := m.ls
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		w, err := ls.Window(c, gameID, at)
		if err != nil {
			return gamePollMsg{gameID: gameID, err: err}
		}
		d, err := ls.Details(c, gameID, at)
		if err != nil && !errors.Is(err, livestats.ErrNoData) {
			return gamePollMsg{gameID: gameID, window: w, err: err}
		}
		return gamePollMsg{gameID: gameID, window: w, details: d}
	}
}

// backfill samples one window per minute between start and end so the
// gold-lead history covers the part of the game played before it was
// opened.
func (m Model) backfill(gameID string, start, end time.Time) tea.Cmd {
	ls := m.ls
	return func() tea.Msg {
		var times []time.Time
		for t := start.Add(backfillStep); t.Before(end); t = t.Add(backfillStep) {
			times = append(times, t)
		}
		out := make([]*livestats.Window, len(times))
		sem := make(chan struct{}, 6)
		var wg sync.WaitGroup
		for i, t := range times {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				c, cancel := ctx()
				defer cancel()
				if w, err := ls.Window(c, gameID, t); err == nil {
					out[i] = w
				}
			}()
		}
		wg.Wait()
		return backfillMsg{gameID: gameID, windows: out}
	}
}

func (m Model) loadDDragon(patch string) tea.Cmd {
	dd := m.dd
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		d, err := dd.ForPatch(c, patch)
		return ddMsg{data: d, err: err}
	}
}

// loadIcons fetches the icons of every champion in the loaded games that
// has not been asked for yet, in one command so a whole draft arrives as a
// single message. Keys are marked as asked here; failed ones are cleared
// again by the iconsMsg so a later game can retry them.
func (m *Model) loadIcons() tea.Cmd {
	if m.ddData == nil {
		return nil
	}
	var want []string
	for _, g := range m.games {
		for _, tm := range []livestats.TeamMeta{g.Meta.BlueTeamMetadata, g.Meta.RedTeamMetadata} {
			for _, p := range tm.ParticipantMetadata {
				if k := p.ChampionID; k != "" && !m.iconsAsked[k] {
					m.iconsAsked[k] = true
					want = append(want, k)
				}
			}
		}
	}
	if len(want) == 0 {
		return nil
	}
	dd, ver := m.dd, m.ddData.Version
	return func() tea.Msg {
		c, cancel := ctx()
		defer cancel()
		msg := iconsMsg{version: ver, icons: map[string]*champIcon{}}
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, k := range want {
			wg.Add(1)
			go func() {
				defer wg.Done()
				img, err := dd.ChampionIcon(c, ver, k)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					msg.failed = append(msg.failed, k)
					return
				}
				msg.icons[k] = newChampIcon(img)
			}()
		}
		wg.Wait()
		return msg
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick(), m.loadTournament(), m.loadLive()}
	if m.cfg.GameID != "" {
		cmds = append(cmds, m.startGame(m.cfg.GameID))
	}
	return tea.Batch(cmds...)
}

func (m *Model) setErr(err error) {
	if err != nil {
		m.err, m.errAt = err, time.Now()
	}
}

// currentGame returns the selected game of the open match.
func (m Model) currentGame() (esports.Game, bool) {
	if m.details == nil || m.gameIdx < 0 || m.gameIdx >= len(m.details.Match.Games) {
		return esports.Game{}, false
	}
	return m.details.Match.Games[m.gameIdx], true
}

// chooseGame picks the game to show when the user hasn't picked one: the
// preferred game, else the one in progress, else the last completed.
func chooseGame(d *esports.EventDetails, prefer string) int {
	games := d.Match.Games
	if len(games) == 0 {
		return -1
	}
	for i, g := range games {
		if prefer != "" && g.ID == prefer {
			return i
		}
	}
	for i, g := range games {
		if g.State == esports.StateInProgress {
			return i
		}
	}
	last := 0
	for i, g := range games {
		if g.State == esports.StateCompleted {
			last = i
		}
	}
	return last
}

// openMatch shows a match on the Live tab.
func (m *Model) openMatch(matchID string, pinned bool) tea.Cmd {
	m.tab = tabLive
	m.pinned = pinned
	if matchID == m.matchID {
		return nil
	}
	m.matchID = matchID
	m.details = nil
	m.gameIdx = -1
	m.gamePinned = false
	m.scrolls[tabLive] = 0
	m.lastDetails = time.Now()
	return m.loadDetails(matchID)
}

// followLive switches the Live tab to the first live match of the league
// when the user hasn't pinned another one.
func (m *Model) followLive() tea.Cmd {
	if m.pinned {
		return nil
	}
	for _, e := range m.live {
		if e.Match.ID == m.matchID {
			return nil
		}
	}
	if len(m.live) == 0 {
		return nil
	}
	tab := m.tab
	cmd := m.openMatch(m.live[0].Match.ID, false)
	m.tab = tab
	return cmd
}

// ensureGame starts loading the selected game and keeps polling it while
// it's running.
func (m *Model) ensureGame() tea.Cmd {
	gm, ok := m.currentGame()
	if !ok {
		return nil
	}
	g := m.games[gm.ID]
	if g == nil {
		if gm.State == esports.StateUnstarted && m.details.Match.State != esports.StateInProgress {
			return nil
		}
		if t, ok := m.startAttempt[gm.ID]; ok && time.Since(t) < startRetryEvery {
			return nil
		}
		m.startAttempt[gm.ID] = time.Now()
		return m.startGame(gm.ID)
	}
	if m.polling || g.State() == livestats.GameFinished {
		return nil
	}
	if g.Latest != nil && gm.State == esports.StateCompleted && m.backfilled[gm.ID] {
		return nil
	}
	if time.Since(m.lastPoll) < m.cfg.Refresh {
		return nil
	}
	// Ask for the window after the newest frame once the feed can have it.
	live := livestats.LiveStartingTime(time.Now())
	at := g.NextWindow(live)
	if at.After(live) {
		return nil
	}
	m.polling = true
	m.lastPoll = time.Now()
	return m.pollGame(gm.ID, at)
}

// playhead is the moment of a game the Live tab shows. For a running game
// it trails the newest frame by about one feed window and advances in real
// time, so the clock ticks every second and events appear when they
// happened. A finished game shows its last frame.
func (m Model) playhead(gameID string, g *livestats.Game) time.Time {
	if g == nil || g.Latest == nil {
		return time.Time{}
	}
	p, ok := m.play[gameID]
	if !ok {
		return g.Latest.Timestamp
	}
	t := p.at.Add(m.now.Sub(p.wall))
	if t.After(g.Latest.Timestamp) {
		t = g.Latest.Timestamp
	}
	return t
}

// advancePlay re-anchors the replay after new frames arrive, and holds it
// at the newest frame while waiting for more so it resumes smoothly.
func (m *Model) advancePlay(gameID string, g *livestats.Game, newData bool) {
	if g == nil || g.Latest == nil {
		return
	}
	latest := g.Latest.Timestamp
	p, ok := m.play[gameID]
	switch {
	case !ok && newData && g.State() != livestats.GameFinished:
		m.play[gameID] = playState{at: latest.Add(-livestats.WindowSpan), wall: m.now}
	case !ok:
		return
	case newData && m.playhead(gameID, g).Before(latest.Add(-3*livestats.WindowSpan)):
		// Fell far behind (feed burst or catch-up): skip ahead.
		m.play[gameID] = playState{at: latest.Add(-livestats.WindowSpan), wall: m.now}
	case p.at.Add(m.now.Sub(p.wall)).After(latest):
		m.play[gameID] = playState{at: latest, wall: m.now}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tickMsg:
		m.now = time.Time(msg)
		cmds := []tea.Cmd{tick()}
		if m.tournament.ID != "" && !m.dataLoading && time.Since(m.lastData) > m.cfg.DataRefresh {
			m.dataLoading = true
			cmds = append(cmds, m.loadData())
		}
		if time.Since(m.lastLive) > liveListEvery {
			m.lastLive = time.Now()
			cmds = append(cmds, m.loadLive())
		}
		if m.matchID != "" && time.Since(m.lastDetails) > detailsEvery &&
			(m.details == nil || m.details.Match.State != esports.StateCompleted) {
			m.lastDetails = time.Now()
			cmds = append(cmds, m.loadDetails(m.matchID))
		}
		if gm, ok := m.currentGame(); ok {
			m.advancePlay(gm.ID, m.games[gm.ID], false)
		}
		cmds = append(cmds, m.ensureGame())
		return m, tea.Batch(cmds...)

	case tournamentMsg:
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.tournament = msg.t
		m.dataLoading = true
		return m, m.loadData()

	case dataMsg:
		m.dataLoading = false
		m.lastData = time.Now()
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.stages, m.events = msg.stages, msg.events
		m.bracket = bracket.Build(m.stages, m.events)
		return m, nil

	case liveMsg:
		m.lastLive = time.Now()
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.live = m.live[:0]
		for _, e := range msg.events {
			if e.League.Slug == m.cfg.LeagueSlug && e.Match.ID != "" {
				m.live = append(m.live, e)
			}
		}
		return m, m.followLive()

	case detailsMsg:
		if msg.matchID != m.matchID {
			return m, nil
		}
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.details = msg.details
		if !m.gamePinned {
			m.gameIdx = chooseGame(m.details, m.preferGame)
		}
		return m, m.ensureGame()

	case gameStartMsg:
		if msg.err != nil {
			if !errors.Is(msg.err, livestats.ErrNoData) {
				m.setErr(msg.err)
			}
			return m, nil
		}
		g := livestats.NewGame(msg.gameID)
		g.SetStart(msg.window)
		m.games[msg.gameID] = g
		var cmds []tea.Cmd
		if m.matchID == "" && g.MatchID != "" {
			cmds = append(cmds, m.openMatch(g.MatchID, true))
		}
		if m.ddData == nil && !m.ddLoading && g.Meta.PatchVersion != "" {
			m.ddLoading = true
			cmds = append(cmds, m.loadDDragon(g.Meta.PatchVersion))
		}
		cmds = append(cmds, m.loadIcons())
		cmds = append(cmds, m.ensureGame())
		return m, tea.Batch(cmds...)

	case gamePollMsg:
		m.polling = false
		g := m.games[msg.gameID]
		if g == nil {
			return m, nil
		}
		if msg.err != nil && !errors.Is(msg.err, livestats.ErrNoData) {
			m.setErr(msg.err)
		}
		var before time.Time
		if g.Latest != nil {
			before = g.Latest.Timestamp
		}
		g.ApplyWindow(msg.window)
		g.ApplyDetails(msg.details)
		m.now = time.Now()
		m.advancePlay(msg.gameID, g, g.Latest != nil && g.Latest.Timestamp.After(before))
		if !m.backfilled[msg.gameID] && g.Latest != nil {
			m.backfilled[msg.gameID] = true
			return m, m.backfill(msg.gameID, g.Start, g.Latest.Timestamp)
		}
		return m, nil

	case backfillMsg:
		if g := m.games[msg.gameID]; g != nil {
			for _, w := range msg.windows {
				g.ApplyWindow(w)
			}
		}
		return m, nil

	case ddMsg:
		m.ddLoading = false
		if msg.err != nil {
			m.setErr(msg.err)
			return m, nil
		}
		m.ddData = msg.data
		return m, m.loadIcons()

	case iconsMsg:
		if m.ddData == nil || msg.version != m.ddData.Version {
			return m, nil
		}
		for k, ic := range msg.icons {
			m.icons[k] = ic
		}
		for _, k := range msg.failed {
			delete(m.iconsAsked, k)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.inMenu {
		if k.String() == "q" {
			return m, tea.Quit
		}
		m, quit := m.handleMenuKey(k.String())
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "esc", "m":
		m.inMenu = true
		m.menuCursor = m.tab
		return m, nil
	case "1", "2", "3", "4", "5":
		m.tab = int(k.String()[0] - '1')
		return m, nil
	case "right", "l":
		// Tabs change only with tab/shift+tab and the number keys.
		if m.tab == tabLive {
			return m.switchGame(1)
		}
		return m, nil
	case "left", "h":
		if m.tab == tabLive {
			return m.switchGame(-1)
		}
		return m, nil
	case "]":
		return m.switchGame(1)
	case "[":
		return m.switchGame(-1)
	case "tab":
		m.tab = (m.tab + 1) % tabCount
		return m, nil
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
		return m, nil
	case "down", "j":
		m.move(1)
		return m, nil
	case "up", "k":
		m.move(-1)
		return m, nil
	case "pgdown":
		m.move(10)
		return m, nil
	case "pgup":
		m.move(-10)
		return m, nil
	case "enter":
		ids := m.tabMatchIDs(m.tab)
		if c := m.cursors[m.tab]; c < len(ids) && ids[c] != "" {
			return m, m.openMatch(ids[c], true)
		}
		return m, nil
	case "n":
		// Cycle through the live matches.
		if len(m.live) == 0 {
			return m, nil
		}
		next := 0
		for i, e := range m.live {
			if e.Match.ID == m.matchID {
				next = (i + 1) % len(m.live)
			}
		}
		return m, m.openMatch(m.live[next].Match.ID, true)
	case "f":
		// Go back to following the live match automatically.
		m.pinned = false
		m.tab = tabLive
		return m, m.followLive()
	case "r":
		m.lastData, m.lastLive, m.lastDetails, m.lastPoll = time.Time{}, time.Time{}, time.Time{}, time.Time{}
		return m, nil
	}
	return m, nil
}

// handleMouse scrolls with the wheel and opens what was clicked: menu
// items, tabs, games of the open match and matches in the bracket views.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action == tea.MouseActionPress {
		step := 0
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			step = -1
		case tea.MouseButtonWheelDown:
			step = 1
		}
		if step != 0 {
			if m.inMenu {
				m.menuCursor = (m.menuCursor + step + len(menuItems)) % len(menuItems)
			} else {
				if m.tab == tabLive {
					step *= 3
				}
				m.move(step)
			}
			return m, nil
		}
	}
	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if m.inMenu {
		for i := range menuItems {
			if m.zones.Get(menuZone(i)).InBounds(msg) {
				m.menuCursor = i
				m, quit := m.handleMenuKey("enter")
				if quit {
					return m, tea.Quit
				}
				return m, nil
			}
		}
		return m, nil
	}

	for i := range tabNames {
		if m.zones.Get(tabZone(i)).InBounds(msg) {
			m.tab = i
			return m, nil
		}
	}
	if m.tab == tabLive && m.details != nil {
		for i := range m.details.Match.Games {
			if m.zones.Get(gameZone(i)).InBounds(msg) {
				return m.switchGame(i - m.gameIdx)
			}
		}
		return m, nil
	}
	for c, id := range m.tabMatchIDs(m.tab) {
		if m.zones.Get(boxZone(id)).InBounds(msg) || m.zones.Get(lineZone(id)).InBounds(msg) {
			m.cursors[m.tab] = c
			return m, m.openMatch(id, true)
		}
	}
	return m, nil
}

// Zone ids for clickable regions.
func menuZone(i int) string     { return fmt.Sprintf("menu-%d", i) }
func tabZone(i int) string      { return fmt.Sprintf("tab-%d", i) }
func gameZone(i int) string     { return fmt.Sprintf("game-%d", i) }
func boxZone(id string) string  { return "box-" + id }
func lineZone(id string) string { return "line-" + id }

func (m Model) switchGame(delta int) (tea.Model, tea.Cmd) {
	if m.details == nil || len(m.details.Match.Games) == 0 {
		return m, nil
	}
	n := len(m.details.Match.Games)
	m.gameIdx = (m.gameIdx + delta + n) % n
	m.gamePinned = true
	m.lastPoll = time.Time{}
	m.scrolls[tabLive] = 0
	return m, m.ensureGame()
}

func (m *Model) move(delta int) {
	if m.tab == tabLive {
		m.scrolls[tabLive] = max(0, m.scrolls[tabLive]+delta)
		return
	}
	n := len(m.tabMatchIDs(m.tab))
	if n == 0 {
		return
	}
	m.cursors[m.tab] = min(n-1, max(0, m.cursors[m.tab]+delta))
}

// tabMatchIDs is the cursor order of the selectable matches on a tab.
func (m Model) tabMatchIDs(tab int) []string {
	var ids []string
	add := func(ms ...*bracket.Match) {
		for _, x := range ms {
			if x != nil {
				ids = append(ids, x.ID)
			}
		}
	}
	switch tab {
	case tabPlayIn:
		add(m.playInOrder()...)
	case tabSwiss:
		for _, r := range m.bracket.Swiss.Rounds {
			add(r...)
		}
	case tabKnockout:
		add(m.bracket.Knockout.Quarterfinals...)
		add(m.bracket.Knockout.Semifinals...)
		add(m.bracket.Knockout.Final)
	case tabSchedule:
		for _, e := range m.events {
			ids = append(ids, e.Match.ID)
		}
	}
	return ids
}

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	if m.inMenu {
		return m.zones.Scan(m.renderMenu())
	}
	header := m.renderHeader()
	footer := m.renderFooter()
	bodyH := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer))

	var body string
	focus := -1
	switch m.tab {
	case tabLive:
		body = m.renderLive()
	case tabPlayIn:
		body, focus = m.renderPlayIn()
	case tabSwiss:
		body, focus = m.renderSwiss()
	case tabKnockout:
		body, focus = m.renderKnockout()
	case tabSchedule:
		body, focus = m.renderSchedule()
	}
	body = m.crop(body, focus, bodyH)
	return m.zones.Scan(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
}

// crop fits body into h lines. With a focus line the view scrolls just
// enough to keep it visible; otherwise the tab's scroll offset is used.
func (m Model) crop(body string, focus, h int) string {
	lines := strings.Split(body, "\n")
	off := m.scrolls[m.tab]
	if focus >= 0 {
		off = max(0, focus-h+2)
	}
	off = max(0, min(off, len(lines)-h))
	out := lines[off:min(len(lines), off+h)]
	for len(out) < h {
		out = append(out, "")
	}
	for i, l := range out {
		out[i] = truncateANSI(l, m.width)
	}
	return strings.Join(out, "\n")
}

func truncateANSI(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}

func (m Model) renderHeader() string {
	title := stTitle.Render("WORLDS")
	if m.tournament.Slug != "" {
		title += stMuted.Render(" " + strings.ReplaceAll(m.tournament.Slug, "_", " "))
	}
	var tabs []string
	for i, n := range tabNames {
		if i == m.tab {
			tabs = append(tabs, m.zones.Mark(tabZone(i), stTabActive.Render(n)))
		} else {
			tabs = append(tabs, m.zones.Mark(tabZone(i), stTab.Render(n)))
		}
	}
	live := ""
	if len(m.live) > 0 {
		live = stLive.Render(fmt.Sprintf("  ● %d live", len(m.live)))
	}
	line := title + "  " + strings.Join(tabs, "") + live
	rule := stMuted.Render(strings.Repeat("─", max(0, m.width)))
	return line + "\n" + rule
}

func (m Model) renderFooter() string {
	var keys string
	switch m.tab {
	case tabLive:
		keys = "←/→ game · n next live · f follow live · ↑/↓ scroll"
	default:
		keys = "↑/↓ select · enter or click open match"
	}
	keys += " · tab/1-5 views · esc menu · r refresh · q quit"
	status := ""
	if m.err != nil && time.Since(m.errAt) < 30*time.Second {
		status = stRed.Render("⚠ " + m.err.Error())
	} else if !m.lastData.IsZero() {
		status = stMuted.Render("updated " + m.lastData.Local().Format("15:04:05"))
	} else {
		status = stMuted.Render("loading…")
	}
	return truncateANSI(status, m.width) + "\n" + truncateANSI(stMuted.Render(keys), m.width)
}
