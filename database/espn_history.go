package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func SaveFullMatchHistory(match models.ESPNMatchDB, lineups []models.ESPNLineupDB, events []models.ESPNEventDB) error {
	utils.CustomLog("DATABASE", "Iniciando persistência da partida %s...", match.MatchID)

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	// 🔥 1. AUTO-REGISTO NA TABELA TEAMS
	_, _ = tx.Exec(`
        INSERT INTO teams (api_id, espn_team_id, name, crest_url) 
        VALUES ($1::BIGINT, $1::BIGINT, $2, $3) 
        ON CONFLICT (api_id) DO NOTHING`,
		match.ESPNHomeTeamID, match.HomeTeam, match.HomeLogo)

	_, _ = tx.Exec(`
        INSERT INTO teams (api_id, espn_team_id, name, crest_url) 
        VALUES ($1::BIGINT, $1::BIGINT, $2, $3) 
        ON CONFLICT (api_id) DO NOTHING`,
		match.ESPNAwayTeamID, match.AwayTeam, match.AwayLogo)

	// 🔥 2. SALVAR NA ESPN_MATCHES (AGORA COM SEASON E 13 PARÂMETROS!)
	_, err = tx.Exec(
		`INSERT INTO espn_matches (espn_match_id, league, season, match_date, home_logo, espn_home_team_id, away_logo, espn_away_team_id, home_score, away_score, status, stage, group_name) 
         VALUES ($1::BIGINT, $2, $3, NULLIF($4, '')::TIMESTAMP, $5, $6::BIGINT, $7, $8::BIGINT, $9, $10, $11, $12, $13)
         ON CONFLICT (espn_match_id) DO UPDATE 
         SET home_score = EXCLUDED.home_score,
             away_score = EXCLUDED.away_score,
             status = EXCLUDED.status,
             league = EXCLUDED.league,
             season = EXCLUDED.season,
             match_date = NULLIF(EXCLUDED.match_date::TEXT, '')::TIMESTAMP,
             home_logo = EXCLUDED.home_logo,
             away_logo = EXCLUDED.away_logo,
             espn_home_team_id = EXCLUDED.espn_home_team_id,
             espn_away_team_id = EXCLUDED.espn_away_team_id,
             stage = COALESCE(NULLIF(EXCLUDED.stage, ''), espn_matches.stage),           
             group_name = COALESCE(NULLIF(EXCLUDED.group_name, ''), espn_matches.group_name)`,
		match.MatchID, match.League, match.Season, match.MatchDate,
		match.HomeLogo, match.ESPNHomeTeamID,
		match.AwayLogo, match.ESPNAwayTeamID,
		match.HomeScore, match.AwayScore, match.Status,
		match.Stage, match.GroupName,
	)
	if err != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao inserir partida: %v", err)
		tx.Rollback()
		return err
	}

	tx.Exec(`DELETE FROM espn_match_lineups WHERE espn_match_id = $1::BIGINT`, match.MatchID)
	tx.Exec(`DELETE FROM espn_match_events WHERE espn_match_id = $1::BIGINT`, match.MatchID)

	for _, l := range lineups {
		_, err = tx.Exec(
			`INSERT INTO espn_match_lineups (espn_match_id, espn_team_id, espn_player_id, player_name, jersey, position, is_starter, formation)
             VALUES ($1::BIGINT, $2::BIGINT, $3::BIGINT, $4, $5, $6, $7, $8)`,
			l.MatchID, l.ESPNTeamID, l.ESPNPlayerID, l.PlayerName, l.Jersey, l.Position, l.IsStarter, l.Formation,
		)
		if err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao inserir escalação: %v", err)
			tx.Rollback()
			return err
		}
	}

	for _, e := range events {
		_, err = tx.Exec(
			`INSERT INTO espn_match_events (espn_match_id, minute, event_type, espn_team_id, player_name, details)
             VALUES ($1::BIGINT, $2, $3, $4::BIGINT, $5, $6)`,
			e.MatchID, e.Minute, e.EventType, e.ESPNTeamID, e.PlayerName, e.Details,
		)
		if err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao inserir evento: %v", err)
			tx.Rollback()
			return err
		}
	}

	utils.CustomLog("DATABASE", "Dados da partida %s sincronizados com sucesso!", match.MatchID)
	return tx.Commit()
}

