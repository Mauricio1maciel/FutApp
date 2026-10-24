package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

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

	// 🔥 1. AUTO-REGISTO NA TABELA TEAMS (só nas ligas que vêm só da ESPN, leagues.espn_only:
	// Liga das Nações e Sul-Americana). Os times que a football-data não conhece são
	// cadastrados com api_id = ESPNOnlyTeamIDOffset + ID da ESPN. Usar o ID da ESPN puro
	// colidia com os IDs da football-data: o SaveTeam de um clube sobrescrevia a seleção
	// (ex: Polônia virou Sassuolo).
	// Nas outras ligas os times vêm da football-data e são vinculados pelo SyncESPNTeamLinks.
	// ON CONFLICT sem alvo: ignora conflito em api_id E em espn_team_id (time já vinculado,
	// como os brasileiros da Sul-Americana, que já existem pelo Brasileirão).
	// Qualquer erro aqui aborta a transação no Postgres, por isso não pode ser ignorado.
	var espnOnly bool
	if err = tx.QueryRowContext(ctx,
		`SELECT COALESCE((SELECT espn_only FROM leagues WHERE code_api = $1), false)`, match.League).Scan(&espnOnly); err != nil {
		return err
	}
	if espnOnly {
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
			// Vincula à liga (busca e detalhes exigem team_leagues). Usa o api_id do time
			// dono do espn_team_id: quem já existe pela football-data (seleções da Copa,
			// clubes do Brasileirão) mantém o próprio ID.
			// $2 e $3 com tipo explícito: sem cast, o Postgres não consegue deduzir o tipo
			if match.Season != "" {
				if _, err = tx.ExecContext(ctx, `
                    INSERT INTO team_leagues (team_api_id, league, season)
                    SELECT api_id, $3::varchar, $2::varchar FROM teams WHERE espn_team_id = $1
                    ON CONFLICT DO NOTHING`, t.id, match.Season, match.League); err != nil {
					utils.CustomLog("DATABASE_ERRO", "Falha ao vincular time %d à liga %s: %v", t.id, match.League, err)
					return err
				}
			}
		}
	}

	// 🔥 2. SALVAR NA ESPN_MATCHES (com pênaltis: o trigger leva para matches nas ligas
	// que vêm só da ESPN)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO espn_matches (espn_match_id, league, season, match_date, home_logo, espn_home_team_id, away_logo, espn_away_team_id, home_score, away_score, status, stage, group_name, home_penalty, away_penalty, statistics, commentary) 
         VALUES ($1::BIGINT, $2, $3, NULLIF($4, '')::TIMESTAMP, $5, $6::BIGINT, $7, $8::BIGINT, $9, $10, $11, $12, $13, $14, $15, $16::JSONB, $17::JSONB)
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
             group_name = COALESCE(NULLIF(EXCLUDED.group_name, ''), espn_matches.group_name),
             home_penalty = COALESCE(EXCLUDED.home_penalty, espn_matches.home_penalty),
             away_penalty = COALESCE(EXCLUDED.away_penalty, espn_matches.away_penalty),
             statistics = COALESCE(EXCLUDED.statistics, espn_matches.statistics),
             commentary = COALESCE(EXCLUDED.commentary, espn_matches.commentary)`,
		match.MatchID, match.League, match.Season, match.MatchDate,
		match.HomeLogo, match.ESPNHomeTeamID,
		match.AwayLogo, match.ESPNAwayTeamID,
		match.HomeScore, match.AwayScore, match.Status,
		match.Stage, match.GroupName,
		match.HomePenalty, match.AwayPenalty, statsJSON(match.Statistics), commentaryJSON(match.Commentary),
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

	// Escalação e eventos num comando cada (eram uma linha por comando: ~50 idas e voltas
	// por jogo ao vivo, a cada minuto). WITH ORDINALITY mantém a ordem da ESPN nos ids,
	// que o aviso de gol usa para achar o último gol
	if len(lineups) > 0 {
		n := len(lineups)
		matchIDs, names, jerseys := make([]string, n), make([]string, n), make([]string, n)
		positions, formations := make([]string, n), make([]string, n)
		teamIDs, playerIDs := make([]int64, n), make([]int64, n)
		starters := make([]bool, n)
		for i, l := range lineups {
			matchIDs[i], teamIDs[i], playerIDs[i] = l.MatchID, l.ESPNTeamID, l.ESPNPlayerID
			names[i], jerseys[i], positions[i] = l.PlayerName, l.Jersey, l.Position
			starters[i], formations[i] = l.IsStarter, l.Formation
		}
		if _, err = tx.ExecContext(ctx, `
            INSERT INTO espn_match_lineups (espn_match_id, espn_team_id, espn_player_id, player_name, jersey, position, is_starter, formation)
            SELECT m, t, p, n, j, pos, s, f
            FROM unnest($1::bigint[], $2::bigint[], $3::bigint[], $4::varchar[], $5::varchar[], $6::varchar[], $7::boolean[], $8::varchar[])
                 WITH ORDINALITY AS l(m, t, p, n, j, pos, s, f, ord)
            ORDER BY ord`,
			pq.Array(matchIDs), pq.Array(teamIDs), pq.Array(playerIDs), pq.Array(names),
			pq.Array(jerseys), pq.Array(positions), pq.Array(starters), pq.Array(formations)); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao inserir escalação: %v", err)
			return err
		}
	}

	if len(events) > 0 {
		n := len(events)
		matchIDs, minutes, types := make([]string, n), make([]string, n), make([]string, n)
		players, details := make([]string, n), make([]string, n)
		teamIDs := make([]int64, n)
		for i, e := range events {
			matchIDs[i], minutes[i], types[i] = e.MatchID, e.Minute, e.EventType
			teamIDs[i], players[i], details[i] = e.ESPNTeamID, e.PlayerName, e.Details
		}
		if _, err = tx.ExecContext(ctx, `
            INSERT INTO espn_match_events (espn_match_id, minute, event_type, espn_team_id, player_name, details)
            SELECT m, mi, ty, t, p, d
            FROM unnest($1::bigint[], $2::varchar[], $3::varchar[], $4::bigint[], $5::varchar[], $6::text[])
                 WITH ORDINALITY AS e(m, mi, ty, t, p, d, ord)
            ORDER BY ord`,
			pq.Array(matchIDs), pq.Array(minutes), pq.Array(types), pq.Array(teamIDs),
			pq.Array(players), pq.Array(details)); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao inserir evento: %v", err)
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

	// Avisos dos lances (cartão vermelho, pênaltis e intervalo), isolados do mesmo jeito
	if len(events) > 0 {
		if _, err := tx.ExecContext(ctx, `SAVEPOINT push_events`); err == nil {
			if err := enqueueEventPushes(ctx, tx, match, events, lineups); err != nil {
				utils.CustomLog("DATABASE_ERRO", "Avisos dos lances do jogo %s não foram anotados: %v", match.MatchID, err)
				tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT push_events`)
			} else {
				tx.ExecContext(ctx, `RELEASE SAVEPOINT push_events`)
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
	var homePen, awayPen sql.NullInt64
	var rawStats []byte

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
            COALESCE(e.group_name, ''),
            e.home_penalty,
            e.away_penalty,
            e.statistics
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
		&homePen, &awayPen, &rawStats,
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

	if homePen.Valid && awayPen.Valid {
		h, a := int(homePen.Int64), int(awayPen.Int64)
		history.Match.HomePenalty, history.Match.AwayPenalty = &h, &a
	}
	var stats *models.MatchStatistics
	if len(rawStats) > 0 {
		stats = &models.MatchStatistics{}
		if err := json.Unmarshal(rawStats, stats); err != nil {
			utils.CustomLog("DATABASE_ERRO", "Estatísticas ilegíveis do jogo %s: %v", matchID, err)
			stats = nil
		}
	}
	history.Statistics = models.MatchStatList(stats)

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
		  AND e.statistics IS NOT NULL
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

// statsJSON converte as estatísticas para gravar (nil vira NULL: o banco mantém as que já
// tinha, como na gravação só com os dados do placar)
func statsJSON(s *models.MatchStatistics) interface{} {
	if s == nil {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return string(b)
}

func commentaryJSON(c []models.CommentaryEntry) interface{} {
	if len(c) == 0 {
		return nil // fica a narração já gravada
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	return string(b)
}

// GetMatchCommentary devolve a narração do jogo (ID da ESPN), do lance mais recente para o
// mais antigo. found = false se o jogo não existe
func GetMatchCommentary(ctx context.Context, espnMatchID int64) (entries []models.CommentaryEntry, found bool, err error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var raw []byte
	err = DB.QueryRowContext(ctx, `SELECT commentary FROM espn_matches WHERE espn_match_id = $1`, espnMatchID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	entries = []models.CommentaryEntry{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, true, err
		}
	}
	slices.Reverse(entries)
	return entries, true, nil
}

// ClearOldCommentary apaga a narração dos jogos com mais de maxAge (ocupa espaço e só
// interessa no jogo e logo depois)
func ClearOldCommentary(ctx context.Context, maxAge time.Duration) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	res, err := DB.ExecContext(ctx, `
        UPDATE espn_matches SET commentary = NULL
        WHERE commentary IS NOT NULL AND match_date < (now() AT TIME ZONE 'UTC') - $1 * INTERVAL '1 second'`,
		maxAge.Seconds())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
