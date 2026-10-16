// Package esports is a client for the undocumented lolesports.com gateway
// (esports-api.lolesports.com/persisted/gw) that powers the official site.
package esports

import "time"

// Match states as reported by the gateway.
const (
	StateUnstarted  = "unstarted"
	StateInProgress = "inProgress"
	StateCompleted  = "completed"
)

type League struct {
	ID     string `json:"id"`
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Region string `json:"region"`
	Image  string `json:"image"`
}

type Tournament struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// Contains reports whether t falls between the tournament's start and end
// dates (inclusive, compared as UTC calendar days).
func (t Tournament) Contains(at time.Time) bool {
	day := at.UTC().Format(time.DateOnly)
	return day >= t.StartDate && day <= t.EndDate
}

type Result struct {
	Outcome  string `json:"outcome"`
	GameWins int    `json:"gameWins"`
}

type Record struct {
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
}

type Team struct {
	ID     string  `json:"id"`
	Slug   string  `json:"slug"`
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	Image  string  `json:"image"`
	Result *Result `json:"result"`
	Record *Record `json:"record"`
}

// Known reports whether the slot holds a real team rather than a TBD
// placeholder.
func (t Team) Known() bool {
	return t.Code != "" && t.Code != "TBD"
}

type Strategy struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type GameTeam struct {
	ID   string `json:"id"`
	Side string `json:"side"`
}

type Game struct {
	ID     string     `json:"id"`
	Number int        `json:"number"`
	State  string     `json:"state"`
	Teams  []GameTeam `json:"teams"`
}

type Match struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	Flags    []string `json:"flags"`
	Teams    []Team   `json:"teams"`
	Strategy Strategy `json:"strategy"`
	Games    []Game   `json:"games"`
}

// Event is one entry of getSchedule or getLive.
type Event struct {
	ID         string    `json:"id"`
	StartTime  time.Time `json:"startTime"`
	State      string    `json:"state"`
	Type       string    `json:"type"`
	BlockName  string    `json:"blockName"`
	League     League    `json:"league"`
	Tournament struct {
		ID string `json:"id"`
	} `json:"tournament"`
	Match Match `json:"match"`
}

// EventDetails is the getEventDetails payload: a match with its games.
type EventDetails struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Tournament struct {
		ID string `json:"id"`
	} `json:"tournament"`
	League League `json:"league"`
	Match  Match  `json:"match"`
}

type StandingsMatch struct {
	ID               string   `json:"id"`
	State            string   `json:"state"`
	PreviousMatchIDs []string `json:"previousMatchIds"`
	Flags            []string `json:"flags"`
	Teams            []Team   `json:"teams"`
}

type Section struct {
	Name     string           `json:"name"`
	Matches  []StandingsMatch `json:"matches"`
	Rankings []struct {
		Ordinal int    `json:"ordinal"`
		Teams   []Team `json:"teams"`
	} `json:"rankings"`
}

type Stage struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Slug     string    `json:"slug"`
	Sections []Section `json:"sections"`
}

// Matches returns every match of the stage across all its sections.
func (s Stage) Matches() []StandingsMatch {
	var out []StandingsMatch
	for _, sec := range s.Sections {
		out = append(out, sec.Matches...)
	}
	return out
}
