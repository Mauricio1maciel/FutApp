package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"log"
)

func GetMatchesByLeague(league string, roundStr string) ([]models.Match, error) {
	query := `
    SELECT 
        COALESCE(0), 
        COALESCE(m.league, ''),
		COALESCE(l.name, ''), 
		COALESCE(l.logo_url, ''),
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
        COALESCE(th.crest_url, '') AS home_logo,
        COALESCE(ta.crest_url, '') AS away_logo
    FROM matches m
    LEFT JOIN teams th ON m.api_home_team_id = th.api_id
    LEFT JOIN teams ta ON m.api_away_team_id = ta.api_id
	LEFT JOIN leagues l ON m.league = l.code_api
    WHERE m.league = $1
    `
	if roundStr != "" {
		query += ` AND round = ` + roundStr
	}

	query += ` ORDER BY m.match_date ASC`

	rows, err := DB.Query(query, league)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetMatchesByLeague: %v", err)
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		err := rows.Scan(
			&m.IDEvent,
			&m.League,
			&m.LeagueName,
			&m.LeagueLogo,
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
			&m.HomeLogo,
			&m.AwayLogo,
		)
		if err != nil {
			utils.CustomLog("DB_ERRO", "Erro no Scan GetMatchesByLeague: %v", err)
			return nil, err
		}
		matches = append(matches, m)
	}

	if matches == nil {
		matches = []models.Match{}
	}
	return matches, nil
}

func SaveMatch(
	idEvent int64,
	league string,
	season string,
	round int,
	apiHomeTeamID int64,
	apiAwayTeamID int64,
	homeScore int,
	awayScore int,
	date string,
	status string,
) error {

	query := `
    INSERT INTO matches
    (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9, '')::TIMESTAMP,$10)
    ON CONFLICT (id_event, league) 
    DO UPDATE SET
         home_score = EXCLUDED.home_score,
         away_score = EXCLUDED.away_score,
         match_date = EXCLUDED.match_date,
         status = EXCLUDED.status,
         -- Só atualiza os IDs e a rodada se precisarem de correção na API
         round = EXCLUDED.round,
         api_home_team_id = EXCLUDED.api_home_team_id,
         api_away_team_id = EXCLUDED.api_away_team_id
    `

	_, err := DB.Exec(
		query,
		idEvent,
		league,
		season,
		round,
		apiHomeTeamID,
		apiAwayTeamID,
		homeScore,
		awayScore,
		date,
		status,
	)

	if err != nil {
		log.Printf("Erro ao salvar jogo: %v", err)
	}

	return err
}

func GetCurrentRound(league string) (int, error) {
	var round int

	query := `
        SELECT round FROM matches 
        WHERE league = $1 AND match_date <= NOW() 
        ORDER BY match_date DESC LIMIT 1
    `
	err := DB.QueryRow(query, league).Scan(&round)

	if err != nil {
		queryFallback := `SELECT MAX(round) FROM matches WHERE league = $1`
		errFallback := DB.QueryRow(queryFallback, league).Scan(&round)
		if errFallback != nil {
			return 1, nil
		}
	}

	return round, nil
}
