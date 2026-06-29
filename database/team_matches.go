package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func GetMatchesByTeamID(teamID int64, roundStr string) ([]models.Match, error) {
	query := `
    SELECT 
        COALESCE(e.espn_match_id::TEXT, 0::TEXT), 
        COALESCE(m.league, ''),
        COALESCE(m.season, ''),
        COALESCE(m.round, 0),
        COALESCE(m.api_home_team_id, 0),
        COALESCE(th.name, ''),          
        COALESCE(th.espn_team_id, 0),   
        COALESCE(m.api_away_team_id, 0),
        COALESCE(ta.name, ''),          
        COALESCE(ta.espn_team_id, 0),   
        COALESCE(m.home_score, 0),
        COALESCE(m.away_score, 0),
        COALESCE(m.match_date::TEXT, ''), 
        COALESCE(m.status, ''),
		COALESCE(m.stage, ''),       -- ADICIONADO: Fase da competição
        COALESCE(m.group_name, ''),
        COALESCE(th.crest_url, '') AS home_logo,
        COALESCE(ta.crest_url, '') AS away_logo
    FROM matches m 
    LEFT JOIN teams th ON m.api_home_team_id = th.api_id
    LEFT JOIN teams ta ON m.api_away_team_id = ta.api_id
    LEFT JOIN espn_matches e ON (
        th.espn_team_id = e.espn_home_team_id 
        AND ta.espn_team_id = e.espn_away_team_id
        AND m.match_date::DATE = e.match_date::DATE
    )
    WHERE (m.api_home_team_id = $1 OR m.api_away_team_id = $1)
    `

	if roundStr != "" {
		// 🔥 AJUSTE AQUI: COALESCE garante que se a rodada for NULL (vazia no banco),
		// ela vira 0 e é lida pelo "0" que enviamos no Handler.
		query += ` AND COALESCE(m.round, 0) IN (` + roundStr + `)`
	}

	query += ` ORDER BY m.match_date ASC`

	rows, err := DB.Query(query, teamID)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetMatchesByTeamID: %v", err)
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		err := rows.Scan(
			&m.IDEvent,
			&m.League,
			&m.Season,
			&m.Round,
			&m.APIHomeTeamID,
			&m.HomeTeam,
			&m.ESPNHomeTeamID,
			&m.APIAwayTeamID,
			&m.AwayTeam,
			&m.ESPNAwayTeamID,
			&m.HomeScore,
			&m.AwayScore,
			&m.DateEvent,
			&m.Status,
			&m.Stage,
			&m.GroupName,
			&m.HomeLogo,
			&m.AwayLogo,
		)
		if err != nil {
			utils.CustomLog("DB_ERRO", "Erro no Scan GetMatchesByTeamID: %v", err)
			return nil, err
		}
		matches = append(matches, m)
	}

	if matches == nil {
		matches = []models.Match{}
	}
	return matches, nil
}

func GetCurrentRoundTeam(teamIDStr string) (int, error) {
	var round int

	query := `
        SELECT round FROM matches m
        WHERE (m.api_home_team_id = $1 OR m.api_away_team_id = $1) AND match_date <= NOW() 
        ORDER BY match_date DESC LIMIT 1
    `
	err := DB.QueryRow(query, teamIDStr).Scan(&round)

	if err != nil {
		queryFallback := `SELECT COALESCE(MAX(round), 1) FROM matches m WHERE m.api_home_team_id = $1 OR m.api_away_team_id = $1`
		errFallback := DB.QueryRow(queryFallback, teamIDStr).Scan(&round)
		if errFallback != nil {
			utils.CustomLog("DB_ERRO", "Erro no fallback GetCurrentRoundTeam: %v", errFallback)
			return 1, nil
		}
	}

	return round, nil
}
