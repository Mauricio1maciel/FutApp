package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"errors"
	"fmt"
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
// PushKinds são os tipos de aviso que a pessoa escolhe ao seguir, na ordem do jogo
var PushKinds = []string{"lineup", "start", "goal", "penalty", "red_card", "halftime", "end"}

// subscriptionKind é o tipo de inscrição que recebe cada aviso da fila: o gol anulado vai
// para quem recebe gols; os lances de pênalti, para quem recebe pênaltis
func subscriptionKind(kind string) string {
	switch kind {
	case "goal_cancelled":
		return "goal"
	case "penalty_awarded", "penalty_missed", "penalty_saved", "penalty_woodwork", "shootout":
		return "penalty"
	}
	return kind
}

// kindsArray: nil = NULL (todos os tipos)
func kindsArray(kinds []string) interface{} {
	if kinds == nil {
		return nil
	}
	return pq.Array(kinds)
}

// Owner é o dono de uma inscrição: um aparelho (convidado) ou uma conta. O que a conta
// segue vale em todos os aparelhos logados nela
type Owner struct {
	DeviceID string
	UserID   int64
}

func Device(deviceID string) Owner { return Owner{DeviceID: deviceID} }
func User(userID int64) Owner      { return Owner{UserID: userID} }

// column e value: a coluna e o valor do dono nas tabelas de inscrição
func (o Owner) column() string {
	if o.UserID != 0 {
		return "user_id"
	}
	return "device_id"
}

func (o Owner) value() interface{} {
	if o.UserID != 0 {
		return o.UserID
	}
	return o.DeviceID
}

// subscribe segue um alvo (jogo, time ou liga). setKinds = false mantém os tipos de aviso
// que a inscrição já tinha (ou todos, se é nova); setKinds = true grava kinds (nil = todos)
func subscribe(ctx context.Context, table, target string, owner Owner, value interface{}, kinds []string, setKinds bool) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	col := owner.column()
	_, err := DB.ExecContext(ctx, fmt.Sprintf(`
        INSERT INTO %[1]s (%[2]s, %[3]s, kinds)
        VALUES ($1, $2, CASE WHEN $4 THEN $3::text[] END)
        ON CONFLICT (%[2]s, %[3]s) DO UPDATE
        SET kinds = CASE WHEN $4 THEN EXCLUDED.kinds ELSE %[1]s.kinds END`, table, col, target),
		owner.value(), value, kindsArray(kinds), setKinds)
	return subscriptionError(err, table+"_device_fkey")
}

