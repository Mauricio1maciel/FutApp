// package services

// import (
// 	"App-Futebol/models"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"strconv"
// )

// func FetchAndParseESPNMatch(matchID string, leagueCode string) (models.ESPNMatchDB, []models.ESPNLineupDB, []models.ESPNEventDB, error) {
// 	espnLeague := getESPNLeague(leagueCode)
// 	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/summary?event=%s", espnLeague, matchID)

// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return models.ESPNMatchDB{}, nil, nil, err
// 	}
// 	defer resp.Body.Close()

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return models.ESPNMatchDB{}, nil, nil, err
// 	}

// 	var data models.ESPNSummaryResponse
// 	if err := json.Unmarshal(body, &data); err != nil {
// 		return models.ESPNMatchDB{}, nil, nil, fmt.Errorf("erro unmarshal: %v", err)
// 	}

// 	match := models.ESPNMatchDB{
// 		MatchID: matchID,
// 		League:  leagueCode,
// 	}

// 	if len(data.Header.Competitions) > 0 {
// 		comp := data.Header.Competitions[0]
// 		match.MatchDate = comp.Date
// 		match.Status = comp.Status.Type.State

// 		for _, team := range comp.Competitors {
// 			teamID, _ := strconv.ParseInt(team.Team.ID, 10, 64)

// 			if team.HomeAway == "home" {
// 				match.ESPNHomeTeamID = teamID
// 				match.HomeScore = team.Score
// 			} else {
// 				match.ESPNAwayTeamID = teamID
// 				match.AwayScore = team.Score
// 			}
// 		}
// 	}

// 	var lineups []models.ESPNLineupDB
// 	for _, roster := range data.Rosters {
// 		teamID, _ := strconv.ParseInt(roster.Team.ID, 10, 64)

// 		for _, athlete := range roster.Roster {

// 			playerID, _ := strconv.ParseInt(athlete.Athlete.ID, 10, 64)

// 			lineups = append(lineups, models.ESPNLineupDB{
// 				MatchID:      matchID,
// 				ESPNTeamID:   teamID,
// 				ESPNPlayerID: playerID,
// 				PlayerName:   athlete.Athlete.DisplayName,
// 				Jersey:       athlete.Jersey,
// 				Position:     athlete.Position.Abbreviation,
// 				IsStarter:    athlete.Starter,
// 				Formation:    roster.Formation,
// 			})
// 		}
// 	}
// 	var events []models.ESPNEventDB
// 	for _, evt := range data.KeyEvents {
// 		teamID, _ := strconv.ParseInt(evt.Team.ID, 10, 64)

// 		pName := ""
// 		if len(evt.Participants) > 0 {
// 			pName = evt.Participants[0].Athlete.DisplayName
// 		}

// 		events = append(events, models.ESPNEventDB{
// 			MatchID:    matchID,
// 			Minute:     evt.Clock.DisplayValue,
// 			EventType:  evt.Type.Text,
// 			ESPNTeamID: teamID,
// 			PlayerName: pName,
// 		})
// 	}

//		return match, lineups, events, nil
//	}
package services

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func FetchAndParseESPNMatch(matchID string, leagueCode string) (models.ESPNMatchDB, []models.ESPNLineupDB, []models.ESPNEventDB, error) {
	espnLeague := getESPNLeague(leagueCode)
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/summary?event=%s", espnLeague, matchID)

	resp, err := http.Get(url)
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

	match := models.ESPNMatchDB{
		MatchID: matchID,
		League:  leagueCode,
	}

	if len(data.Header.Competitions) > 0 {
		comp := data.Header.Competitions[0]
		match.MatchDate = comp.Date
		match.Status = comp.Status.Type.State

		for _, team := range comp.Competitors {
			teamID, _ := strconv.ParseInt(team.Team.ID, 10, 64)

			if team.HomeAway == "home" {
				match.ESPNHomeTeamID = teamID
				match.HomeScore = team.Score
			} else {
				match.ESPNAwayTeamID = teamID
				match.AwayScore = team.Score
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

		events = append(events, models.ESPNEventDB{
			MatchID:    matchID,
			Minute:     evt.Clock.DisplayValue,
			EventType:  evt.Type.Text,
			ESPNTeamID: teamID,
			PlayerName: pName,
		})
	}

	return match, lineups, events, nil
}

// 🔥 FUNÇÃO AJUSTADA COM O SEU TIPO "ESPNMatchDB" E SUA FUNÇÃO "getESPNLeague"
func UpdateLiveMatchClock(match *models.ESPNMatchDB) {
	if match == nil || match.MatchID == "" {
		return
	}

	// Usando a sua função que já sabe traduzir WC para fifa.world
	espnLeague := getESPNLeague(match.League)
	scoreboardURL := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/scoreboard", espnLeague)

	resp, err := http.Get(scoreboardURL)
	if err != nil {
		utils.CustomLog("ESPN_CLOCK", "Erro ao buscar scoreboard para o jogo %s: %v", match.MatchID, err)
		return
	}
	defer resp.Body.Close()

	// Estrutura anônima para pegar apenas o que importa (performance!)
	var scoreboardData struct {
		Events []struct {
			ID     string `json:"id"`
			Status struct {
				DisplayClock string `json:"displayClock"`
				Type         struct {
					State string `json:"state"`
				} `json:"type"`
			} `json:"status"`
		} `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scoreboardData); err == nil {
		for _, event := range scoreboardData.Events {
			if event.ID == match.MatchID {
				// Atualiza o relógio!
				match.Clock = event.Status.DisplayClock
				// Atualiza o status também, caso a partida tenha acabado nesse meio tempo
				match.Status = event.Status.Type.State
				utils.CustomLog("ESPN_CLOCK", "⏰ Relógio atualizado para %s: %s", match.MatchID, match.Clock)
				break
			}
		}
	}
}
