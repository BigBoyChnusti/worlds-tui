// Package bracket turns gateway standings and schedule data into the three
// Worlds stages. The gateway's previousMatchIds links are empty, so the
// structure is inferred from schedule order and block names.
package bracket

import (
	"sort"
	"strings"
	"time"

	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
)

// Slot is one side of a match.
type Slot struct {
	Code    string
	Name    string
	Wins    int
	Outcome string // "win", "loss" or ""
}

// Known reports whether the slot holds a decided team.
func (s Slot) Known() bool { return s.Code != "" && s.Code != "TBD" }

type Match struct {
	ID     string
	State  string
	Start  time.Time
	Block  string
	BestOf int
	Teams  [2]Slot
}

// Winner returns the index of the winning slot, or -1.
func (m Match) Winner() int {
	for i, t := range m.Teams {
		if t.Outcome == "win" {
			return i
		}
	}
	return -1
}

// PlayIn is the four-team double-elimination bracket. Any position may be
// nil when the gateway has fewer matches than expected.
type PlayIn struct {
	Openers     [2]*Match
	UpperFinal  *Match
	LowerRound1 *Match
	LowerFinal  *Match
	Final       *Match
	All         []*Match // chronological
}

// Swiss holds the rounds and the running record of every team.
type Swiss struct {
	Rounds  [][]*Match
	Records map[string]Record // keyed by team code
	// Pools maps a match id to the record both teams carried into it.
	Pools map[string]Record
	Teams []TeamRecord // sorted standings
}

type Record struct{ Wins, Losses int }

type TeamRecord struct {
	Code   string
	Name   string
	Record Record
}

// Advanced and Eliminated follow the three-win / three-loss Swiss rule.
func (r Record) Advanced() bool   { return r.Wins >= 3 }
func (r Record) Eliminated() bool { return r.Losses >= 3 }

type Knockout struct {
	Quarterfinals []*Match
	Semifinals    []*Match
	Final         *Match
}

type Tournament struct {
	PlayIn   PlayIn
	Swiss    Swiss
	Knockout Knockout
}

// swissRoundSizes is the match count per round of a 16-team Swiss stage
// with best-of-three advancement and elimination matches.
var swissRoundSizes = []int{8, 8, 8, 6, 3}

// Build joins the standings stages with schedule events (for start time,
// block name and best-of) and lays out each stage.
func Build(stages []esports.Stage, events []esports.Event) Tournament {
	byID := map[string]esports.Event{}
	for _, e := range events {
		byID[e.Match.ID] = e
	}

	var t Tournament
	for _, st := range stages {
		var matches []*Match
		for _, sm := range st.Matches() {
			matches = append(matches, newMatch(sm, byID))
		}
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].Start.Before(matches[j].Start) })

		switch slug := strings.ToLower(st.Slug); {
		case strings.Contains(slug, "play"):
			t.PlayIn = buildPlayIn(matches)
		case strings.Contains(slug, "swiss"):
			t.Swiss = buildSwiss(matches)
		case strings.Contains(slug, "knock") || strings.Contains(slug, "bracket") || strings.Contains(slug, "playoff"):
			t.Knockout = buildKnockout(matches)
		}
	}
	return t
}

func newMatch(sm esports.StandingsMatch, byID map[string]esports.Event) *Match {
	m := &Match{ID: sm.ID, State: sm.State}
	if e, ok := byID[sm.ID]; ok {
		m.Start = e.StartTime
		m.Block = e.BlockName
		m.BestOf = e.Match.Strategy.Count
		if m.State == "" {
			m.State = e.State
		}
	}
	for i := 0; i < 2 && i < len(sm.Teams); i++ {
		tm := sm.Teams[i]
		s := Slot{Code: tm.Code, Name: tm.Name}
		if tm.Result != nil {
			s.Wins = tm.Result.GameWins
			s.Outcome = tm.Result.Outcome
		}
		m.Teams[i] = s
	}
	return m
}

// buildPlayIn assigns chronological positions: the two openers, then the
// upper final, the lower first round, the lower final and the final.
func buildPlayIn(ms []*Match) PlayIn {
	p := PlayIn{All: ms}
	at := func(i int) *Match {
		if i < len(ms) {
			return ms[i]
		}
		return nil
	}
	p.Openers = [2]*Match{at(0), at(1)}
	p.UpperFinal = at(2)
	p.LowerRound1 = at(3)
	p.LowerFinal = at(4)
	p.Final = at(5)
	return p
}

func buildSwiss(ms []*Match) Swiss {
	s := Swiss{Records: map[string]Record{}, Pools: map[string]Record{}}

	total := 0
	for _, n := range swissRoundSizes {
		total += n
	}
	if len(ms) == total {
		i := 0
		for _, n := range swissRoundSizes {
			s.Rounds = append(s.Rounds, ms[i:i+n])
			i += n
		}
	} else {
		s.Rounds = groupByDay(ms)
	}

	names := map[string]string{}
	for _, round := range s.Rounds {
		// Pools reflect records before the round, so read them all first.
		for _, m := range round {
			if m.Teams[0].Known() {
				s.Pools[m.ID] = s.Records[m.Teams[0].Code]
			}
		}
		for _, m := range round {
			for _, t := range m.Teams {
				if t.Known() {
					names[t.Code] = t.Name
					if _, ok := s.Records[t.Code]; !ok {
						s.Records[t.Code] = Record{}
					}
				}
			}
			w := m.Winner()
			if m.State != esports.StateCompleted || w < 0 {
				continue
			}
			win, loss := m.Teams[w].Code, m.Teams[1-w].Code
			r := s.Records[win]
			r.Wins++
			s.Records[win] = r
			r = s.Records[loss]
			r.Losses++
			s.Records[loss] = r
		}
	}

	for code, r := range s.Records {
		s.Teams = append(s.Teams, TeamRecord{Code: code, Name: names[code], Record: r})
	}
	sort.Slice(s.Teams, func(i, j int) bool {
		a, b := s.Teams[i].Record, s.Teams[j].Record
		if a.Wins-a.Losses != b.Wins-b.Losses {
			return a.Wins-a.Losses > b.Wins-b.Losses
		}
		if a.Wins != b.Wins {
			return a.Wins > b.Wins
		}
		return s.Teams[i].Code < s.Teams[j].Code
	})
	return s
}

func groupByDay(ms []*Match) [][]*Match {
	var out [][]*Match
	var last string
	for _, m := range ms {
		day := m.Start.UTC().Format(time.DateOnly)
		if len(out) == 0 || day != last {
			out = append(out, nil)
			last = day
		}
		out[len(out)-1] = append(out[len(out)-1], m)
	}
	return out
}

func buildKnockout(ms []*Match) Knockout {
	var k Knockout
	for _, m := range ms {
		b := strings.ToLower(m.Block)
		switch {
		case strings.Contains(b, "quarter"):
			k.Quarterfinals = append(k.Quarterfinals, m)
		case strings.Contains(b, "semi"):
			k.Semifinals = append(k.Semifinals, m)
		case strings.Contains(b, "final"):
			k.Final = m
		}
	}
	// Without schedule block names fall back to the 4-2-1 order.
	if len(k.Quarterfinals)+len(k.Semifinals) == 0 && k.Final == nil && len(ms) == 7 {
		k.Quarterfinals, k.Semifinals, k.Final = ms[:4], ms[4:6], ms[6]
	}
	return k
}
