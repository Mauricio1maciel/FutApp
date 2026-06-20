package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var ESPNLeagueMap = map[string]string{
	"BSA": "bra.1",
	"PL":  "eng.1",
	"PD":  "esp.1",
	"SA":  "ita.1",
	"CL":  "uefa.champions",
	"BL1": "ger.1",
	"FL1": "fra.1",
	"CLI": "conmebol.libertadores",
	"CSU": "conmebol.sudamericana",
	"WC":  "fifa.world",
}

func GetLiveScoreboard(leagueCode string, date string) ([]models.AppLiveMatch, error) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	if date != "" {
		url = fmt.Sprintf("%s?dates=%s", url, date)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao acessar ESPN: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data models.ESPNScoreboard
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("erro unmarshal scoreboard: %v", err)
	}

	var leagueName, leagueLogo string
	if len(data.Leagues) > 0 {
		leagueName = data.Leagues[0].Name
		if len(data.Leagues[0].Logos) > 0 {
			leagueLogo = data.Leagues[0].Logos[0].Href
		}
	}

	var liveMatches []models.AppLiveMatch

	for _, event := range data.Events {
		if len(event.Competitions) == 0 {
			continue
		}

		comp := event.Competitions[0]
		match := models.AppLiveMatch{
			MatchID:    event.ID,
			LeagueName: leagueName,
			LeagueLogo: leagueLogo,
			MatchDate:  event.Date,
			State:      comp.Status.Type.State,
			Clock:      comp.Status.DisplayClock,
		}

		if match.Clock == "" || match.Clock == "0'" {
			match.Clock = comp.Status.Type.State
		}

		for _, competitor := range comp.Competitors {
			if competitor.HomeAway == "home" {
				match.HomeTeam = competitor.Team.DisplayName
				match.HomeLogo = competitor.Team.Logo
				match.HomeScore = competitor.Score
				match.ESPNHomeTeamID = competitor.Team.ID
			} else {
				match.AwayTeam = competitor.Team.DisplayName
				match.AwayLogo = competitor.Team.Logo
				match.AwayScore = competitor.Score
				match.ESPNAwayTeamID = competitor.Team.ID
			}
		}

		liveMatches = append(liveMatches, match)
	}

	return liveMatches, nil
}

func UpdateMatchFromESPN(home, away, dateStr, leagueCode string) error {
	if len(dateStr) < 10 {
		return fmt.Errorf("data inválida: %s", dateStr)
	}
	espnDate := dateStr[0:4] + dateStr[5:7] + dateStr[8:10]

	matches, err := GetLiveScoreboard(leagueCode, espnDate)
	if err != nil {
		return err
	}

	var foundID string
	for _, m := range matches {
		if utils.CompareTeams(m.HomeTeam, home) && utils.CompareTeams(m.AwayTeam, away) {
			foundID = m.MatchID
			break
		}
	}

	if foundID == "" {
		return fmt.Errorf("jogo não encontrado na ESPN para data %s", espnDate)
	}

	matchDB, lineups, events, err := FetchAndParseESPNMatch(foundID, leagueCode)
	if err != nil {
		return err
	}

	return database.SaveFullMatchHistory(matchDB, lineups, events)
}

func getESPNLeague(leagueCode string) string {
	if espnCode, exists := ESPNLeagueMap[leagueCode]; exists {
		return espnCode
	}
	return leagueCode
}
