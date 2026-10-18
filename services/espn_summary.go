package services

import (
	"App-Futebol/models"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Ex: "2026-27 UEFA Nations League" -> 2026 e 27
var espnSeasonRe = regexp.MustCompile(`(\d{4})-(\d{2})`)

// seasonFromESPN monta a temporada no padrão do banco: "2026" ou "2026-2027"
func seasonFromESPN(year int, name string) string {
	if m := espnSeasonRe.FindStringSubmatch(name); len(m) == 3 {
		return fmt.Sprintf("%s-20%s", m[1], m[2])
	}
	return strconv.Itoa(year)
}

// groupFromCompetitors pega o grupo do jogo (ex: "Group C1") a partir do grupo de
// cada time. O campo de grupo da própria competição no summary vem errado
// (sempre "LEAGUE A - GROUP 1"), por isso não é usado.
// Só aceita se os dois times estiverem no mesmo grupo.
func groupFromCompetitors(teamGroups []json.RawMessage) string {
	group := ""
	for _, raw := range teamGroups {
		var g struct {
			Name string `json:"name"`
		}
		if len(raw) == 0 || json.Unmarshal(raw, &g) != nil || !strings.HasPrefix(strings.ToLower(g.Name), "group") {
			return ""
		}
		if group != "" && g.Name != group {
			return ""
		}
		group = g.Name
	}
	return group
}

func FetchAndParseESPNMatch(matchID string, leagueCode string) (models.ESPNMatchDB, []models.ESPNLineupDB, []models.ESPNEventDB, error) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/summary?event=%s", espnLeague, matchID)

	resp, err := httpClient.Get(url)
	if err != nil {
		return models.ESPNMatchDB{}, nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.ESPNMatchDB{}, nil, nil, err
	}

	var data models.ESPNSummaryResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return models.ESPNMatchDB{}, nil, nil, fmt.Errorf("erro unmarshal: %v", err)
	}

	seasonStr := seasonFromESPN(data.Header.Season.Year, data.Header.Season.Name)
	stats := statsFromBoxscore(data)
	if len(data.Header.Competitions) > 0 && data.Header.Competitions[0].Status.Type.State == "pre" {
		// Antes do jogo a ESPN pode mandar tudo zerado: o app mostraria um 0 x 0 que
		// não aconteceu
		stats = nil
	}

	match := models.ESPNMatchDB{
		MatchID:    matchID,
		League:     leagueCode,
		Season:     seasonStr,
		Statistics: stats,
	}

	if len(data.Header.Competitions) > 0 {
		comp := data.Header.Competitions[0]
		match.MatchDate = comp.Date
		match.Status = comp.Status.Type.State

		match.Stage = normalizeESPNStage(data.Header.Season.Slug)

		var teamGroups []json.RawMessage
		for _, team := range comp.Competitors {
			teamGroups = append(teamGroups, team.Team.Groups)
		}
		match.GroupName = groupFromCompetitors(teamGroups)

		for _, team := range comp.Competitors {
			teamID, _ := strconv.ParseInt(team.Team.ID, 10, 64)
			teamName := team.Team.DisplayName
			teamLogo := ""
			if len(team.Team.Logos) > 0 {
				teamLogo = team.Team.Logos[0].Href
			}
			if isPlaceholderTeam(teamName) {
				teamID, teamName, teamLogo = 0, "", ""
			}

			if team.HomeAway == "home" {
				match.ESPNHomeTeamID = teamID
				match.HomeScore = team.Score
				match.HomeTeam = teamName
				match.HomeLogo = teamLogo
				match.HomePenalty = shootout(team.ShootoutScore)
			} else {
				match.ESPNAwayTeamID = teamID
				match.AwayScore = team.Score
				match.AwayTeam = teamName
				match.AwayLogo = teamLogo
				match.AwayPenalty = shootout(team.ShootoutScore)
			}
		}
	}

	var lineups []models.ESPNLineupDB
	for _, roster := range data.Rosters {
		teamID, _ := strconv.ParseInt(roster.Team.ID, 10, 64)

		for _, athlete := range roster.Roster {
			playerID, _ := strconv.ParseInt(athlete.Athlete.ID, 10, 64)

			lineups = append(lineups, models.ESPNLineupDB{
				MatchID:      matchID,
				ESPNTeamID:   teamID,
				ESPNPlayerID: playerID,
				PlayerName:   athlete.Athlete.DisplayName,
				Jersey:       athlete.Jersey,
				Position:     athlete.Position.Abbreviation,
				IsStarter:    athlete.Starter,
				Formation:    roster.Formation,
			})
		}
	}
	var events []models.ESPNEventDB
	for _, evt := range data.KeyEvents {
		teamID, _ := strconv.ParseInt(evt.Team.ID, 10, 64)

		pName := ""
		if len(evt.Participants) > 0 {
			pName = evt.Participants[0].Athlete.DisplayName
		}
		// Substituição: a ESPN manda [quem entrou, quem saiu]; quem saiu vai em details
		details := ""
		if strings.Contains(evt.Type.Text, "Substitution") && len(evt.Participants) > 1 {
			details = evt.Participants[1].Athlete.DisplayName
		}

		events = append(events, models.ESPNEventDB{
			MatchID:    matchID,
			Minute:     evt.Clock.DisplayValue,
			EventType:  evt.Type.Text,
			ESPNTeamID: teamID,
			PlayerName: pName,
			Details:    details,
		})
	}

	return match, lineups, events, nil
}

// statsFromBoxscore lê as estatísticas de cada time (a ESPN manda o número em texto).
// nil quando ela ainda não tem (antes do jogo) ou só tem de um time.
func statsFromBoxscore(data models.ESPNSummaryResponse) *models.MatchStatistics {
	stats := &models.MatchStatistics{Home: map[string]float64{}, Away: map[string]float64{}}
	for _, team := range data.Boxscore.Teams {
		target := stats.Away
		if team.HomeAway == "home" {
			target = stats.Home
		}
		for _, st := range team.Statistics {
			if v, err := strconv.ParseFloat(st.DisplayValue, 64); err == nil {
				target[st.Name] = v
			}
		}
	}
	if len(stats.Home) == 0 || len(stats.Away) == 0 {
		return nil
	}
	return stats
}
