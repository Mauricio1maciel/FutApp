package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/lib/pq"
)

// Notificações push: aparelhos, o que cada um segue (jogos, times e ligas) e a fila de
// avisos (migrations 009 e 010).
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
            web_p256dh = NULL,
            web_auth   = NULL,
            updated_at = now()`, deviceID, token, platform); err != nil {
		return err
	}
	return tx.Commit()
}

// RegisterWebPushDevice registra a inscrição de push do navegador (versão web). O endpoint
// faz o papel do token do Expo; p256dh e auth são as chaves da inscrição.
func RegisterWebPushDevice(ctx context.Context, deviceID, endpoint, p256dh, auth string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM push_devices WHERE push_token = $1 AND device_id <> $2`, endpoint, deviceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO push_devices (device_id, push_token, platform, web_p256dh, web_auth)
        VALUES ($1, $2, 'web', $3, $4)
        ON CONFLICT (device_id) DO UPDATE
        SET push_token = EXCLUDED.push_token,
            platform   = 'web',
            web_p256dh = EXCLUDED.web_p256dh,
            web_auth   = EXCLUDED.web_auth,
            updated_at = now()`, deviceID, endpoint, p256dh, auth); err != nil {
		return err
	}
	return tx.Commit()
}

// PushTarget é para onde vai um aviso: um aparelho com o app (Token = ExponentPushToken,
// enviado pelo Expo) ou um navegador (Platform "web", Token = endpoint, enviado por Web
// Push com as chaves P256dh e Auth)
type PushTarget struct {
	Token    string
	Platform string
	P256dh   string
	Auth     string
}

func (t PushTarget) IsWeb() bool { return t.Platform == "web" }

const pushTargetColumns = `d.push_token, COALESCE(d.platform, ''), COALESCE(d.web_p256dh, ''), COALESCE(d.web_auth, '')`

func scanPushTargets(rows *sql.Rows) ([]PushTarget, error) {
	defer rows.Close()
	var targets []PushTarget
	for rows.Next() {
		var t PushTarget
		if err := rows.Scan(&t.Token, &t.Platform, &t.P256dh, &t.Auth); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// GetPushTarget devolve o destino de um aparelho (ErrNotFound se não registrou o push)
func GetPushTarget(ctx context.Context, deviceID string) (PushTarget, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		`SELECT `+pushTargetColumns+` FROM push_devices d WHERE d.device_id = $1`, deviceID)
	if err != nil {
		return PushTarget{}, err
	}
	targets, err := scanPushTargets(rows)
	if err != nil {
		return PushTarget{}, err
	}
	if len(targets) == 0 {
		return PushTarget{}, ErrNotFound
	}
	return targets[0], nil
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
// PushKinds são os tipos de aviso, na ordem em que acontecem no jogo
var PushKinds = []string{"lineup", "start", "goal", "red_card", "end"}

// kindsArray: nil = NULL (todos os tipos)
func kindsArray(kinds []string) interface{} {
	if kinds == nil {
		return nil
	}
	return pq.Array(kinds)
}

// As funções Subscribe* seguem um jogo, time ou liga. setKinds = false mantém os tipos de
// aviso que a inscrição já tinha (ou todos, se é nova); setKinds = true grava kinds
// (nil = todos).
func SubscribeMatch(ctx context.Context, deviceID string, matchID int64, kinds []string, setKinds bool) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO match_subscriptions (device_id, match_id, kinds)
        VALUES ($1, $2, CASE WHEN $4 THEN $3::text[] END)
        ON CONFLICT (device_id, match_id) DO UPDATE
        SET kinds = CASE WHEN $4 THEN EXCLUDED.kinds ELSE match_subscriptions.kinds END`,
		deviceID, matchID, kindsArray(kinds), setKinds)
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
func SubscribeTeam(ctx context.Context, deviceID string, teamID int64, kinds []string, setKinds bool) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO team_subscriptions (device_id, team_api_id, kinds)
        VALUES ($1, $2, CASE WHEN $4 THEN $3::text[] END)
        ON CONFLICT (device_id, team_api_id) DO UPDATE
        SET kinds = CASE WHEN $4 THEN EXCLUDED.kinds ELSE team_subscriptions.kinds END`,
		deviceID, teamID, kindsArray(kinds), setKinds)
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

// SubscribeLeague passa a seguir todos os jogos de uma liga (repetir não dá erro)
func SubscribeLeague(ctx context.Context, deviceID string, league string, kinds []string, setKinds bool) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        INSERT INTO league_subscriptions (device_id, league, kinds)
        VALUES ($1, $2, CASE WHEN $4 THEN $3::text[] END)
        ON CONFLICT (device_id, league) DO UPDATE
        SET kinds = CASE WHEN $4 THEN EXCLUDED.kinds ELSE league_subscriptions.kinds END`,
		deviceID, league, kindsArray(kinds), setKinds)
	return subscriptionError(err, "league_subscriptions_device_fkey")
}

