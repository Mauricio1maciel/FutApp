package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
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
	"CDB": "bra.copa_do_brazil",
	"BSB": "bra.2",
	"WC":  "fifa.world",
	"UNL": "uefa.nations",
}

func GetLiveScoreboard(leagueCode string, date string) ([]models.AppLiveMatch, error) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	if date != "" {
		url = fmt.Sprintf("%s?dates=%s", url, date)
	}

	resp, err := httpClient.Get(url)
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
	return liveMatchesFromScoreboard(data, time.Now()), nil
}

// liveMatchesFromScoreboard monta os jogos do placar da ESPN lido em fetchedAt
func liveMatchesFromScoreboard(data models.ESPNScoreboard, fetchedAt time.Time) []models.AppLiveMatch {
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
			MatchID:      event.ID,
			IDEvent:      event.ID,
			LeagueName:   leagueName,
			LeagueLogo:   leagueLogo,
			MatchDate:    event.Date,
			State:        comp.Status.Type.State,
			Clock:        comp.Status.DisplayClock,
			ClockSeconds: int(comp.Status.Clock),
			Period:       comp.Status.Period,
			ClockRunning: clockRunning(comp.Status),
			ClockAt:      fetchedAt.UTC().Format(time.RFC3339),
		}

		if match.State == "in" && match.ClockSeconds == 0 && match.Period >= 1 && match.Period <= 2 {
			if secs, ok := estimateClock(event.Date, match.Period, fetchedAt); ok {
				match.ClockSeconds, match.ClockEstimated = secs, true
				match.Clock = fmt.Sprintf("%d'", secs/60)
			}
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
				match.HomePenalty = shootout(competitor.ShootoutScore)
			} else {
				match.AwayTeam = competitor.Team.DisplayName
				match.AwayLogo = competitor.Team.Logo
				match.AwayScore = competitor.Score
				match.ESPNAwayTeamID = competitor.Team.ID
				match.AwayPenalty = shootout(competitor.ShootoutScore)
			}
		}

		liveMatches = append(liveMatches, match)
	}

	return liveMatches
}

// estimateClock estima os segundos de jogo pelo horário de início, quando a ESPN manda o
// jogo rolando com o relógio zerado: no 2º tempo, desconta 15 minutos de intervalo. O
// resultado fica dentro do tempo (até 45+5 no 1º tempo; de 45 a 90+10 no 2º)
func estimateClock(kickoff string, period int, now time.Time) (int, bool) {
	var start time.Time
	var err error
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z"} {
		if start, err = time.Parse(layout, kickoff); err == nil {
			break
		}
	}
	if err != nil || now.Before(start) {
		return 0, false
	}
	secs := int(now.Sub(start).Seconds())
	if period == 1 {
		return min(secs, 50*60), true
	}
	return min(max(secs-15*60, 45*60), 100*60), true
}

// clockRunning: o relógio só anda com a bola rolando. No intervalo, entre o tempo normal e
// a prorrogação e nos pênaltis (período 5), ele fica parado
func clockRunning(st models.ESPNStatus) bool {
	if st.Type.State != "in" || st.Period >= 5 {
		return false
	}
	switch st.Type.Name {
	case "STATUS_HALFTIME", "STATUS_HALFTIME_ET", "STATUS_END_PERIOD", "STATUS_END_OF_REGULATION",
		"STATUS_END_OF_EXTRATIME", "STATUS_SHOOTOUT", "STATUS_DELAYED", "STATUS_SUSPENDED":
		return false
	}
	return true
}

func UpdateMatchFromESPN(ctx context.Context, home, away, dateStr, leagueCode string) error {
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

	return database.SaveFullMatchHistory(ctx, matchDB, lineups, events)
}

func getESPNLeague(leagueCode string) string {
	if espnCode, exists := ESPNLeagueMap[leagueCode]; exists {
		return espnCode
	}
	return leagueCode
}
