package esports

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// scheduleServer serves the saved getSchedule pages by page token and
// fails any other request, so tests catch unexpected paging.
func scheduleServer(t *testing.T) *httptest.Server {
	t.Helper()
	pages := map[string]string{
		"":                                     "testdata/schedule_page.json",
		"b2xkZXI6OjExMzQ3NTc5ODAwNjY2NDU3NA==": "testdata/schedule_older.json",
		"bmV3ZXI6OjExNTY2MDU0MDcyNjA5NTEwOQ==": "testdata/schedule_newer.json",
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != DefaultAPIKey {
			http.Error(w, "missing key", http.StatusForbidden)
			return
		}
		if r.URL.Path != "/getSchedule" || r.URL.Query().Get("leagueId") != WorldsLeagueID {
			http.NotFound(w, r)
			return
		}
		file, ok := pages[r.URL.Query().Get("pageToken")]
		if !ok {
			t.Errorf("unexpected page token %q", r.URL.Query().Get("pageToken"))
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(b)
	}))
}

func TestGetSchedulePagesBothWays(t *testing.T) {
	srv := scheduleServer(t)
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL

	from := time.Date(2025, 10, 14, 0, 0, 0, 0, time.UTC)
	events, err := c.GetSchedule(t.Context(), WorldsLeagueID, from)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no events")
	}
	if events[0].StartTime.Before(from) {
		t.Errorf("first event %v is before %v", events[0].StartTime, from)
	}
	// The 2025 Play-In match lives on the older page.
	if events[0].BlockName == "Swiss" {
		t.Errorf("older page not followed, first event is %s %v", events[0].BlockName, events[0].StartTime)
	}
	// The 2026 final lives on the newer page.
	last := events[len(events)-1]
	if last.BlockName != "Finals" || last.StartTime.Year() != 2026 {
		t.Errorf("newer page not followed, last event is %s %v", last.BlockName, last.StartTime)
	}
	for i := 1; i < len(events); i++ {
		if events[i].StartTime.Before(events[i-1].StartTime) {
			t.Fatalf("events out of order at %d", i)
		}
	}
	seen := map[string]bool{}
	for _, e := range events {
		if seen[e.Match.ID] {
			t.Fatalf("duplicate match %s", e.Match.ID)
		}
		seen[e.Match.ID] = true
	}
}

func TestCurrentTournament(t *testing.T) {
	ts := []Tournament{
		{Slug: "worlds_2026", StartDate: "2026-10-15", EndDate: "2026-11-15"},
		{Slug: "worlds_2025", StartDate: "2025-10-14", EndDate: "2025-11-09"},
	}
	cases := []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), "worlds_2026"},
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "worlds_2025"},
		{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "worlds_2026"},
	}
	for _, c := range cases {
		got, ok := CurrentTournament(ts, c.now)
		if !ok || got.Slug != c.want {
			t.Errorf("CurrentTournament(%v) = %q, want %q", c.now, got.Slug, c.want)
		}
	}
}
