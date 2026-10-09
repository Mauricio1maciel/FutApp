package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"

	"github.com/lib/pq"
)

func SaveFullMatchHistory(ctx context.Context, match models.ESPNMatchDB, lineups []models.ESPNLineupDB, events []models.ESPNEventDB) error {
	ctx, cancel := withTxTimeout(ctx)
	defer cancel()

	utils.CustomLog("DATABASE", "Iniciando persistência da partida %s...", match.MatchID)

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	// 🔥 1. AUTO-REGISTO NA TABELA TEAMS (só UNL: a football-data não cobre a Liga das Nações,
	// então as seleções são cadastradas com api_id = ESPNOnlyTeamIDOffset + ID da ESPN.
	// Usar o ID da ESPN puro colidia com os IDs da football-data: o SaveTeam de um clube
	// sobrescrevia a seleção (ex: Polônia virou Sassuolo).
	// Nas outras ligas os times vêm da football-data e são vinculados pelo SyncESPNTeamLinks.
	// ON CONFLICT sem alvo: ignora conflito em api_id E em espn_team_id (time já vinculado).
	// Qualquer erro aqui aborta a transação no Postgres, por isso não pode ser ignorado.
	if match.League == "UNL" {
		teamQuery := `
            INSERT INTO teams (api_id, espn_team_id, name, crest_url)
            VALUES ($4::BIGINT + $1::BIGINT, $1::BIGINT, $2, $3)
            ON CONFLICT DO NOTHING`

		teams := []struct {
			id         int64
			name, logo string
		}{
			{match.ESPNHomeTeamID, match.HomeTeam, match.HomeLogo},
			{match.ESPNAwayTeamID, match.AwayTeam, match.AwayLogo},
		}
		for _, t := range teams {
			if t.id == 0 || t.name == "" {
				continue // Time ainda indefinido (ex: mata-mata sem confronto)
			}
			if _, err = tx.ExecContext(ctx, teamQuery, t.id, t.name, t.logo, ESPNOnlyTeamIDOffset); err != nil {
				utils.CustomLog("DATABASE_ERRO", "Falha ao registrar time %d: %v", t.id, err)
				return err
			}
			// Vincula à UNL (busca e detalhes exigem team_leagues). Usa o api_id do time
			// dono do espn_team_id: seleções já cadastradas pela Copa mantêm o ID da football-data.
			// $2 com tipo explícito: usado sem cast, o Postgres não consegue deduzir o tipo
			if match.Season != "" {
				if _, err = tx.ExecContext(ctx, `
                    INSERT INTO team_leagues (team_api_id, league, season)
                    SELECT api_id, 'UNL', $2::varchar FROM teams WHERE espn_team_id = $1
                    ON CONFLICT DO NOTHING`, t.id, match.Season); err != nil {
					utils.CustomLog("DATABASE_ERRO", "Falha ao vincular time %d à UNL: %v", t.id, err)
					return err
				}
			}
		}
	}

	// 🔥 2. SALVAR NA ESPN_MATCHES (AGORA COM SEASON E 13 PARÂMETROS!)
	_, err = tx.ExecContext(ctx,
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

	// Só substitui a escalação se a ESPN devolveu uma nova; resposta vazia não apaga a que já existe
	if len(lineups) > 0 {
		if _, err = tx.ExecContext(ctx, `DELETE FROM espn_match_lineups WHERE espn_match_id = $1::BIGINT`, match.MatchID); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao limpar escalação: %v", err)
			return err
		}
	}
	// Sem escalação e sem eventos = gravação só do básico (dados do scoreboard): mantém os eventos
	if len(lineups) > 0 || len(events) > 0 {
		if _, err = tx.ExecContext(ctx, `DELETE FROM espn_match_events WHERE espn_match_id = $1::BIGINT`, match.MatchID); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao limpar eventos: %v", err)
			return err
		}
	}

	for _, l := range lineups {
		_, err = tx.ExecContext(ctx,
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
		_, err = tx.ExecContext(ctx,
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

	// Aviso de escalação confirmada. O savepoint isola uma falha aqui: sem ele, o erro
	// abortaria a transação e a escalação não seria gravada
	if len(lineups) > 0 {
		if _, err := tx.ExecContext(ctx, `SAVEPOINT push_lineup`); err == nil {
			if err := enqueueLineupPush(ctx, tx, match); err != nil {
				utils.CustomLog("DATABASE_ERRO", "Aviso de escalação do jogo %s não foi anotado: %v", match.MatchID, err)
				tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT push_lineup`)
			} else {
				tx.ExecContext(ctx, `RELEASE SAVEPOINT push_lineup`)
			}
		}
	}

	utils.CustomLog("DATABASE", "Dados da partida %s sincronizados com sucesso!", match.MatchID)
	return tx.Commit()
}

func GetFullMatchFromDB(ctx context.Context, matchID string) (*models.FullMatchHistory, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

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
	err := DB.QueryRowContext(ctx, query, matchID).Scan(
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

	rowsLineups, err := DB.QueryContext(ctx,
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

	rowsEvents, err := DB.QueryContext(ctx,
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

// GetCompleteESPNMatchIDs devolve quais desses jogos já estão encerrados e com
// escalação no banco (não precisam buscar o summary de novo)
func GetCompleteESPNMatchIDs(ctx context.Context, ids []int64) (map[string]bool, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	complete := make(map[string]bool)
	if len(ids) == 0 {
		return complete, nil
	}

	rows, err := DB.QueryContext(ctx, `
		SELECT e.espn_match_id::TEXT
		FROM espn_matches e
		WHERE e.espn_match_id = ANY($1)
		  AND e.status = 'post'
		  AND EXISTS (SELECT 1 FROM espn_match_lineups l WHERE l.espn_match_id = e.espn_match_id)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		complete[id] = true
	}
	return complete, rows.Err()
}
