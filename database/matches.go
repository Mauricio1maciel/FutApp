package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"log"
	"strconv"
)

func GetMatchesByLeague(league string, roundStr string, dateStr string, season string, isCurrentRound bool) ([]models.Match, error) {
	query := `
    SELECT 
        COALESCE(e.espn_match_id::TEXT, 0::TEXT),  
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
        COALESCE(m.stage, ''),       -- ADICIONADO: Fase da competição
        COALESCE(m.group_name, ''),  -- ADICIONADO: Nome do grupo
        COALESCE(th.crest_url, '') AS home_logo,
        COALESCE(ta.crest_url, '') AS away_logo
    FROM matches m
    LEFT JOIN teams th ON m.api_home_team_id = th.api_id
    LEFT JOIN teams ta ON m.api_away_team_id = ta.api_id
    LEFT JOIN leagues l ON m.league = l.code_api
	 LEFT JOIN espn_matches e ON (
        th.espn_team_id = e.espn_home_team_id 
        AND ta.espn_team_id = e.espn_away_team_id
        AND m.match_date::DATE = e.match_date::DATE
    )
    WHERE m.league = $1 AND m.season = $2
    `
	// 🔥 LÓGICA DE FILTRO ALTERADA
	if dateStr != "" {
		// Se veio data, ignora rodada e fase. Pega só os jogos desse dia (no fuso BR)
		query += ` AND (m.match_date AT TIME ZONE 'UTC' AT TIME ZONE 'America/Sao_Paulo')::DATE = '` + dateStr + `'::DATE`
	} else if roundStr != "" {
		if _, err := strconv.Atoi(roundStr); err == nil {
			query += ` AND m.round = ` + roundStr
		} else {
			query += ` AND m.stage = '` + roundStr + `'`
		}

		if isCurrentRound && (league == "WC" || league == "CL" || league == "CLI") {
			query += ` 
               AND (m.match_date AT TIME ZONE 'UTC' AT TIME ZONE 'America/Sao_Paulo')::DATE >= (CURRENT_TIMESTAMP AT TIME ZONE 'America/Sao_Paulo')::DATE
               AND (m.match_date AT TIME ZONE 'UTC' AT TIME ZONE 'America/Sao_Paulo')::DATE <= (CURRENT_TIMESTAMP AT TIME ZONE 'America/Sao_Paulo')::DATE + INTERVAL '1 day'`
		}
	}

	query += ` ORDER BY m.match_date ASC`

	rows, err := DB.Query(query, league, season)
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
			&m.Stage,
			&m.GroupName,
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
	stage string,
	groupName string,
) error {

	query := `
    INSERT INTO matches
    (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, match_date, status, stage, group_name)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9, '')::TIMESTAMP,$10,$11,$12)
    ON CONFLICT (id_event, league) 
    DO UPDATE SET
         home_score = EXCLUDED.home_score,
         away_score = EXCLUDED.away_score,
         match_date = EXCLUDED.match_date,
         status = EXCLUDED.status,
         -- Só atualiza os IDs, rodada, fase e grupo se precisarem de correção na API
         round = EXCLUDED.round,
         api_home_team_id = EXCLUDED.api_home_team_id,
         api_away_team_id = EXCLUDED.api_away_team_id,
         stage = EXCLUDED.stage,             -- ADICIONADO
         group_name = EXCLUDED.group_name    -- ADICIONADO
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
		stage,     // ADICIONADO
		groupName, // ADICIONADO
	)

	if err != nil {
		log.Printf("Erro ao salvar jogo: %v", err)
	}

	return err
}

func GetCurrentRound(league string, season string) (int, error) {
	var round int

	query := `
        SELECT round FROM matches 
        WHERE league = $1 AND AND season = $2 AND match_date <= NOW() 
        ORDER BY match_date DESC LIMIT 1
    `
	err := DB.QueryRow(query, league, season).Scan(&round)

	if err != nil {
		return 1, nil

	}

	return round, nil
}
func GetCurrentPhase(league string, season string) string {
	var stage string

	// 1. Tenta buscar a fase mais recente que já aconteceu
	query := `
        SELECT stage FROM matches 
        WHERE league = $1 
		  AND match_date >= NOW() 
		  AND stage != ''
		  AND stage NOT IN ('REGULAR_SEASON', 'GROUP_STAGE')
        ORDER BY match_date ASC LIMIT 1
    `
	err := DB.QueryRow(query, league).Scan(&stage)

	// 2. Se não achou (ou está na fase de grupos), retorna um valor que o sistema entenda como rodada atual
	if err != nil {
		return "CURRENT_ROUND" // Marcador para o handler saber que deve tratar como rodada numérica
	}

	return stage
}
func GetLatestSeason(league string) string {
	var season string
	// Busca a season mais recente cadastrada para aquela liga
	query := `SELECT season FROM matches WHERE league = $1 ORDER BY season DESC LIMIT 1`
	err := DB.QueryRow(query, league).Scan(&season)
	if err != nil {
		return "2026" // Fallback
	}
	return season
}
