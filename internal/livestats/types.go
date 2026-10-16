// Package livestats reads the lolesports live stats feed
// (feed.lolesports.com/livestats/v1), which serves in-game state in
// 10-second windows of frames.
package livestats

import "time"

// Frame game states.
const (
	GameInGame   = "in_game"
	GamePaused   = "paused"
	GameFinished = "finished"
)

type ParticipantMeta struct {
	ParticipantID   int    `json:"participantId"`
	EsportsPlayerID string `json:"esportsPlayerId"`
	SummonerName    string `json:"summonerName"`
	ChampionID      string `json:"championId"`
	Role            string `json:"role"`
}

type TeamMeta struct {
	EsportsTeamID       string            `json:"esportsTeamId"`
	ParticipantMetadata []ParticipantMeta `json:"participantMetadata"`
}

type GameMetadata struct {
	PatchVersion     string   `json:"patchVersion"`
	BlueTeamMetadata TeamMeta `json:"blueTeamMetadata"`
	RedTeamMetadata  TeamMeta `json:"redTeamMetadata"`
}

type WindowParticipant struct {
	ParticipantID int `json:"participantId"`
	TotalGold     int `json:"totalGold"`
	Level         int `json:"level"`
	Kills         int `json:"kills"`
	Deaths        int `json:"deaths"`
	Assists       int `json:"assists"`
	CreepScore    int `json:"creepScore"`
	CurrentHealth int `json:"currentHealth"`
	MaxHealth     int `json:"maxHealth"`
}

type TeamFrame struct {
	TotalGold    int                 `json:"totalGold"`
	Inhibitors   int                 `json:"inhibitors"`
	Towers       int                 `json:"towers"`
	Barons       int                 `json:"barons"`
	TotalKills   int                 `json:"totalKills"`
	Dragons      []string            `json:"dragons"`
	Participants []WindowParticipant `json:"participants"`
}

type WindowFrame struct {
	Timestamp time.Time `json:"rfc460Timestamp"`
	GameState string    `json:"gameState"`
	BlueTeam  TeamFrame `json:"blueTeam"`
	RedTeam   TeamFrame `json:"redTeam"`
}

type Window struct {
	EsportsGameID  string        `json:"esportsGameId"`
	EsportsMatchID string        `json:"esportsMatchId"`
	GameMetadata   GameMetadata  `json:"gameMetadata"`
	Frames         []WindowFrame `json:"frames"`
}

type PerkMetadata struct {
	StyleID    int   `json:"styleId"`
	SubStyleID int   `json:"subStyleId"`
	Perks      []int `json:"perks"`
}

type DetailsParticipant struct {
	ParticipantID       int          `json:"participantId"`
	Level               int          `json:"level"`
	Kills               int          `json:"kills"`
	Deaths              int          `json:"deaths"`
	Assists             int          `json:"assists"`
	TotalGoldEarned     int          `json:"totalGoldEarned"`
	CreepScore          int          `json:"creepScore"`
	KillParticipation   float64      `json:"killParticipation"`
	ChampionDamageShare float64      `json:"championDamageShare"`
	WardsPlaced         int          `json:"wardsPlaced"`
	WardsDestroyed      int          `json:"wardsDestroyed"`
	Items               []int        `json:"items"`
	PerkMetadata        PerkMetadata `json:"perkMetadata"`
}

type DetailsFrame struct {
	Timestamp    time.Time            `json:"rfc460Timestamp"`
	Participants []DetailsParticipant `json:"participants"`
}

type Details struct {
	Frames []DetailsFrame `json:"frames"`
}
