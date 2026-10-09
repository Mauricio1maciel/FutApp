package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

// Notificações push: aparelhos, o que cada um segue e a fila de avisos (migration 009).
// O trigger trg_matches_push_outbox anota início, gol e fim; a escalação é anotada
// aqui, por enqueueLineupPush.

var (
	// ErrDeviceNotRegistered: o aparelho tentou seguir algo antes de registrar o token
	ErrDeviceNotRegistered = errors.New("aparelho não registrado para push")
	// ErrNotFound: o jogo ou o time não existe
	ErrNotFound = errors.New("não encontrado")
)

// RegisterPushDevice grava (ou atualiza) o token de push do aparelho. Se o mesmo token
// estava com outro device_id (app reinstalado), o registro antigo sai.
func RegisterPushDevice(ctx context.Context, deviceID, token, platform string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM push_devices WHERE push_token = $1 AND device_id <> $2`, token, deviceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO push_devices (device_id, push_token, platform)
        VALUES ($1, $2, $3)
        ON CONFLICT (device_id) DO UPDATE
        SET push_token = EXCLUDED.push_token,
            platform   = EXCLUDED.platform,
            updated_at = now()`, deviceID, token, platform); err != nil {
		return err
	}
	return tx.Commit()
}

// UnregisterPushDevice apaga o aparelho e, em cascata, tudo o que ele seguia
func UnregisterPushDevice(ctx context.Context, deviceID string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `DELETE FROM push_devices WHERE device_id = $1`, deviceID)
	return err
}

// DeletePushToken remove um token que o Expo disse não existir mais (app desinstalado)
func DeletePushToken(ctx context.Context, token string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `DELETE FROM push_devices WHERE push_token = $1`, token)
	return err
}

// GetPushToken devolve o token de push de um aparelho
func GetPushToken(ctx context.Context, deviceID string) (string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var token string
	err := DB.QueryRowContext(ctx,
		`SELECT push_token FROM push_devices WHERE device_id = $1`, deviceID).Scan(&token)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return token, err
}

// SubscribeMatch passa a seguir um jogo (repetir não dá erro)
func SubscribeMatch(ctx context.Context, deviceID string, matchID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO match_subscriptions (device_id, match_id) VALUES ($1, $2)
        ON CONFLICT DO NOTHING`, deviceID, matchID)
	return subscriptionError(err, "match_subscriptions_device_fkey")
}

// UnsubscribeMatch deixa de seguir um jogo
func UnsubscribeMatch(ctx context.Context, deviceID string, matchID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`DELETE FROM match_subscriptions WHERE device_id = $1 AND match_id = $2`, deviceID, matchID)
	return err
}

// SubscribeTeam passa a seguir todos os jogos de um time (repetir não dá erro)
func SubscribeTeam(ctx context.Context, deviceID string, teamID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO team_subscriptions (device_id, team_api_id) VALUES ($1, $2)
        ON CONFLICT DO NOTHING`, deviceID, teamID)
	return subscriptionError(err, "team_subscriptions_device_fkey")
}

// UnsubscribeTeam deixa de seguir um time
func UnsubscribeTeam(ctx context.Context, deviceID string, teamID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`DELETE FROM team_subscriptions WHERE device_id = $1 AND team_api_id = $2`, deviceID, teamID)
	return err
}

// subscriptionError traduz a violação de chave estrangeira: ou o aparelho não foi
// registrado, ou o jogo/time não existe
func subscriptionError(err error, deviceConstraint string) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23503" {
		if pqErr.Constraint == deviceConstraint {
			return ErrDeviceNotRegistered
		}
		return ErrNotFound
	}
	return err
}

// GetPushSubscriptions devolve os jogos e os times que o aparelho segue
func GetPushSubscriptions(ctx context.Context, deviceID string) ([]int64, []int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	matches, err := queryIDs(ctx,
		`SELECT match_id FROM match_subscriptions WHERE device_id = $1 ORDER BY match_id`, deviceID)
	if err != nil {
		return nil, nil, err
	}
	teams, err := queryIDs(ctx,
		`SELECT team_api_id FROM team_subscriptions WHERE device_id = $1 ORDER BY team_api_id`, deviceID)
	if err != nil {
		return nil, nil, err
	}
	return matches, teams, nil
}

