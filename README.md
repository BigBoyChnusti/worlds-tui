# worlds-tui

A terminal app for following the League of Legends World Championship: live
game stats and a viewer for the Play-In, Swiss Stage and Knockouts.

Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Install and run

```sh
go install github.com/BigBoyChnusti/worlds-tui/cmd/worlds-tui@latest
worlds-tui
```

Or from a checkout: `go run ./cmd/worlds-tui`.

| Flag | Default | |
|---|---|---|
| `-tournament` | current | Tournament slug, e.g. `worlds_2025` |
| `-game` | | Open a game id on start, e.g. to replay a finished game |
| `-refresh` | `10s` | Live game poll interval |
| `-data-refresh` | `1m` | Standings and schedule refresh interval |
| `-snapshot` | | Print every view once and exit (`-width` sets the width) |

Try a finished game: `worlds-tui -tournament worlds_2025 -game 113475798006664579`.

## Views and keys

| Key | |
|---|---|
| `1`–`5`, `tab` | Live · Play-In · Swiss · Knockouts · Schedule |
| `↑`/`↓`, `j`/`k` | Select a match (bracket tabs) or scroll (Live) |
| `enter` | Open the selected match on the Live tab |
| `←`/`→`, `[`/`]` | Previous/next game of the series on the Live tab |
| `n` | Next live match |
| `f` | Follow the live match again after opening another one |
| `r` | Refresh now |
| `q` | Quit |

- **Live**: series score, game clock, team kills, gold, towers,
  inhibitors, barons and dragons, a gold-lead graph, and a 5v5 table with
  K/D/A, CS, gold, lane gold difference, level, kill participation, damage
  share and item builds. It follows the first live Worlds match on its own;
  between matches it shows a countdown to the next one.
- **Play-In**: the four-team double-elimination bracket.
- **Swiss**: all five rounds grouped by the record teams carried into each
  match, plus standings with advanced and eliminated teams.
- **Knockouts**: quarterfinals, semifinals and final.
- **Schedule**: every match by day, in local time.

## Data sources

There is no official public API for pro games; the Riot developer API only
covers ranked and normal games. The app uses the same undocumented
endpoints as [lolesports.com](https://lolesports.com):

| Source | Used for |
|---|---|
| `esports-api.lolesports.com/persisted/gw` (`getLive`, `getSchedule`, `getStandings`, `getEventDetails`, `getTournamentsForLeague`) with the public `x-api-key` from the site | Live matches, schedule, stages and results, games of a match |
| `feed.lolesports.com/livestats/v1/window/{gameId}` | Team gold, kills, towers, inhibitors, barons and dragons, and per-player K/D/A, CS, gold and level, in 10-second windows of frames |
| `feed.lolesports.com/livestats/v1/details/{gameId}` | Items, runes, kill participation, damage share, wards |
| [Data Dragon](https://developer.riotgames.com/docs/lol#data-dragon) | Item and champion names, cached in the user cache directory |

Things the feed doesn't provide and the app derives:

- **Game clock**: time since the game's first frame, minus pauses seen
  between consecutive frames. Pauses that fall between sampled frames
  aren't seen, so it can run slightly ahead of the in-game timer.
- **Gold leads**: blue minus red team gold, and per lane by role.
- **Bracket structure**: the gateway's `previousMatchIds` are empty, so
  stages are laid out from schedule order and block names. Swiss records
  are computed from completed matches.

Caveats:

- The endpoints are undocumented and can change without notice.
- `startingTime` must be a multiple of 10 seconds, and the feed has no
  frames for roughly the last minute, so live stats run about a minute
  behind real time.
- The gateway's per-game `side` field has been seen swapped; team sides
  come from the feed's `esportsTeamId` instead.

## Development

```sh
go test ./...
go vet ./...
```

Tests run against API responses saved in `internal/*/testdata` (Worlds 2025
and 2026), so they don't need the network.
