package livestats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const DefaultBaseURL = "https://feed.lolesports.com/livestats/v1"

// FeedDelay is how far behind real time requests are made. The feed has no
// frames for the most recent stretch of a live game.
const FeedDelay = 60 * time.Second

// ErrNoData means the feed has no frames for the requested game or time yet.
var ErrNoData = errors.New("livestats: no data")

type Client struct {
	HTTP    *http.Client
	BaseURL string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, BaseURL: DefaultBaseURL}
}

// StartingTime formats t the way the feed requires: UTC, floored to a
// multiple of 10 seconds. Unaligned values are rejected with HTTP 400.
func StartingTime(t time.Time) string {
	return t.UTC().Truncate(10 * time.Second).Format("2006-01-02T15:04:05.000Z")
}

// LiveStartingTime is the newest starting time the feed can serve at now.
func LiveStartingTime(now time.Time) time.Time {
	return now.Add(-FeedDelay).UTC().Truncate(10 * time.Second)
}

func (c *Client) get(ctx context.Context, kind, gameID string, at time.Time, out any) error {
	u := fmt.Sprintf("%s/%s/%s", c.BaseURL, kind, url.PathEscape(gameID))
	if !at.IsZero() {
		u += "?startingTime=" + url.QueryEscape(StartingTime(at))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("livestats %s: %w", kind, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound:
		return ErrNoData
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("livestats %s: HTTP %d: %s", kind, resp.StatusCode, body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("livestats %s: %w", kind, err)
	}
	if len(body) == 0 {
		return ErrNoData
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("livestats %s: decode: %w", kind, err)
	}
	return nil
}

// Window returns the team-level frames starting at at. A zero at returns the
// first frames of the game; a time after the game ended returns its last
// frames.
func (c *Client) Window(ctx context.Context, gameID string, at time.Time) (*Window, error) {
	var w Window
	if err := c.get(ctx, "window", gameID, at, &w); err != nil {
		return nil, err
	}
	if len(w.Frames) == 0 {
		return nil, ErrNoData
	}
	return &w, nil
}

// Details returns per-player frames (items, runes, damage share) starting
// at at, with the same time semantics as Window.
func (c *Client) Details(ctx context.Context, gameID string, at time.Time) (*Details, error) {
	var d Details
	if err := c.get(ctx, "details", gameID, at, &d); err != nil {
		return nil, err
	}
	if len(d.Frames) == 0 {
		return nil, ErrNoData
	}
	return &d, nil
}
