package livestats

import (
	"sort"
	"time"
)

// maxContiguousGap is the largest gap between two observed frames that is
// still treated as continuous when accounting for pauses. Larger gaps come
// from sampled backfill and are ignored.
const maxContiguousGap = 15 * time.Second

// Roles in the order the feed lists participants.
var Roles = []string{"top", "jungle", "mid", "bottom", "support"}

type observation struct {
	at       time.Time
	state    string
	blueGold int
	redGold  int
}

// GoldPoint is the blue-minus-red gold difference at a game clock time.
type GoldPoint struct {
	Clock time.Duration
	Diff  int
}

// Game accumulates window and details responses for one game into a
// current view plus derived values the feed doesn't provide: the game
// clock and gold-lead history.
type Game struct {
	ID       string
	MatchID  string
	Meta     GameMetadata
	Start    time.Time
	Latest   *WindowFrame
	Details  *DetailsFrame
	obs      []observation
	hasStart bool
}

func NewGame(id string) *Game {
	return &Game{ID: id}
}

// SetStart records the window returned without a starting time, whose first
// frame marks the beginning of the game.
func (g *Game) SetStart(w *Window) {
	if w == nil || len(w.Frames) == 0 {
		return
	}
	g.Start = w.Frames[0].Timestamp
	g.hasStart = true
	g.ApplyWindow(w)
}

// ApplyWindow merges a window response. Frames may arrive in any order.
func (g *Game) ApplyWindow(w *Window) {
	if w == nil {
		return
	}
	if g.Meta.PatchVersion == "" && len(w.GameMetadata.BlueTeamMetadata.ParticipantMetadata) > 0 {
		g.Meta = w.GameMetadata
	}
	if g.MatchID == "" {
		g.MatchID = w.EsportsMatchID
	}
	for i := range w.Frames {
		f := w.Frames[i]
		g.insert(observation{at: f.Timestamp, state: f.GameState, blueGold: f.BlueTeam.TotalGold, redGold: f.RedTeam.TotalGold})
		if g.Latest == nil || !f.Timestamp.Before(g.Latest.Timestamp) {
			g.Latest = &f
		}
	}
	if !g.hasStart && len(g.obs) > 0 {
		g.Start = g.obs[0].at
	}
}

// ApplyDetails keeps the newest details frame.
func (g *Game) ApplyDetails(d *Details) {
	if d == nil {
		return
	}
	for i := range d.Frames {
		f := d.Frames[i]
		if g.Details == nil || !f.Timestamp.Before(g.Details.Timestamp) {
			g.Details = &f
		}
	}
}

func (g *Game) insert(o observation) {
	i := sort.Search(len(g.obs), func(i int) bool { return !g.obs[i].at.Before(o.at) })
	if i < len(g.obs) && g.obs[i].at.Equal(o.at) {
		g.obs[i] = o
		return
	}
	g.obs = append(g.obs, observation{})
	copy(g.obs[i+1:], g.obs[i:])
	g.obs[i] = o
}

// State is the game state of the newest frame, or "" before any data.
func (g *Game) State() string {
	if g.Latest == nil {
		return ""
	}
	return g.Latest.GameState
}

// pausedBefore sums the paused time between contiguous observations up to
// and including index n.
func (g *Game) pausedBefore(n int) time.Duration {
	var paused time.Duration
	for i := 1; i <= n && i < len(g.obs); i++ {
		gap := g.obs[i].at.Sub(g.obs[i-1].at)
		if g.obs[i-1].state == GamePaused && gap <= maxContiguousGap {
			paused += gap
		}
	}
	return paused
}

// Clock is the in-game time of the newest frame: wall time since the first
// frame minus observed pauses. Pauses that fall between sampled frames are
// not visible, so the value is an approximation of the in-game timer.
func (g *Game) Clock() time.Duration {
	if g.Latest == nil || g.Start.IsZero() {
		return 0
	}
	c := g.Latest.Timestamp.Sub(g.Start) - g.pausedBefore(len(g.obs)-1)
	if c < 0 {
		return 0
	}
	return c
}

// GoldDiff is blue team gold minus red team gold in the newest frame.
func (g *Game) GoldDiff() int {
	if g.Latest == nil {
		return 0
	}
	return g.Latest.BlueTeam.TotalGold - g.Latest.RedTeam.TotalGold
}

// History returns the gold difference sampled once per step of game clock,
// using the last observation in each step.
func (g *Game) History(step time.Duration) []GoldPoint {
	if len(g.obs) == 0 || g.Start.IsZero() {
		return nil
	}
	var out []GoldPoint
	var paused time.Duration
	for i, o := range g.obs {
		if i > 0 {
			gap := o.at.Sub(g.obs[i-1].at)
			if g.obs[i-1].state == GamePaused && gap <= maxContiguousGap {
				paused += gap
			}
		}
		clock := o.at.Sub(g.Start) - paused
		if clock < 0 {
			clock = 0
		}
		bucket := clock.Truncate(step)
		p := GoldPoint{Clock: bucket, Diff: o.blueGold - o.redGold}
		if n := len(out); n > 0 && out[n-1].Clock == bucket {
			out[n-1] = p
		} else {
			out = append(out, p)
		}
	}
	return out
}

// Player joins a participant's metadata with its newest window and details
// stats.
type Player struct {
	Meta    ParticipantMeta
	Stats   WindowParticipant
	Details *DetailsParticipant
}

// Players returns the five players of a side ("blue" or "red") in feed
// order (top, jungle, mid, bottom, support).
func (g *Game) Players(side string) []Player {
	meta := g.Meta.BlueTeamMetadata.ParticipantMetadata
	var stats []WindowParticipant
	if g.Latest != nil {
		stats = g.Latest.BlueTeam.Participants
	}
	if side == "red" {
		meta = g.Meta.RedTeamMetadata.ParticipantMetadata
		stats = nil
		if g.Latest != nil {
			stats = g.Latest.RedTeam.Participants
		}
	}
	out := make([]Player, 0, len(meta))
	for _, m := range meta {
		p := Player{Meta: m}
		for _, s := range stats {
			if s.ParticipantID == m.ParticipantID {
				p.Stats = s
			}
		}
		if g.Details != nil {
			for i := range g.Details.Participants {
				if g.Details.Participants[i].ParticipantID == m.ParticipantID {
					p.Details = &g.Details.Participants[i]
				}
			}
		}
		out = append(out, p)
	}
	return out
}

// LaneDiff returns blue minus red gold for the players sharing role.
func (g *Game) LaneDiff(role string) int {
	var blue, red int
	for _, p := range g.Players("blue") {
		if p.Meta.Role == role {
			blue = p.Stats.TotalGold
		}
	}
	for _, p := range g.Players("red") {
		if p.Meta.Role == role {
			red = p.Stats.TotalGold
		}
	}
	return blue - red
}
