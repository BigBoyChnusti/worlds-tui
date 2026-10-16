package livestats

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func load[T any](t *testing.T, file string) *T {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return &v
}

func TestStartingTime(t *testing.T) {
	at := time.Date(2025, 10, 15, 9, 50, 7, 123e6, time.FixedZone("CEST", 2*3600))
	if got, want := StartingTime(at), "2025-10-15T07:50:00.000Z"; got != want {
		t.Errorf("StartingTime = %s, want %s", got, want)
	}
	now := time.Date(2026, 10, 16, 12, 0, 59, 0, time.UTC)
	if got, want := LiveStartingTime(now), time.Date(2026, 10, 16, 11, 59, 50, 0, time.UTC); !got.Equal(want) {
		t.Errorf("LiveStartingTime = %v, want %v", got, want)
	}
}

// Worlds 2025 Swiss, T1 vs FlyQuest: FlyQuest on blue, T1 won.
func TestGameFromFixtures(t *testing.T) {
	g := NewGame("113475798006664579")
	g.SetStart(load[Window](t, "testdata/window_start.json"))
	g.ApplyWindow(load[Window](t, "testdata/window_end.json"))
	g.ApplyWindow(load[Window](t, "testdata/window_mid.json")) // older data must not replace the latest frame
	g.ApplyDetails(load[Details](t, "testdata/details_end.json"))

	if g.State() != GameFinished {
		t.Errorf("state = %q, want finished", g.State())
	}
	if c := g.Clock(); c < 34*time.Minute || c > 35*time.Minute {
		t.Errorf("clock = %v, want about 34:30", c)
	}
	if g.Meta.BlueTeamMetadata.ParticipantMetadata[0].SummonerName != "FLY Bwipo" {
		t.Errorf("unexpected blue top %q", g.Meta.BlueTeamMetadata.ParticipantMetadata[0].SummonerName)
	}
	bt, rt := g.Latest.BlueTeam, g.Latest.RedTeam
	if d := g.GoldDiff(); d != bt.TotalGold-rt.TotalGold || d >= 0 {
		t.Errorf("gold diff = %d, want red (T1) ahead", d)
	}
	if rt.Barons != 2 || len(rt.Dragons) != 3 {
		t.Errorf("red objectives: barons %d dragons %v", rt.Barons, rt.Dragons)
	}

	blue := g.Players("blue")
	if len(blue) != 5 || blue[0].Meta.Role != "top" || blue[0].Details == nil || len(blue[0].Details.Items) == 0 {
		t.Fatalf("blue players not joined with details: %+v", blue)
	}
	red := g.Players("red")
	if got, want := g.LaneDiff("top"), blue[0].Stats.TotalGold-red[0].Stats.TotalGold; got != want {
		t.Errorf("top lane diff = %d, want %d", got, want)
	}

	hist := g.History(time.Minute)
	if len(hist) < 3 {
		t.Fatalf("history has %d points", len(hist))
	}
	for i := 1; i < len(hist); i++ {
		if hist[i].Clock <= hist[i-1].Clock {
			t.Fatalf("history not increasing at %d: %v", i, hist)
		}
	}
	if last := hist[len(hist)-1]; last.Diff != g.GoldDiff() {
		t.Errorf("last history diff %d, want %d", last.Diff, g.GoldDiff())
	}
}

func frame(at time.Time, state string, blue, red int) WindowFrame {
	return WindowFrame{Timestamp: at, GameState: state, BlueTeam: TeamFrame{TotalGold: blue}, RedTeam: TeamFrame{TotalGold: red}}
}

func TestClockSkipsPauses(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 17, 0, 0, 0, time.UTC)
	w := &Window{Frames: []WindowFrame{frame(t0, GameInGame, 2500, 2500)}}
	g := NewGame("x")
	g.SetStart(w)

	// Ten minutes of play, a two-minute pause seen in 10s steps, then a
	// minute more.
	var frames []WindowFrame
	at := t0
	for ; at.Before(t0.Add(10 * time.Minute)); at = at.Add(10 * time.Second) {
		frames = append(frames, frame(at, GameInGame, 3000, 2800))
	}
	pauseEnd := at.Add(2 * time.Minute)
	for ; at.Before(pauseEnd); at = at.Add(10 * time.Second) {
		frames = append(frames, frame(at, GamePaused, 3000, 2800))
	}
	end := at.Add(time.Minute)
	for ; !at.After(end); at = at.Add(10 * time.Second) {
		frames = append(frames, frame(at, GameInGame, 3100, 2800))
	}
	g.ApplyWindow(&Window{Frames: frames})

	if got, want := g.Clock(), 11*time.Minute; got != want {
		t.Errorf("clock = %v, want %v", got, want)
	}

	// A sampled gap with a paused frame before it isn't counted as paused.
	g2 := NewGame("y")
	g2.SetStart(w)
	g2.ApplyWindow(&Window{Frames: []WindowFrame{
		frame(t0.Add(5*time.Minute), GamePaused, 0, 0),
		frame(t0.Add(6*time.Minute), GameInGame, 0, 0),
	}})
	if got, want := g2.Clock(), 6*time.Minute; got != want {
		t.Errorf("clock across sampled gap = %v, want %v", got, want)
	}
}
