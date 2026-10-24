package database

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// CardMatch é um jogo encerrado do time, com os cartões dos jogadores dele
type CardMatch struct {
	MatchID  int64
	Date     time.Time
	Opponent string
	Stage    string // fase (ex: GROUP_STAGE, QUARTER_FINALS): há competição que zera os amarelos numa fase
	HasData  bool   // a ESPN tem os lances do jogo (sem eles, os cartões não entram na conta)
	Cards    []CardEvent
}

type CardEvent struct {
	Player   string
	PlayerID int64  // ID da ESPN, pela escalação (0 se o jogador não está nela)
	Type     string // Yellow Card | Red Card | VAR - Red Card Given | VAR - (Red) Card Upgrade
}

// NextMatch é o próximo jogo do time na competição (em andamento conta)
type NextMatch struct {
	MatchID  int64     `json:"match_id"`
	Date     time.Time `json:"date"`
	Opponent string    `json:"opponent"`
	Home     bool      `json:"home"`
	Stage    string    `json:"stage"`
}

// GetTeamCards devolve os jogos encerrados do time na competição, em ordem, com os cartões
// dos jogadores dele, e o próximo jogo (nil se não houver). As duas consultas vão ao banco
// ao mesmo tempo.
func GetTeamCards(ctx context.Context, league, season string, teamID int64) ([]CardMatch, *NextMatch, error) {
	var (
		matches    []CardMatch
		next       *NextMatch
		errM, errN error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() { defer wg.Done(); matches, errM = teamCardMatches(ctx, league, season, teamID) }()
	go func() { defer wg.Done(); next, errN = teamNextMatch(ctx, league, season, teamID) }()
	wg.Wait()
	return matches, next, errors.Join(errM, errN)
}

func teamCardMatches(ctx context.Context, league, season string, teamID int64) ([]CardMatch, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        WITH jogos AS (
            SELECT m.id, m.match_date, COALESCE(m.stage, '') AS stage,
                   CASE WHEN m.api_home_team_id = $3 THEN m.api_away_team_id ELSE m.api_home_team_id END AS opponent_id,
                   e.espn_match_id
            FROM matches m
            JOIN leagues l ON l.code_api = m.league
            LEFT JOIN teams th ON th.api_id = m.api_home_team_id
            LEFT JOIN teams ta ON ta.api_id = m.api_away_team_id
            LEFT JOIN LATERAL (
                -- Liga só da ESPN: o jogo foi criado a partir dela (id_event = ID da ESPN). As
                -- outras: o mesmo vínculo do resto da API, pelos times e pela data
                SELECT e.espn_match_id FROM espn_matches e
                WHERE (l.espn_only AND e.espn_match_id = m.id_event)
                   OR (NOT l.espn_only
                       AND e.espn_home_team_id = th.espn_team_id
                       AND e.espn_away_team_id = ta.espn_team_id
                       AND e.match_date::date = m.match_date::date)
                ORDER BY e.espn_match_id DESC
                LIMIT 1
            ) e ON true
            WHERE m.league = $1 AND m.season = $2 AND m.status = 'FINISHED'
              AND $3 IN (m.api_home_team_id, m.api_away_team_id)
        )
        SELECT j.id, j.match_date AT TIME ZONE 'UTC', COALESCE(op.name, ''), j.stage,
               EXISTS (SELECT 1 FROM espn_match_events x WHERE x.espn_match_id = j.espn_match_id),
               c.player_name, c.event_type, c.player_id
        FROM jogos j
        LEFT JOIN teams op ON op.api_id = j.opponent_id
        LEFT JOIN LATERAL (
            -- Cartões do time. O time do lance às vezes vem vazio: aí vale o da escalação
            SELECT ev.id, ev.player_name, ev.event_type, COALESCE(lu.espn_player_id, 0) AS player_id
            FROM espn_match_events ev
            LEFT JOIN LATERAL (
                SELECT lu.espn_player_id, lu.espn_team_id FROM espn_match_lineups lu
                WHERE lu.espn_match_id = ev.espn_match_id AND lu.player_name = ev.player_name
                LIMIT 1
            ) lu ON true
            WHERE ev.espn_match_id = j.espn_match_id
              AND ev.event_type IN ('Yellow Card', 'Red Card', 'VAR - Red Card Given', 'VAR - (Red) Card Upgrade')
              AND COALESCE(NULLIF(ev.espn_team_id, 0), lu.espn_team_id) = (SELECT espn_team_id FROM teams WHERE api_id = $3)
        ) c ON true
        ORDER BY j.match_date, j.id, c.id`, league, season, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []CardMatch
	for rows.Next() {
		var id int64
		var date time.Time
		var opponent, stage string
		var hasData bool
		var player, kind sql.NullString
		var playerID sql.NullInt64
		if err := rows.Scan(&id, &date, &opponent, &stage, &hasData, &player, &kind, &playerID); err != nil {
			return nil, err
		}
		if len(matches) == 0 || matches[len(matches)-1].MatchID != id {
			matches = append(matches, CardMatch{MatchID: id, Date: date, Opponent: opponent, Stage: stage, HasData: hasData})
		}
		if player.Valid {
			last := &matches[len(matches)-1]
			last.Cards = append(last.Cards, CardEvent{Player: player.String, PlayerID: playerID.Int64, Type: kind.String})
		}
	}
	return matches, rows.Err()
}

func teamNextMatch(ctx context.Context, league, season string, teamID int64) (*NextMatch, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Jogo adiado sem data nova fica de fora: ele tem a data antiga e passaria na frente
	var n NextMatch
	err := DB.QueryRowContext(ctx, `
        SELECT m.id, m.match_date AT TIME ZONE 'UTC', COALESCE(op.name, ''), m.api_home_team_id = $3,
               COALESCE(m.stage, '')
        FROM matches m
        LEFT JOIN teams op ON op.api_id = CASE WHEN m.api_home_team_id = $3 THEN m.api_away_team_id ELSE m.api_home_team_id END
        WHERE m.league = $1 AND m.season = $2 AND $3 IN (m.api_home_team_id, m.api_away_team_id)
          AND m.status IN ('SCHEDULED', 'TIMED', 'IN_PLAY', 'PAUSED')
          AND m.match_date > (now() AT TIME ZONE 'UTC') - INTERVAL '4 hours'
        ORDER BY m.match_date
        LIMIT 1`, league, season, teamID).Scan(&n.MatchID, &n.Date, &n.Opponent, &n.Home, &n.Stage)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}