func unsubscribe(ctx context.Context, table, target string, owner Owner, value interface{}) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s = $1 AND %s = $2`, table, owner.column(), target),
		owner.value(), value)
	return err
}

// SubscribeMatch passa a seguir um jogo (repetir não dá erro)
func SubscribeMatch(ctx context.Context, owner Owner, matchID int64, kinds []string, setKinds bool) error {
	return subscribe(ctx, "match_subscriptions", "match_id", owner, matchID, kinds, setKinds)
}

// UnsubscribeMatch deixa de seguir um jogo
func UnsubscribeMatch(ctx context.Context, owner Owner, matchID int64) error {
	return unsubscribe(ctx, "match_subscriptions", "match_id", owner, matchID)
}

// SubscribeTeam passa a seguir todos os jogos de um time (repetir não dá erro)
func SubscribeTeam(ctx context.Context, owner Owner, teamID int64, kinds []string, setKinds bool) error {
	return subscribe(ctx, "team_subscriptions", "team_api_id", owner, teamID, kinds, setKinds)
}

// UnsubscribeTeam deixa de seguir um time
func UnsubscribeTeam(ctx context.Context, owner Owner, teamID int64) error {
	return unsubscribe(ctx, "team_subscriptions", "team_api_id", owner, teamID)
}

// SubscribeLeague passa a seguir todos os jogos de uma liga (repetir não dá erro)
func SubscribeLeague(ctx context.Context, owner Owner, league string, kinds []string, setKinds bool) error {
	return subscribe(ctx, "league_subscriptions", "league", owner, league, kinds, setKinds)
}

// UnsubscribeLeague deixa de seguir uma liga
func UnsubscribeLeague(ctx context.Context, owner Owner, league string) error {
	return unsubscribe(ctx, "league_subscriptions", "league", owner, league)
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

// PushSubscriptions é tudo o que um aparelho (ou uma conta) segue
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

func GetPushSubscriptions(ctx context.Context, owner Owner) (PushSubscriptions, error) {
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
		{`SELECT match_id::text, kinds FROM match_subscriptions WHERE ` + owner.column() + ` = $1 ORDER BY match_id`,
			func(k string) { subs.Matches = append(subs.Matches, parseID(k)) }, subs.Kinds.Matches},
		{`SELECT team_api_id::text, kinds FROM team_subscriptions WHERE ` + owner.column() + ` = $1 ORDER BY team_api_id`,
			func(k string) { subs.Teams = append(subs.Teams, parseID(k)) }, subs.Kinds.Teams},
		{`SELECT league, kinds FROM league_subscriptions WHERE ` + owner.column() + ` = $1 ORDER BY league`,
			func(k string) { subs.Leagues = append(subs.Leagues, k) }, subs.Kinds.Leagues},
	}
	for _, q := range queries {
		rows, err := DB.QueryContext(ctx, q.query, owner.value())
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
	kind = subscriptionKind(kind)
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT DISTINCT `+pushTargetColumns+`
        FROM push_devices d
        JOIN (
            -- Quem segue por mais de um caminho recebe se qualquer um pedir esse tipo. A
            -- inscrição é de um aparelho (convidado) ou de uma conta (todos os aparelhos dela)
            SELECT device_id, user_id FROM match_subscriptions
            WHERE match_id = $1 AND (kinds IS NULL OR $2 = ANY(kinds))
            UNION
            SELECT ts.device_id, ts.user_id
            FROM team_subscriptions ts
            JOIN matches m ON m.id = $1
            WHERE ts.team_api_id IN (m.api_home_team_id, m.api_away_team_id)
              AND (ts.kinds IS NULL OR $2 = ANY(ts.kinds))
            UNION
            SELECT ls.device_id, ls.user_id
            FROM league_subscriptions ls
            JOIN matches m ON m.id = $1 AND m.league = ls.league
            WHERE ls.kinds IS NULL OR $2 = ANY(ls.kinds)
        ) s ON s.device_id = d.device_id OR s.user_id = d.user_id`, matchID, kind)
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
	Kind        string // lineup | start | goal | goal_cancelled | penalty_* | shootout | red_card | halftime | end
	HomeScore   int
	AwayScore   int
	GoalSide    string // gol e gol anulado: home | away | both; cartão e pênalti: home | away | "" (lado desconhecido)
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
// Lances que viram aviso de pênalti (inscrição "penalty"), com o tipo na fila. O gol de
// pênalti já sai como gol. O time do lance é quem ganhou o pênalti (VAR) ou quem bateu
var penaltyEventKinds = map[string]string{
	"VAR - Penalty Awarded":  "penalty_awarded",
	"Penalty - Missed":       "penalty_missed",
	"Penalty - Saved":        "penalty_saved",
	"Penalty - Hit Woodwork": "penalty_woodwork",
}

type eventPush struct {
	kind  string
	event models.ESPNEventDB
}

