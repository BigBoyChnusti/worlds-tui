package esports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL = "https://esports-api.lolesports.com/persisted/gw"
	// DefaultAPIKey is the public key shipped in the lolesports.com frontend.
	DefaultAPIKey = "0TvQnueqKa5mxJntVWt0w4LpLfEkrV1Ta8rQBb9Z"
	// WorldsLeagueID is the gateway id of the Worlds league.
	WorldsLeagueID = "98767975604431411"
)

type Client struct {
	HTTP    *http.Client
	BaseURL string
	APIKey  string
	Lang    string
}

func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		BaseURL: DefaultBaseURL,
		APIKey:  DefaultAPIKey,
		Lang:    "en-US",
	}
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("hl", c.Lang)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: decode: %w", path, err)
	}
	return nil
}

func (c *Client) GetLeagues(ctx context.Context) ([]League, error) {
	var r struct {
		Data struct {
			Leagues []League `json:"leagues"`
		} `json:"data"`
	}
	err := c.get(ctx, "getLeagues", nil, &r)
	return r.Data.Leagues, err
}

func (c *Client) GetTournamentsForLeague(ctx context.Context, leagueID string) ([]Tournament, error) {
	var r struct {
		Data struct {
			Leagues []struct {
				Tournaments []Tournament `json:"tournaments"`
			} `json:"leagues"`
		} `json:"data"`
	}
	if err := c.get(ctx, "getTournamentsForLeague", url.Values{"leagueId": {leagueID}}, &r); err != nil {
		return nil, err
	}
	var out []Tournament
	for _, l := range r.Data.Leagues {
		out = append(out, l.Tournaments...)
	}
	return out, nil
}

// CurrentTournament picks the tournament running at now, else the most
// recent one that has started, else the first listed.
func CurrentTournament(ts []Tournament, now time.Time) (Tournament, bool) {
	if len(ts) == 0 {
		return Tournament{}, false
	}
	var best *Tournament
	for i := range ts {
		t := &ts[i]
		if t.Contains(now) {
			return *t, true
		}
		if t.StartDate <= now.UTC().Format(time.DateOnly) && (best == nil || t.StartDate > best.StartDate) {
			best = t
		}
	}
	if best != nil {
		return *best, true
	}
	return ts[0], true
}

type schedulePage struct {
	Data struct {
		Schedule struct {
			Pages struct {
				Older string `json:"older"`
				Newer string `json:"newer"`
			} `json:"pages"`
			Events []Event `json:"events"`
		} `json:"schedule"`
	} `json:"data"`
}

func (c *Client) schedulePage(ctx context.Context, leagueID, token string) (*schedulePage, error) {
	params := url.Values{"leagueId": {leagueID}}
	if token != "" {
		params.Set("pageToken", token)
	}
	var p schedulePage
	err := c.get(ctx, "getSchedule", params, &p)
	return &p, err
}

// GetSchedule returns all events of the league starting at or after from,
// following the gateway's page tokens in both directions. Events are in
// chronological order.
func (c *Client) GetSchedule(ctx context.Context, leagueID string, from time.Time) ([]Event, error) {
	first, err := c.schedulePage(ctx, leagueID, "")
	if err != nil {
		return nil, err
	}
	events := first.Data.Schedule.Events

	// Walk back while the oldest event seen is still inside the window.
	older := first.Data.Schedule.Pages.Older
	for older != "" && len(events) > 0 && !events[0].StartTime.Before(from) {
		p, err := c.schedulePage(ctx, leagueID, older)
		if err != nil {
			return nil, err
		}
		events = append(p.Data.Schedule.Events, events...)
		older = p.Data.Schedule.Pages.Older
	}
	for newer := first.Data.Schedule.Pages.Newer; newer != ""; {
		p, err := c.schedulePage(ctx, leagueID, newer)
		if err != nil {
			return nil, err
		}
		events = append(events, p.Data.Schedule.Events...)
		newer = p.Data.Schedule.Pages.Newer
	}

	out := events[:0]
	seen := map[string]bool{}
	for _, e := range events {
		if e.StartTime.Before(from) || seen[e.Match.ID] {
			continue
		}
		seen[e.Match.ID] = true
		out = append(out, e)
	}
	return out, nil
}

// GetLive returns the events currently in progress across all leagues.
func (c *Client) GetLive(ctx context.Context) ([]Event, error) {
	var r struct {
		Data struct {
			Schedule struct {
				Events []Event `json:"events"`
			} `json:"schedule"`
		} `json:"data"`
	}
	err := c.get(ctx, "getLive", nil, &r)
	return r.Data.Schedule.Events, err
}

func (c *Client) GetEventDetails(ctx context.Context, matchID string) (*EventDetails, error) {
	var r struct {
		Data struct {
			Event EventDetails `json:"event"`
		} `json:"data"`
	}
	if err := c.get(ctx, "getEventDetails", url.Values{"id": {matchID}}, &r); err != nil {
		return nil, err
	}
	return &r.Data.Event, nil
}

func (c *Client) GetStandings(ctx context.Context, tournamentID string) ([]Stage, error) {
	var r struct {
		Data struct {
			Standings []struct {
				Stages []Stage `json:"stages"`
			} `json:"standings"`
		} `json:"data"`
	}
	if err := c.get(ctx, "getStandings", url.Values{"tournamentId": {tournamentID}}, &r); err != nil {
		return nil, err
	}
	var out []Stage
	for _, s := range r.Data.Standings {
		out = append(out, s.Stages...)
	}
	return out, nil
}
