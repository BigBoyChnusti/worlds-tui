package bracket

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/BigBoyChnusti/worlds-tui/internal/esports"
)

func loadStages(t *testing.T, file string) []esports.Stage {
	t.Helper()
	var r struct {
		Data struct {
			Standings []struct {
				Stages []esports.Stage `json:"stages"`
			} `json:"standings"`
		} `json:"data"`
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r.Data.Standings[0].Stages
}

func loadEvents(t *testing.T, files ...string) []esports.Event {
	t.Helper()
	var out []esports.Event
	for _, f := range files {
		var r struct {
			Data struct {
				Schedule struct {
					Events []esports.Event `json:"events"`
				} `json:"schedule"`
			} `json:"data"`
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r.Data.Schedule.Events...)
	}
	return out
}

var scheduleFiles = []string{
	"../esports/testdata/schedule_older.json",
	"../esports/testdata/schedule_page.json",
	"../esports/testdata/schedule_newer.json",
}

func TestWorlds2025(t *testing.T) {
	tour := Build(loadStages(t, "testdata/standings_2025.json"), loadEvents(t, scheduleFiles...))

	s := tour.Swiss
	if len(s.Rounds) != 5 {
		t.Fatalf("swiss rounds = %d, want 5", len(s.Rounds))
	}
	want := map[string]Record{
		"AL": {3, 0}, "KT": {3, 0},
		"G2": {3, 1}, "GEN": {3, 1}, "HLE": {3, 1},
		"CFO": {3, 2}, "T1": {3, 2}, "TES": {3, 2},
		"FNC": {0, 3}, "PSG": {0, 3},
	}
	for code, r := range want {
		if got := s.Records[code]; got != r {
			t.Errorf("%s record = %v, want %v", code, got, r)
		}
	}
	if len(s.Teams) != 16 {
		t.Errorf("swiss teams = %d, want 16", len(s.Teams))
	}
	if top := s.Teams[0]; top.Record != (Record{3, 0}) {
		t.Errorf("standings start with %v", top)
	}
	// Round 3 bo3 matches are the 2-0 and 0-2 pools.
	for _, m := range s.Rounds[2] {
		p := s.Pools[m.ID]
		if m.BestOf == 3 && p != (Record{2, 0}) && p != (Record{0, 2}) {
			t.Errorf("round 3 bo3 %s has pool %v", m.ID, p)
		}
		if m.BestOf == 1 && p != (Record{1, 1}) {
			t.Errorf("round 3 bo1 %s has pool %v", m.ID, p)
		}
	}

	k := tour.Knockout
	if len(k.Quarterfinals) != 4 || len(k.Semifinals) != 2 || k.Final == nil {
		t.Fatalf("knockout shape %d/%d/%v", len(k.Quarterfinals), len(k.Semifinals), k.Final != nil)
	}
	f := k.Final
	if w := f.Winner(); w < 0 || f.Teams[w].Code != "T1" {
		t.Errorf("final winner = %+v", f.Teams)
	}
	for i := 1; i < len(k.Quarterfinals); i++ {
		if k.Quarterfinals[i].Start.Before(k.Quarterfinals[i-1].Start) {
			t.Error("quarterfinals out of order")
		}
	}
}

func TestWorlds2026PlayIn(t *testing.T) {
	tour := Build(loadStages(t, "testdata/standings_2026.json"), loadEvents(t, scheduleFiles...))
	p := tour.PlayIn
	if len(p.All) != 6 {
		t.Fatalf("play-in matches = %d, want 6", len(p.All))
	}
	codes := func(m *Match) [2]string { return [2]string{m.Teams[0].Code, m.Teams[1].Code} }
	if got := codes(p.Openers[0]); got != [2]string{"KC", "C9"} {
		t.Errorf("opener 1 = %v", got)
	}
	if got := codes(p.Openers[1]); got != [2]string{"MVK", "FUR"} {
		t.Errorf("opener 2 = %v", got)
	}
	// The opener winners meet in the upper final, the losers in the lower round.
	if got := codes(p.UpperFinal); got != [2]string{"KC", "MVK"} {
		t.Errorf("upper final = %v", got)
	}
	if got := codes(p.LowerRound1); got != [2]string{"C9", "FUR"} {
		t.Errorf("lower round 1 = %v", got)
	}
	if p.Final == nil || p.Final.BestOf != 5 {
		t.Errorf("final = %+v", p.Final)
	}

	if n := len(tour.Swiss.Rounds); n != 5 {
		t.Errorf("swiss rounds = %d", n)
	}
	if b := tour.Knockout.Final; b == nil || b.Block != "Finals" {
		t.Errorf("knockout final = %+v", b)
	}
}
