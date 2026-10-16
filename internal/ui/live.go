package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
	"github.com/BigBoyChnusti/worlds-tui/internal/livestats"
)

func fmtClock(d time.Duration) string {
	d = d.Truncate(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func fmtCountdown(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	d = d.Truncate(time.Minute)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, h)
	case h > 0:
		return fmt.Sprintf("%dh %02dm", h, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// eventFor finds the schedule or live entry of a match.
func (m Model) eventFor(matchID string) (esports.Event, bool) {
	for _, e := range m.live {
		if e.Match.ID == matchID {
			return e, true
		}
	}
	for _, e := range m.events {
		if e.Match.ID == matchID {
			return e, true
		}
	}
	return esports.Event{}, false
}

func (m Model) renderLive() string {
	if m.matchID == "" {
		return m.renderNoLive()
	}
	if m.details == nil {
		return stMuted.Render("loading match…")
	}
	var b strings.Builder
	b.WriteString(m.renderSeriesHeader())
	b.WriteString("\n\n")

	gm, ok := m.currentGame()
	if !ok {
		b.WriteString(m.renderNotStarted())
		return b.String()
	}
	g := m.games[gm.ID]
	if g == nil || g.Latest == nil {
		if gm.State == esports.StateUnstarted && m.details.Match.State != esports.StateInProgress {
			b.WriteString(m.renderNotStarted())
		} else {
			b.WriteString(m.renderWaiting(gm))
		}
		return b.String()
	}
	b.WriteString(m.renderGame(gm, g))
	return b.String()
}

func (m Model) renderNoLive() string {
	var b strings.Builder
	b.WriteString(stBold.Render("No Worlds match is live right now."))
	b.WriteString("\n\n")
	var upcoming []esports.Event
	for _, e := range m.events {
		if e.State != esports.StateCompleted && e.StartTime.After(m.now.Add(-6*time.Hour)) {
			upcoming = append(upcoming, e)
		}
	}
	if len(upcoming) == 0 {
		b.WriteString(stMuted.Render("No upcoming matches in the schedule."))
		return b.String()
	}
	next := upcoming[0]
	fmt.Fprintf(&b, "Next up in %s: %s  %s\n\n",
		stAccent.Render(fmtCountdown(next.StartTime.Sub(m.now))),
		stBold.Render(eventTitle(next)),
		stMuted.Render(next.BlockName+" · "+bestOf(next.Match.Strategy.Count)))
	b.WriteString(stSection.Render("Upcoming"))
	b.WriteString("\n")
	for i, e := range upcoming {
		if i == 8 {
			break
		}
		fmt.Fprintf(&b, "  %s  %s %s  %s\n",
			e.StartTime.Local().Format("Mon 02 Jan 15:04"),
			pad(e.BlockName, 14), pad(eventTitle(e), 16), stMuted.Render(bestOf(e.Match.Strategy.Count)))
	}
	b.WriteString("\n")
	b.WriteString(stMuted.Render("Pick any match in the bracket or schedule tabs and press enter to view it."))
	return b.String()
}

func eventTitle(e esports.Event) string {
	codes := []string{"TBD", "TBD"}
	for i := 0; i < 2 && i < len(e.Match.Teams); i++ {
		if e.Match.Teams[i].Known() {
			codes[i] = e.Match.Teams[i].Code
		}
	}
	return codes[0] + " vs " + codes[1]
}

// renderWaiting covers a game the gateway reports as running while the
// stats feed has no frames yet: during the draft and the break before a
// game loads, and when the feed lags behind a running game (seen for 25
// minutes in the 2026 Play-In).
func (m Model) renderWaiting(gm esports.Game) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n\n", stBold.Render(fmt.Sprintf("Game %d", gm.Number)), stAccent.Render("waiting for the game to load"))
	b.WriteString("The match is live, but the stats feed has no data for this game yet.\n")
	b.WriteString("That happens during the draft and the break between games, and sometimes\n")
	b.WriteString("the feed is late for a game that is already running. Stats appear as soon\n")
	b.WriteString("as it delivers them.\n\n")
	if t, ok := m.startAttempt[gm.ID]; ok {
		next := t.Add(startRetryEvery).Sub(m.now)
		fmt.Fprintf(&b, "%s", stMuted.Render(fmt.Sprintf("Last checked %s, next check in %ds.", t.Local().Format("15:04:05"), max(0, int(next.Seconds())))))
	} else {
		b.WriteString(stMuted.Render("Checking…"))
	}
	if m.gameIdx > 0 {
		b.WriteString("\n" + stMuted.Render("Press ← to see the previous game."))
	}
	return b.String()
}

func (m Model) renderNotStarted() string {
	e, ok := m.eventFor(m.matchID)
	if ok && e.StartTime.After(m.now) {
		return fmt.Sprintf("Starts in %s  %s", stAccent.Render(fmtCountdown(e.StartTime.Sub(m.now))),
			stMuted.Render(e.StartTime.Local().Format("Mon 02 Jan 15:04")))
	}
	return stMuted.Render("Game not started yet.")
}

// teamCode maps a team id from the match to its code.
func (m Model) teamCode(id string) string {
	for _, t := range m.details.Match.Teams {
		if t.ID == id {
			return t.Code
		}
	}
	return "?"
}

func (m Model) renderSeriesHeader() string {
	d := m.details
	teams := d.Match.Teams
	if len(teams) < 2 {
		return stMuted.Render("match " + d.ID)
	}
	left, right := teams[0], teams[1]
	score := func(t esports.Team) int {
		if t.Result != nil {
			return t.Result.GameWins
		}
		return 0
	}
	title := fmt.Sprintf("%s  %s  %s",
		stBold.Render(left.Name),
		stAccent.Render(fmt.Sprintf("%d – %d", score(left), score(right))),
		stBold.Render(right.Name))

	meta := []string{}
	if e, ok := m.eventFor(d.ID); ok && e.BlockName != "" {
		meta = append(meta, e.BlockName)
	}
	if bo := bestOf(d.Match.Strategy.Count); bo != "" {
		meta = append(meta, bo)
	}
	if m.pinned {
		meta = append(meta, "pinned")
	} else {
		meta = append(meta, "following live")
	}
	line1 := title + "   " + stMuted.Render(strings.Join(meta, " · "))

	// Name and state marker are styled separately: lipgloss underlines rune
	// by rune, which breaks escape codes already inside the string.
	var games []string
	for i, g := range d.Match.Games {
		name := stMuted.Render(fmt.Sprintf("G%d", g.Number))
		if i == m.gameIdx {
			name = stGameActive.Render(fmt.Sprintf("G%d", g.Number))
		}
		var marker string
		switch g.State {
		case esports.StateInProgress:
			marker = stLive.Render("●")
		case esports.StateCompleted:
			marker = stMuted.Render("✓")
		default:
			marker = stMuted.Render("·")
		}
		games = append(games, m.zones.Mark(gameZone(i), " "+name+" "+marker+" "))
	}
	return line1 + "\n" + strings.Join(games, "")
}

func (m Model) renderGame(gm esports.Game, full *livestats.Game) string {
	// Everything below shows the game at the playhead, not the newest frame.
	at := m.playhead(gm.ID, full)
	g := full.At(at)
	var b strings.Builder
	// The feed's team ids decide the sides; the gateway's game "side" field
	// has been seen swapped.
	blueCode, redCode := "BLUE", "RED"
	for _, t := range gm.Teams {
		if t.Side == "blue" {
			blueCode = m.teamCode(t.ID)
		} else if t.Side == "red" {
			redCode = m.teamCode(t.ID)
		}
	}
	if c := m.teamCode(g.Meta.BlueTeamMetadata.EsportsTeamID); c != "?" {
		blueCode = c
	}
	if c := m.teamCode(g.Meta.RedTeamMetadata.EsportsTeamID); c != "?" {
		redCode = c
	}

	state := g.State()
	stateTag := ""
	switch state {
	case livestats.GameInGame:
		stateTag = stLive.Render("● LIVE") + stMuted.Render("  stats run about a minute behind the broadcast")
	case livestats.GamePaused:
		stateTag = stAccent.Render("❚❚ PAUSED")
	case livestats.GameFinished:
		stateTag = stMuted.Render("FINAL")
	}
	fmt.Fprintf(&b, "%s  %s  %s\n\n", stBold.Render(fmt.Sprintf("Game %d", gm.Number)),
		stAccent.Render(fmtClock(full.ClockAt(at))), stateTag)

	// Team summary.
	bt, rt := g.Latest.BlueTeam, g.Latest.RedTeam
	hdr := pad("", 8) + padLeft("Kills", 6) + padLeft("Gold", 9) + padLeft("Towers", 8) +
		padLeft("Inhib", 7) + padLeft("Baron", 7) + "  Dragons"
	b.WriteString(stMuted.Render(hdr) + "\n")
	row := func(code string, st func(...string) string, t livestats.TeamFrame) string {
		var drakes []string
		for _, d := range t.Dragons {
			drakes = append(drakes, dragonTag(d))
		}
		return st(pad(code, 8)) + padLeft(fmt.Sprint(t.TotalKills), 6) + padLeft(fmtGold(t.TotalGold), 9) +
			padLeft(fmt.Sprint(t.Towers), 8) + padLeft(fmt.Sprint(t.Inhibitors), 7) +
			padLeft(fmt.Sprint(t.Barons), 7) + "  " + strings.Join(drakes, " ")
	}
	b.WriteString(row(blueCode, stBlue.Render, bt) + "\n")
	b.WriteString(row(redCode, stRed.Render, rt) + "\n\n")

	diff := g.GoldDiff()
	lead := stMuted.Render("Gold even")
	if diff > 0 {
		lead = "Gold lead " + stBlue.Render(blueCode) + " " + fmtDiff(diff)
	} else if diff < 0 {
		lead = "Gold lead " + stRed.Render(redCode) + " " + fmtDiff(diff)
	}
	b.WriteString(lead + "\n")
	if hist := g.History(time.Minute); len(hist) > 1 {
		b.WriteString(sparkline(hist, max(10, m.width-2)) + "\n")
	}
	b.WriteString("\n")

	b.WriteString(m.renderPlayers(g, "blue", blueCode))
	b.WriteString("\n")
	b.WriteString(m.renderPlayers(g, "red", redCode))
	return b.String()
}

var roleShort = map[string]string{"top": "TOP", "jungle": "JNG", "mid": "MID", "bottom": "BOT", "support": "SUP"}

func (m Model) renderPlayers(g *livestats.Game, side, code string) string {
	var b strings.Builder
	st := stBlue
	if side == "red" {
		st = stRed
	}
	b.WriteString(st.Render(code+" "+strings.ToUpper(side)) + "\n")
	b.WriteString(stMuted.Render(pad("Role", 5)+pad("Player", 14)+pad("Champion", 14)+pad("K/D/A", 9)+
		padLeft("CS", 5)+padLeft("Gold", 8)+padLeft("Lane±", 8)+padLeft("Lvl", 5)+padLeft("KP", 6)+padLeft("Dmg", 6)) + "\n")
	for _, p := range g.Players(side) {
		name := p.Meta.SummonerName
		if i := strings.Index(name, " "); i > 0 && strings.HasPrefix(name, code+" ") {
			name = name[i+1:]
		}
		lane := g.LaneDiff(p.Meta.Role)
		if side == "red" {
			lane = -lane
		}
		laneStr := stMuted.Render("even")
		if lane > 0 {
			laneStr = stGood.Render("+" + fmtGold(lane))
		} else if lane < 0 {
			laneStr = stMuted.Render("-" + fmtGold(-lane))
		}
		kp, dmg := "", ""
		if p.Details != nil {
			kp = fmt.Sprintf("%.0f%%", p.Details.KillParticipation*100)
			dmg = fmt.Sprintf("%.0f%%", p.Details.ChampionDamageShare*100)
		}
		s := p.Stats
		fmt.Fprintf(&b, "%s%s%s%s%s%s%s%s%s%s\n",
			pad(roleShort[p.Meta.Role], 5),
			pad(name, 14),
			pad(m.ddData.ChampionName(p.Meta.ChampionID), 14),
			pad(fmt.Sprintf("%d/%d/%d", s.Kills, s.Deaths, s.Assists), 9),
			padLeft(fmt.Sprint(s.CreepScore), 5),
			padLeft(fmtGold(s.TotalGold), 8),
			padLeft(laneStr, 8),
			padLeft(fmt.Sprint(s.Level), 5),
			padLeft(kp, 6),
			padLeft(dmg, 6))
		if p.Details != nil && len(p.Details.Items) > 0 {
			var names []string
			for _, id := range p.Details.Items {
				names = append(names, m.ddData.ItemName(id))
			}
			b.WriteString(stMuted.Render("     "+strings.Join(names, " · ")) + "\n")
		}
	}
	return b.String()
}

// sparkline draws the gold difference over time, blue above zero and red
// below, keeping the most recent points that fit in width.
func sparkline(hist []livestats.GoldPoint, width int) string {
	if len(hist) > width {
		hist = hist[len(hist)-width:]
	}
	maxAbs := 1
	for _, p := range hist {
		maxAbs = max(maxAbs, p.Diff, -p.Diff)
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, p := range hist {
		v := p.Diff
		if v < 0 {
			v = -v
		}
		idx := v * (len(bars) - 1) / maxAbs
		ch := string(bars[idx])
		switch {
		case p.Diff > 0:
			b.WriteString(stBlue.Render(ch))
		case p.Diff < 0:
			b.WriteString(stRed.Render(ch))
		default:
			b.WriteString(stMuted.Render(ch))
		}
	}
	return b.String() + stMuted.Render(fmt.Sprintf("  max %s", fmtGold(maxAbs)))
}
