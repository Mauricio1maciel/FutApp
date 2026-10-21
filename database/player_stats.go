package database

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
)

// counted: o jogo da ESPN existe na nossa base como encerrado. Jogo cancelado (ex: a
// football-data dá CANCELLED e a ESPN registra o W.O.) não entra nas estatísticas
const countedMatch = `EXISTS (
        SELECT 1 FROM matches m
        JOIN teams th ON th.api_id = m.api_home_team_id
        JOIN teams ta ON ta.api_id = m.api_away_team_id
        WHERE th.espn_team_id = e.espn_home_team_id AND ta.espn_team_id = e.espn_away_team_id
          AND m.match_date::date = e.match_date::date AND m.status = 'FINISHED')`

// PlayerAppearance é o jogador na escalação de um jogo encerrado (titular ou reserva)
type PlayerAppearance struct {
	ESPNMatchID  int64
	League       string
	Season       string
	MatchDate    string
	ESPNTeamID   int64
	PlayerName   string
	Starter      bool
	HomeESPNID   int64
	AwayESPNID   int64
	HomeScore    int
	AwayScore    int
	HomePenalty  *int
	AwayPenalty  *int
	PlayerTeamID int64 // api_id do time do jogador (0 se o time não tem cadastro)
}

// GetPlayerAppearances devolve as escalações do jogador (ID da ESPN) em jogos encerrados,
// do mais recente para o mais antigo. season vazio = todas as temporadas.
func GetPlayerAppearances(ctx context.Context, espnPlayerID int64, season string) ([]PlayerAppearance, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT DISTINCT ON (l.espn_match_id)
               l.espn_match_id, COALESCE(e.league, ''), COALESCE(e.season, ''),
               COALESCE(to_char(e.match_date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
               l.espn_team_id, l.player_name, COALESCE(l.is_starter, false),
               COALESCE(e.espn_home_team_id, 0), COALESCE(e.espn_away_team_id, 0),
               COALESCE(NULLIF(e.home_score, '')::int, 0), COALESCE(NULLIF(e.away_score, '')::int, 0),
               e.home_penalty, e.away_penalty, COALESCE(t.api_id, 0)
        FROM espn_match_lineups l
        JOIN espn_matches e ON e.espn_match_id = l.espn_match_id
        LEFT JOIN teams t ON t.espn_team_id = l.espn_team_id
        WHERE l.espn_player_id = $1 AND e.status = 'post' AND ($2 = '' OR e.season = $2)
          AND `+countedMatch+`
        ORDER BY l.espn_match_id, l.is_starter DESC`, espnPlayerID, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []PlayerAppearance
	for rows.Next() {
		var a PlayerAppearance
		var hp, ap sql.NullInt64
		if err := rows.Scan(&a.ESPNMatchID, &a.League, &a.Season, &a.MatchDate, &a.ESPNTeamID, &a.PlayerName,
			&a.Starter, &a.HomeESPNID, &a.AwayESPNID, &a.HomeScore, &a.AwayScore, &hp, &ap, &a.PlayerTeamID); err != nil {
			return nil, err
		}
		if hp.Valid && ap.Valid {
			h, w := int(hp.Int64), int(ap.Int64)
			a.HomePenalty, a.AwayPenalty = &h, &w
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// MatchEvent é um lance de jogo que conta nas estatísticas de jogador
type MatchEvent struct {
	ESPNMatchID int64
	Type        string
	Minute      string
	Player      string // em substituição: quem entrou
	Details     string // em substituição: quem saiu
}

// GetStatEvents devolve gols, cartões, substituições e o início da prorrogação dos jogos
func GetStatEvents(ctx context.Context, espnMatchIDs []int64) ([]MatchEvent, error) {
	if len(espnMatchIDs) == 0 {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT espn_match_id, event_type, COALESCE(minute, ''), COALESCE(player_name, ''), COALESCE(details, '')
        FROM espn_match_events
        WHERE espn_match_id = ANY($1)
          AND (event_type IN ('Goal', 'Penalty - Scored', 'Own Goal', 'Yellow Card', 'Red Card',
                              'Substitution', 'Start Extra Time')
               OR event_type LIKE 'Goal - %')
        ORDER BY id`, pq.Array(espnMatchIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []MatchEvent
	for rows.Next() {
		var e MatchEvent
		if err := rows.Scan(&e.ESPNMatchID, &e.Type, &e.Minute, &e.Player, &e.Details); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// TeamRef é o time para exibir (nome e escudo), pelo ID da ESPN
type TeamRef struct {
	ID    int64  `json:"id"` // api_id (0 se o time não tem cadastro)
	Name  string `json:"name"`
	Crest string `json:"crest_url"`
}

// GetTeamsByESPNIDs devolve nome e escudo dos times, indexados pelo ID da ESPN
func GetTeamsByESPNIDs(ctx context.Context, espnIDs []int64) (map[int64]TeamRef, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT espn_team_id, api_id, name, COALESCE(crest_url, '')
        FROM teams WHERE espn_team_id = ANY($1)`, pq.Array(espnIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := map[int64]TeamRef{}
	for rows.Next() {
		var espn int64
		var t TeamRef
		if err := rows.Scan(&espn, &t.ID, &t.Name, &t.Crest); err != nil {
			return nil, err
		}
		teams[espn] = t
	}
	return teams, rows.Err()
}

// GetLeagueNames devolve o nome de cada liga pelo código
func GetLeagueNames(ctx context.Context) (map[string]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `SELECT code_api, COALESCE(name, '') FROM leagues`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[string]string{}
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			return nil, err
		}
		names[code] = name
	}
	return names, rows.Err()
}

// TeamMatchStatistics são as estatísticas de um jogo encerrado, do lado do time
type TeamMatchStatistics struct {
	League string
	Season string
	Home   bool
	Raw    []byte // espn_matches.statistics ({"home": {...}, "away": {...}})
}

// GetTeamMatchStatistics devolve as estatísticas dos jogos encerrados do time (ID da ESPN)
func GetTeamMatchStatistics(ctx context.Context, espnTeamID int64) ([]TeamMatchStatistics, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT COALESCE(e.league, ''), COALESCE(e.season, ''), e.espn_home_team_id = $1, e.statistics
        FROM espn_matches e
        WHERE $1 IN (e.espn_home_team_id, e.espn_away_team_id) AND e.status = 'post'
          AND e.statistics IS NOT NULL AND `+countedMatch,
		espnTeamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TeamMatchStatistics
	for rows.Next() {
		var s TeamMatchStatistics
		if err := rows.Scan(&s.League, &s.Season, &s.Home, &s.Raw); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// TeamScorer é um artilheiro do time numa competição
type TeamScorer struct {
	League      string `json:"-"`
	Season      string `json:"-"`
	PlayerID    int64  `json:"player_id"`
	Name        string `json:"name"`
	HeadshotURL string `json:"headshot_url"`
	Goals       int    `json:"goals"`
}

// GetTeamScorers devolve quem fez gol pelo time (ID da ESPN) em cada competição e
// temporada, do que fez mais para o que fez menos. Gol contra não conta.
func GetTeamScorers(ctx context.Context, espnTeamID int64) ([]TeamScorer, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
        SELECT COALESCE(e.league, ''), COALESCE(e.season, ''), l.espn_player_id, l.player_name,
               COALESCE(MAX(p.headshot_url), ''), count(*)
        FROM espn_match_events ev
        JOIN espn_matches e ON e.espn_match_id = ev.espn_match_id AND e.status = 'post'
        JOIN espn_match_lineups l ON l.espn_match_id = ev.espn_match_id
                                 AND l.player_name = ev.player_name AND l.espn_team_id = $1
        LEFT JOIN espn_players p ON p.espn_id = l.espn_player_id
        WHERE (ev.event_type IN ('Goal', 'Penalty - Scored') OR ev.event_type LIKE 'Goal - %')
          AND COALESCE(l.espn_player_id, 0) <> 0
          AND `+countedMatch+`
        GROUP BY 1, 2, 3, 4
        ORDER BY 6 DESC, 4`, espnTeamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scorers []TeamScorer
	for rows.Next() {
		var s TeamScorer
		if err := rows.Scan(&s.League, &s.Season, &s.PlayerID, &s.Name, &s.HeadshotURL, &s.Goals); err != nil {
			return nil, err
		}
		scorers = append(scorers, s)
	}
	return scorers, rows.Err()
}
