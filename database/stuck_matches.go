package database

import (
	"context"
	"database/sql"
	"time"
)

// StuckMatch é um jogo que já devia ter acabado e continua marcado ou em andamento, com o
// que a ESPN tem dele
type StuckMatch struct {
	MatchID    int64
	League     string
	HomeTeam   string
	AwayTeam   string
	MatchDate  time.Time // UTC
	Status     string
	ESPNStatus string // pre | in | post; "" quando a ESPN não tem o jogo
	ESPNHome   string // placar na ESPN
	ESPNAway   string
	ESPNDay    string // o dia do jogo no horário de Brasília (YYYYMMDD), como o placar da ESPN pede
}

// GetStuckMatches devolve os jogos que começaram há mais de 4 horas (nem prorrogação e
// pênaltis passam disso) e continuam marcados ou em andamento. Olha só os últimos 3 dias:
// jogo mais antigo com status velho (ex: o mata-mata da Champions 2025-26) é coisa
// conhecida, não sinal de fonte parada. Jogo com time ainda indefinido fica de fora.
func GetStuckMatches(ctx context.Context) ([]StuckMatch, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT m.id, m.league, COALESCE(th.name, '?'), COALESCE(ta.name, '?'), m.match_date AT TIME ZONE 'UTC', m.status,
               COALESCE(e.status, ''), COALESCE(e.home_score, ''), COALESCE(e.away_score, ''),
               to_char(m.match_date AT TIME ZONE 'UTC' AT TIME ZONE 'America/Sao_Paulo', 'YYYYMMDD')
        FROM matches m
        JOIN leagues l ON l.code_api = m.league
        LEFT JOIN teams th ON th.api_id = m.api_home_team_id
        LEFT JOIN teams ta ON ta.api_id = m.api_away_team_id
        LEFT JOIN LATERAL (
            -- Liga só da ESPN: o jogo foi criado a partir dela (id_event = ID da ESPN). As
            -- outras: o mesmo vínculo do resto da API, pelos times e pela data
            SELECT e.status, e.home_score, e.away_score
            FROM espn_matches e
            WHERE (l.espn_only AND e.espn_match_id = m.id_event)
               OR (NOT l.espn_only
                   AND e.espn_home_team_id = th.espn_team_id
                   AND e.espn_away_team_id = ta.espn_team_id
                   AND e.match_date::date = m.match_date::date)
            ORDER BY e.espn_match_id DESC
            LIMIT 1
        ) e ON true
        WHERE m.status IN ('SCHEDULED', 'TIMED', 'IN_PLAY', 'PAUSED')
          AND m.match_date < (now() AT TIME ZONE 'UTC') - INTERVAL '4 hours'
          AND m.match_date > (now() AT TIME ZONE 'UTC') - INTERVAL '3 days'
          AND COALESCE(m.api_home_team_id, 0) <> 0 AND COALESCE(m.api_away_team_id, 0) <> 0
        ORDER BY m.match_date, m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stuck []StuckMatch
	for rows.Next() {
		var s StuckMatch
		var date sql.NullTime
		if err := rows.Scan(&s.MatchID, &s.League, &s.HomeTeam, &s.AwayTeam, &date, &s.Status,
			&s.ESPNStatus, &s.ESPNHome, &s.ESPNAway, &s.ESPNDay); err != nil {
			return nil, err
		}
		s.MatchDate = date.Time
		stuck = append(stuck, s)
	}
	return stuck, rows.Err()
}

// DeleteOldAlertStates apaga as marcas de alerta com o prefixo, paradas há mais que maxAge
func DeleteOldAlertStates(ctx context.Context, prefix string, maxAge time.Duration) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        DELETE FROM alert_state
        WHERE starts_with(key, $1) AND updated_at < now() - $2 * INTERVAL '1 second'`,
		prefix, maxAge.Seconds())
	return err
}