func GetFullMatchFromDB(matchID string) (*models.FullMatchHistory, error) {
	var history models.FullMatchHistory

	// 🔥 ADICIONADO O CAMPO e.season NO SELECT
	query := `
        SELECT 
            e.espn_match_id::TEXT, 
            COALESCE(e.league, ''), 
            COALESCE(e.season, ''), 
            COALESCE(e.match_date::TEXT, ''), 
            COALESCE(th.api_id, 0), 
            COALESCE(th.name, ''),  
            COALESCE(th.crest_url, ''), 
            COALESCE(e.espn_home_team_id::TEXT, ''), 
            COALESCE(ta.api_id, 0), 
            COALESCE(ta.name, ''),  
            COALESCE(ta.crest_url, ''), 
            COALESCE(e.espn_away_team_id::TEXT, ''), 
            COALESCE(e.home_score, ''), 
            COALESCE(e.away_score, ''), 
            COALESCE(e.status, ''),
            COALESCE(e.stage, ''),       
            COALESCE(e.group_name, '')   
        FROM espn_matches e
        LEFT JOIN teams th ON e.espn_home_team_id::BIGINT = th.espn_team_id::BIGINT
        LEFT JOIN teams ta ON e.espn_away_team_id::BIGINT = ta.espn_team_id::BIGINT
        WHERE e.espn_match_id::BIGINT = $1::BIGINT 
        LIMIT 1`

	// 🔥 ADICIONADO O CAMPO &history.Match.Season NO SCAN
	err := DB.QueryRow(query, matchID).Scan(
		&history.Match.MatchID,
		&history.Match.League,
		&history.Match.Season,
		&history.Match.MatchDate,
		&history.Match.APIHomeTeamID,
		&history.Match.HomeTeam,
		&history.Match.HomeLogo,
		&history.Match.ESPNHomeTeamID,
		&history.Match.APIAwayTeamID,
		&history.Match.AwayTeam,
		&history.Match.AwayLogo,
		&history.Match.ESPNAwayTeamID,
		&history.Match.HomeScore,
		&history.Match.AwayScore,
		&history.Match.Status,
		&history.Match.Stage,
		&history.Match.GroupName,
	)

	if err != nil {
		utils.CustomLog("DATABASE_ERRO", "Erro no Scan Principal: %v", err)
		return nil, err
	}

	rowsLineups, err := DB.Query(
		`SELECT 
            l.espn_team_id::TEXT, 
            COALESCE(l.espn_player_id, 0), 
            l.player_name, 
            l.jersey, 
            l.position, 
            l.is_starter, 
            l.formation,
            COALESCE(p.headshot_url, '') AS headshot_url 
        FROM espn_match_lineups l
        LEFT JOIN espn_players p ON l.espn_player_id::BIGINT = p.espn_id::BIGINT
        WHERE l.espn_match_id::BIGINT = $1::BIGINT`, matchID)
	if err == nil {
		defer rowsLineups.Close()
		for rowsLineups.Next() {
			var l models.ESPNLineupDB
			l.MatchID = matchID
			err := rowsLineups.Scan(&l.ESPNTeamID, &l.ESPNPlayerID, &l.PlayerName, &l.Jersey, &l.Position, &l.IsStarter, &l.Formation, &l.HeadshotURL)
			if err == nil {
				history.Lineups = append(history.Lineups, l)
			} else {
				utils.CustomLog("DATABASE_ERRO", "Erro no Scan de Escalação: %v", err)
			}
		}
	}

	rowsEvents, err := DB.Query(
		`SELECT minute, event_type, espn_team_id::TEXT, player_name, details 
         FROM espn_match_events WHERE espn_match_id::BIGINT = $1::BIGINT ORDER BY id ASC`, matchID)
	if err == nil {
		defer rowsEvents.Close()
		for rowsEvents.Next() {
			var e models.ESPNEventDB
			e.MatchID = matchID
			err := rowsEvents.Scan(&e.Minute, &e.EventType, &e.ESPNTeamID, &e.PlayerName, &e.Details)
			if err == nil {
				history.Events = append(history.Events, e)
			}
		}
	}

	if history.Lineups == nil {
		history.Lineups = []models.ESPNLineupDB{}
	}
	if history.Events == nil {
		history.Events = []models.ESPNEventDB{}
	}

	return &history, nil
}