func queryIDs(ctx context.Context, query string, args ...interface{}) ([]int64, error) {
	rows, err := DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ResolveMatchIDByESPN acha o nosso match_id a partir do ID da ESPN (a tela de jogos ao
// vivo só conhece o da ESPN). Mesmo vínculo das listas de jogos: times + data.
func ResolveMatchIDByESPN(ctx context.Context, espnMatchID int64) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var matchID int64
	err := DB.QueryRowContext(ctx, `
        SELECT m.id
        FROM espn_matches e
        JOIN teams th ON th.espn_team_id = e.espn_home_team_id
        JOIN teams ta ON ta.espn_team_id = e.espn_away_team_id
        JOIN matches m ON m.api_home_team_id = th.api_id
                      AND m.api_away_team_id = ta.api_id
                      AND m.match_date::date = e.match_date::date
        WHERE e.espn_match_id = $1
        LIMIT 1`, espnMatchID).Scan(&matchID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return matchID, err
}

// GetPushTokensForMatch devolve os tokens de quem segue o jogo ou um dos times, sem repetir
func GetPushTokensForMatch(ctx context.Context, matchID int64) ([]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT DISTINCT d.push_token
        FROM push_devices d
        WHERE d.device_id IN (
            SELECT device_id FROM match_subscriptions WHERE match_id = $1
            UNION
            SELECT ts.device_id
            FROM team_subscriptions ts
            JOIN matches m ON m.id = $1
            WHERE ts.team_api_id IN (m.api_home_team_id, m.api_away_team_id)
        )`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// PendingPush é um aviso da fila com o que o texto da notificação precisa
type PendingPush struct {
	ID          int64
	MatchID     int64
	Kind        string // start | goal | end | lineup
	HomeScore   int
	AwayScore   int
	GoalSide    string // gol: home | away | both
	HomePenalty *int
	AwayPenalty *int
	League      string
	HomeTeam    string
	AwayTeam    string
	MatchDate   time.Time // zero se o jogo não tem data
	ESPNMatchID int64     // 0 enquanto o jogo não tem vínculo com a ESPN
	ESPNHomeID  int64
	ESPNAwayID  int64
	Scorer      string // autor do gol, quando a ESPN já informou
	Minute      string // ex: "23'", "90'+3'"
	OwnGoal     bool
}

// GetPendingPushes devolve os avisos ainda não enviados, do mais antigo para o mais novo
func GetPendingPushes(ctx context.Context, limit int) ([]PendingPush, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT o.id, o.match_id, o.kind, o.home_score, o.away_score, o.goal_side,
               m.home_penalty, m.away_penalty,
               COALESCE(m.league, ''),
               COALESCE(NULLIF(th.short, ''), th.name, ''),
               COALESCE(NULLIF(ta.short, ''), ta.name, ''),
               m.match_date AT TIME ZONE 'UTC',
               COALESCE(e.espn_match_id, 0),
               COALESCE(th.espn_team_id, 0),
               COALESCE(ta.espn_team_id, 0),
               COALESCE(gol.player_name, ''),
               COALESCE(gol.minute, ''),
               COALESCE(gol.event_type = 'Own Goal', false)
        FROM push_outbox o
        JOIN matches m ON m.id = o.match_id
        LEFT JOIN teams th ON th.api_id = m.api_home_team_id
        LEFT JOIN teams ta ON ta.api_id = m.api_away_team_id
        LEFT JOIN LATERAL (
            SELECT espn_match_id FROM espn_matches
            WHERE espn_home_team_id = th.espn_team_id
              AND espn_away_team_id = ta.espn_team_id
              AND match_date::date = m.match_date::date
            ORDER BY espn_match_id DESC
            LIMIT 1
        ) e ON true
        LEFT JOIN LATERAL (
            -- Último gol do time que marcou. Só vale se a ESPN já tem todos os gols
            -- dele (total = placar); senão o "último" seria um gol anterior
            SELECT g.player_name, g.minute, g.event_type
            FROM (
                SELECT ev.id, ev.player_name, ev.minute, ev.event_type,
                       count(*) OVER () AS total
                FROM espn_match_events ev
                WHERE ev.espn_match_id = e.espn_match_id
                  AND ev.espn_team_id = CASE o.goal_side WHEN 'home' THEN th.espn_team_id
                                                         ELSE ta.espn_team_id END
                  AND (ev.event_type IN ('Goal', 'Own Goal', 'Penalty - Scored')
                       OR ev.event_type LIKE 'Goal - %')
            ) g
            WHERE o.kind = 'goal'
              AND o.goal_side IN ('home', 'away')
              AND g.total = CASE o.goal_side WHEN 'home' THEN o.home_score ELSE o.away_score END
            ORDER BY g.id DESC
            LIMIT 1
        ) gol ON true
        WHERE o.sent_at IS NULL
        ORDER BY o.id
        LIMIT $1`, limit)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetPendingPushes: %v", err)
		return nil, err
	}
	defer rows.Close()

	var pending []PendingPush
	for rows.Next() {
		var p PendingPush
		var date sql.NullTime
		var homePen, awayPen sql.NullInt64
		if err := rows.Scan(&p.ID, &p.MatchID, &p.Kind, &p.HomeScore, &p.AwayScore, &p.GoalSide,
			&homePen, &awayPen, &p.League, &p.HomeTeam, &p.AwayTeam, &date,
			&p.ESPNMatchID, &p.ESPNHomeID, &p.ESPNAwayID,
			&p.Scorer, &p.Minute, &p.OwnGoal); err != nil {
			utils.CustomLog("DB_ERRO", "Erro no Scan de GetPendingPushes: %v", err)
			return nil, err
		}
		if date.Valid {
			p.MatchDate = date.Time
		}
		if homePen.Valid && awayPen.Valid {
			h, a := int(homePen.Int64), int(awayPen.Int64)
			p.HomePenalty, p.AwayPenalty = &h, &a
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

// MarkPushSent tira o aviso da fila, guardando o resultado do envio
func MarkPushSent(ctx context.Context, id int64, result string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`UPDATE push_outbox SET sent_at = now(), result = $2 WHERE id = $1`, id, result)
	return err
}

// ExpireStalePushes tira da fila os avisos velhos demais: se o worker ficou fora do ar,
// um "GOL!" de meia hora atrás só atrapalha
func ExpireStalePushes(ctx context.Context, maxAge time.Duration) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	res, err := DB.ExecContext(ctx, `
        UPDATE push_outbox SET sent_at = now(), result = 'expirado'
        WHERE sent_at IS NULL AND created_at < now() - $1 * INTERVAL '1 second'`,
		maxAge.Seconds())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// enqueueLineupPush anota o aviso de escalação confirmada, uma vez por jogo, na mesma
// transação que grava a escalação. Só para jogo que ainda vai começar (nas próximas
// 3 horas) e que alguém segue.
func enqueueLineupPush(ctx context.Context, tx *sql.Tx, match models.ESPNMatchDB) error {
	_, err := tx.ExecContext(ctx, `
        WITH jogo AS (
            SELECT m.id
            FROM matches m
            JOIN teams th ON th.api_id = m.api_home_team_id
            JOIN teams ta ON ta.api_id = m.api_away_team_id
            WHERE th.espn_team_id = $1
              AND ta.espn_team_id = $2
              AND m.match_date::date = NULLIF($3, '')::timestamp::date
              AND m.status IN ('SCHEDULED', 'TIMED')
              AND m.match_date BETWEEN (now() AT TIME ZONE 'UTC')
                                   AND (now() AT TIME ZONE 'UTC') + INTERVAL '3 hours'
              AND push_match_has_followers(m.id, m.api_home_team_id, m.api_away_team_id)
            LIMIT 1
        ), marcado AS (
            INSERT INTO match_push_state (match_id, lineup)
            SELECT id, true FROM jogo
            ON CONFLICT (match_id) DO UPDATE SET lineup = true
            WHERE match_push_state.lineup = false
            RETURNING match_id
        )
        INSERT INTO push_outbox (match_id, kind)
        SELECT match_id, 'lineup' FROM marcado`,
		match.ESPNHomeTeamID, match.ESPNAwayTeamID, match.MatchDate)
	return err
}