// UnsubscribeLeague deixa de seguir uma liga
func UnsubscribeLeague(ctx context.Context, deviceID string, league string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`DELETE FROM league_subscriptions WHERE device_id = $1 AND league = $2`, deviceID, league)
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

// PushSubscriptions é tudo o que um aparelho segue
type PushSubscriptions struct {
	Matches []int64  `json:"matches"`
	Teams   []int64  `json:"teams"`
	Leagues []string `json:"leagues"`
	// Tipos de aviso das inscrições que não recebem todos (as que não aparecem, recebem)
	Kinds          SubscriptionKinds `json:"kinds"`
	AvailableKinds []string          `json:"available_kinds"`
}

type SubscriptionKinds struct {
	Matches map[string][]string `json:"matches"`
	Teams   map[string][]string `json:"teams"`
	Leagues map[string][]string `json:"leagues"`
}

func GetPushSubscriptions(ctx context.Context, deviceID string) (PushSubscriptions, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	subs := PushSubscriptions{
		Matches: []int64{}, Teams: []int64{}, Leagues: []string{},
		Kinds: SubscriptionKinds{
			Matches: map[string][]string{}, Teams: map[string][]string{}, Leagues: map[string][]string{},
		},
		AvailableKinds: PushKinds,
	}
	queries := []struct {
		query string
		add   func(key string)
		kinds map[string][]string
	}{
		{`SELECT match_id::text, kinds FROM match_subscriptions WHERE device_id = $1 ORDER BY match_id`,
			func(k string) { subs.Matches = append(subs.Matches, parseID(k)) }, subs.Kinds.Matches},
		{`SELECT team_api_id::text, kinds FROM team_subscriptions WHERE device_id = $1 ORDER BY team_api_id`,
			func(k string) { subs.Teams = append(subs.Teams, parseID(k)) }, subs.Kinds.Teams},
		{`SELECT league, kinds FROM league_subscriptions WHERE device_id = $1 ORDER BY league`,
			func(k string) { subs.Leagues = append(subs.Leagues, k) }, subs.Kinds.Leagues},
	}
	for _, q := range queries {
		rows, err := DB.QueryContext(ctx, q.query, deviceID)
		if err != nil {
			return subs, err
		}
		for rows.Next() {
			var key string
			var kinds pq.StringArray
			if err := rows.Scan(&key, &kinds); err != nil {
				rows.Close()
				return subs, err
			}
			q.add(key)
			if kinds != nil {
				q.kinds[key] = kinds
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return subs, err
		}
	}
	return subs, nil
}

func parseID(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
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

// GetPushTokensForMatch devolve os tokens de quem segue o jogo, um dos times ou a liga,
// sem repetir
// GetPushTargetsForMatch devolve os aparelhos (app e navegador) que devem receber um
// aviso desse tipo do jogo
func GetPushTargetsForMatch(ctx context.Context, matchID int64, kind string) ([]PushTarget, error) {
	if kind == "goal_cancelled" {
		kind = "goal" // o gol anulado vai para quem recebe os avisos de gol
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT DISTINCT `+pushTargetColumns+`
        FROM push_devices d
        WHERE d.device_id IN (
            -- Quem segue por mais de um caminho recebe se qualquer um pedir esse tipo
            SELECT device_id FROM match_subscriptions
            WHERE match_id = $1 AND (kinds IS NULL OR $2 = ANY(kinds))
            UNION
            SELECT ts.device_id
            FROM team_subscriptions ts
            JOIN matches m ON m.id = $1
            WHERE ts.team_api_id IN (m.api_home_team_id, m.api_away_team_id)
              AND (ts.kinds IS NULL OR $2 = ANY(ts.kinds))
            UNION
            SELECT ls.device_id
            FROM league_subscriptions ls
            JOIN matches m ON m.id = $1 AND m.league = ls.league
            WHERE ls.kinds IS NULL OR $2 = ANY(ls.kinds)
        )`, matchID, kind)
	if err != nil {
		return nil, err
	}
	return scanPushTargets(rows)
}

// GetPushTokensForMatch é o GetPushTargetsForMatch só com os tokens
func GetPushTokensForMatch(ctx context.Context, matchID int64, kind string) ([]string, error) {
	targets, err := GetPushTargetsForMatch(ctx, matchID, kind)
	if err != nil {
		return nil, err
	}
	var tokens []string
	for _, t := range targets {
		tokens = append(tokens, t.Token)
	}
	return tokens, nil
}

// PendingPush é um aviso da fila com o que o texto da notificação precisa
type PendingPush struct {
	ID          int64
	MatchID     int64
	Kind        string // lineup | start | goal | goal_cancelled | red_card | end
	HomeScore   int
	AwayScore   int
	GoalSide    string // gol e gol anulado: home | away | both; cartão vermelho: home | away | "" (lado desconhecido)
	HomePenalty *int
	AwayPenalty *int
	League      string
	HomeTeam    string
	AwayTeam    string
	MatchDate   time.Time // zero se o jogo não tem data
	ESPNMatchID int64     // 0 enquanto o jogo não tem vínculo com a ESPN
	ESPNHomeID  int64
	ESPNAwayID  int64
	Scorer      string // autor do gol (quando a ESPN já informou) ou o expulso
	Minute      string // ex: "23'", "90'+3'"
	OwnGoal     bool
}

// GetPendingPushes devolve os avisos ainda não enviados, do mais antigo para o mais novo.
// O de gol anulado só sai depois de cancelDelay na fila: se o placar voltar antes, era
// oscilação da fonte, e o trigger o retira (migration 027). Ele leva o placar de agora,
// porque outro gol pode ter saído enquanto esperava.
func GetPendingPushes(ctx context.Context, limit int, cancelDelay time.Duration) ([]PendingPush, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT o.id, o.match_id, o.kind,
               CASE WHEN o.kind = 'goal_cancelled' THEN COALESCE(m.home_score, 0) ELSE o.home_score END,
               CASE WHEN o.kind = 'goal_cancelled' THEN COALESCE(m.away_score, 0) ELSE o.away_score END,
               o.goal_side,
               m.home_penalty, m.away_penalty,
               COALESCE(m.league, ''),
               COALESCE(NULLIF(th.short, ''), th.name, ''),
               COALESCE(NULLIF(ta.short, ''), ta.name, ''),
               m.match_date AT TIME ZONE 'UTC',
               COALESCE(e.espn_match_id, 0),
               COALESCE(th.espn_team_id, 0),
               COALESCE(ta.espn_team_id, 0),
               COALESCE(NULLIF(o.player, ''), gol.player_name, ''),
               COALESCE(NULLIF(o.minute, ''), gol.minute, ''),
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
          AND (o.kind <> 'goal_cancelled' OR o.created_at <= now() - $2 * INTERVAL '1 second')
        ORDER BY o.id
        LIMIT $1`, limit, cancelDelay.Seconds())
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

// enqueueRedCardPushes anota um aviso para cada cartão vermelho novo do jogo. Os eventos
// vêm da ESPN em ordem; match_push_state.reds_notified guarda quantos já foram avisados.
// Só para jogo rolando ou que acabou há pouco: a carga de jogos antigos não vira aviso.
func enqueueRedCardPushes(ctx context.Context, tx *sql.Tx, match models.ESPNMatchDB, events []models.ESPNEventDB, lineups []models.ESPNLineupDB) error {
	var reds []models.ESPNEventDB
	for _, e := range events {
		if e.EventType == "Red Card" {
			reds = append(reds, e)
		}
	}
	if len(reds) == 0 || !recentForPush(match, time.Now()) {
		return nil
	}

	var matchID int64
	var notified int
	err := tx.QueryRowContext(ctx, `
        SELECT m.id, COALESCE(s.reds_notified, 0)
        FROM matches m
        JOIN teams th ON th.api_id = m.api_home_team_id
        JOIN teams ta ON ta.api_id = m.api_away_team_id
        LEFT JOIN match_push_state s ON s.match_id = m.id
        WHERE th.espn_team_id = $1
          AND ta.espn_team_id = $2
          AND m.match_date::date = NULLIF($3, '')::timestamp::date
          AND push_match_has_followers(m.id, m.api_home_team_id, m.api_away_team_id)
        LIMIT 1`, match.ESPNHomeTeamID, match.ESPNAwayTeamID, match.MatchDate).Scan(&matchID, &notified)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // ninguém segue o jogo (ou ele ainda não tem vínculo)
	}
	if err != nil || len(reds) <= notified {
		return err
	}

	// O time do evento às vezes vem vazio: o lado sai da escalação do jogador
	playerTeam := map[string]int64{}
	for _, l := range lineups {
		playerTeam[l.PlayerName] = l.ESPNTeamID
	}
	for _, e := range reds[notified:] {
		team := e.ESPNTeamID
		if team == 0 {
			team = playerTeam[e.PlayerName]
		}
		side := ""
		switch team {
		case match.ESPNHomeTeamID:
			side = "home"
		case match.ESPNAwayTeamID:
			side = "away"
		}
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO push_outbox (match_id, kind, home_score, away_score, goal_side, player, minute)
            SELECT id, 'red_card', COALESCE(home_score, 0), COALESCE(away_score, 0), $2, $3, $4
            FROM matches WHERE id = $1`, matchID, side, e.PlayerName, e.Minute); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
        INSERT INTO match_push_state (match_id, reds_notified) VALUES ($1, $2)
        ON CONFLICT (match_id) DO UPDATE SET reds_notified = EXCLUDED.reds_notified`, matchID, len(reds))
	return err
}

// recentForPush: jogo rolando, ou encerrado com início nas últimas 4 horas (o cartão dos
// acréscimos pode chegar só junto com o apito final)
func recentForPush(match models.ESPNMatchDB, now time.Time) bool {
	if match.Status == "in" {
		return true
	}
	if match.Status != "post" {
		return false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z", "2006-01-02 15:04:05"} {
		if start, err := time.Parse(layout, match.MatchDate); err == nil {
			return now.Sub(start) < 4*time.Hour
		}
	}
	return false
}