// enqueueEventPushes anota os avisos que saem dos lances da ESPN: cartão vermelho,
// pênaltis (marcado pelo VAR, perdido, defendido, na trave e o começo da disputa) e o
// intervalo. O já avisado de cada jogo fica em match_push_state: o worker grava os mesmos
// lances a cada minuto, e cada um avisa uma vez só.
//
// Depois que a disputa de pênaltis começa, as cobranças não avisam uma a uma (seriam dez
// avisos em poucos minutos): o "Fim de jogo" traz o placar dos pênaltis.
func enqueueEventPushes(ctx context.Context, tx *sql.Tx, match models.ESPNMatchDB, events []models.ESPNEventDB, lineups []models.ESPNLineupDB) error {
	var reds, penalties []eventPush
	halftime, secondHalf, shootout := false, false, false
	for _, e := range events {
		switch {
		case e.EventType == "Red Card":
			reds = append(reds, eventPush{"red_card", e})
		case e.EventType == "Start Shootout" && !shootout:
			penalties = append(penalties, eventPush{"shootout", e})
			shootout = true
		case penaltyEventKinds[e.EventType] != "" && !shootout:
			penalties = append(penalties, eventPush{penaltyEventKinds[e.EventType], e})
		case e.EventType == "Halftime":
			halftime = true
		case e.EventType == "Start 2nd Half":
			secondHalf = true
		}
	}
	if (len(reds) == 0 && len(penalties) == 0 && !halftime) || !recentForPush(match, time.Now()) {
		return nil
	}

	var matchID int64
	var redsNotified, penaltiesNotified int
	var halftimeNotified bool
	err := tx.QueryRowContext(ctx, `
        SELECT m.id, COALESCE(s.reds_notified, 0), COALESCE(s.penalties_notified, 0),
               COALESCE(s.halftime_notified, false)
        FROM matches m
        JOIN teams th ON th.api_id = m.api_home_team_id
        JOIN teams ta ON ta.api_id = m.api_away_team_id
        LEFT JOIN match_push_state s ON s.match_id = m.id
        WHERE th.espn_team_id = $1
          AND ta.espn_team_id = $2
          AND m.match_date::date = NULLIF($3, '')::timestamp::date
          AND push_match_has_followers(m.id, m.api_home_team_id, m.api_away_team_id)
        LIMIT 1`, match.ESPNHomeTeamID, match.ESPNAwayTeamID, match.MatchDate).
		Scan(&matchID, &redsNotified, &penaltiesNotified, &halftimeNotified)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // ninguém segue o jogo (ou ele ainda não tem vínculo)
	}
	if err != nil {
		return err
	}

	// O time do lance às vezes vem vazio: o lado sai da escalação do jogador
	playerTeam := map[string]int64{}
	for _, l := range lineups {
		playerTeam[l.PlayerName] = l.ESPNTeamID
	}
	enqueue := func(p eventPush) error {
		team := p.event.ESPNTeamID
		if team == 0 {
			team = playerTeam[p.event.PlayerName]
		}
		side := ""
		switch {
		case p.kind == "shootout" || p.kind == "halftime":
		case team == match.ESPNHomeTeamID:
			side = "home"
		case team == match.ESPNAwayTeamID:
			side = "away"
		}
		_, err := tx.ExecContext(ctx, `
            INSERT INTO push_outbox (match_id, kind, home_score, away_score, goal_side, player, minute)
            SELECT id, $2, COALESCE(home_score, 0), COALESCE(away_score, 0), $3, $4, $5
            FROM matches WHERE id = $1`, matchID, p.kind, side, p.event.PlayerName, p.event.Minute)
		return err
	}

	for _, list := range []struct {
		pushes   []eventPush
		notified int
	}{{reds, redsNotified}, {penalties, penaltiesNotified}} {
		for _, p := range list.pushes[min(list.notified, len(list.pushes)):] {
			if err := enqueue(p); err != nil {
				return err
			}
		}
	}
	// O intervalo só avisa enquanto o 2º tempo não começou: o worker que subiu no meio do
	// jogo não avisa um intervalo que já passou
	if halftime && !halftimeNotified && !secondHalf {
		if err := enqueue(eventPush{kind: "halftime"}); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO match_push_state (match_id, reds_notified, penalties_notified, halftime_notified)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (match_id) DO UPDATE
        SET reds_notified = GREATEST(match_push_state.reds_notified, EXCLUDED.reds_notified),
            penalties_notified = GREATEST(match_push_state.penalties_notified, EXCLUDED.penalties_notified),
            halftime_notified = match_push_state.halftime_notified OR EXCLUDED.halftime_notified`,
		matchID, max(redsNotified, len(reds)), max(penaltiesNotified, len(penalties)), halftimeNotified || halftime)
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
