package services

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

	// 🔥 EXTRAÇÃO INTELIGENTE DA TEMPORADA DIRETAMENTE DO SEU MODELO
	seasonStr := strconv.Itoa(data.Header.Season.Year) // Padrão seguro (ex: "2026")

	// Procura o padrão "4 dígitos - 2 dígitos" (Ex: "2026-27 UEFA Nations...")
	re := regexp.MustCompile(`(\d{4})-(\d{2})`)
	matchesSeason := re.FindStringSubmatch(data.Header.Season.Name)
	if len(matchesSeason) == 3 {
		// Transforma "2026-27" em "2026-2027"
		seasonStr = fmt.Sprintf("%s-20%s", matchesSeason[1], matchesSeason[2])
	}

	match := models.ESPNMatchDB{
		MatchID: matchID,
		League:  leagueCode,
		Season:  seasonStr, // 🔥 A TEMPORADA VEM PARA AQUI!
	}

	if len(data.Header.Competitions) > 0 {
		comp := data.Header.Competitions[0]
		match.MatchDate = comp.Date
		match.Status = comp.Status.Type.State

		// 🔥 A MÁGICA ENTRA AQUI: Capturamos a Fase e o Grupo!
		match.Stage = data.Header.Season.Slug
		match.GroupName = comp.Group.Name

		for _, team := range comp.Competitors {
			teamID, _ := strconv.ParseInt(team.Team.ID, 10, 64)
			teamName := team.Team.DisplayName
			teamLogo := ""
			if len(team.Team.Logos) > 0 {
				teamLogo = team.Team.Logos[0].Href
			}

			if team.HomeAway == "home" {
				match.ESPNHomeTeamID = teamID
				match.HomeScore = team.Score
				match.HomeTeam = teamName // Guarda em memória
				match.HomeLogo = teamLogo // Guarda em memória
			} else {
				match.ESPNAwayTeamID = teamID
				match.AwayScore = team.Score
				match.AwayTeam = teamName // Guarda em memória
				match.AwayLogo = teamLogo // Guarda em memória
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
