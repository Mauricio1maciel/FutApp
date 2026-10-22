package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"time"

	"github.com/lib/pq"
)

// teamMatchesSelect é a lista de jogos com nomes, escudos, a liga e o vínculo com a ESPN,
// usada pelos jogos de um time, pelo confronto direto e pelos jogos de quem a pessoa segue
const teamMatchesSelect = `
    SELECT 
        m.id,
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
        m.home_penalty,              
        m.away_penalty,
        COALESCE(m.match_date::TEXT, ''), 
        COALESCE(m.status, ''),
        COALESCE(m.stage, ''),       
        COALESCE(m.group_name, ''),
        COALESCE(m.winner, ''),
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
`

// GetMatchesByTeamID devolve os jogos do time na temporada atual de cada competição em
// que ele joga. O banco também guarda temporadas antigas (confronto direto, histórico):
// sem o filtro, a tela do time receberia centenas de jogos de anos atrás.
func GetMatchesByTeamID(ctx context.Context, teamID int64, rounds []int64) ([]models.Match, error) {
	where := `
    WHERE (m.api_home_team_id = $1 OR m.api_away_team_id = $1)
      AND m.season = (SELECT max(m2.season) FROM matches m2
                      WHERE m2.league = m.league AND COALESCE(m2.season, '') <> '')`
	args := []interface{}{teamID}
	if len(rounds) > 0 {
		args = append(args, pq.Array(rounds))
		where += ` AND COALESCE(m.round, 0) = ANY($2)`
	}
	return queryMatches(ctx, "GetMatchesByTeamID", where+` ORDER BY m.match_date ASC`, args...)
}

// GetTeamMatchesInSeason devolve os jogos do time numa temporada, em todas as competições
func GetTeamMatchesInSeason(ctx context.Context, teamID int64, season string) ([]models.Match, error) {
	where := `
    WHERE (m.api_home_team_id = $1 OR m.api_away_team_id = $1) AND m.season = $2
    ORDER BY m.match_date ASC`
	return queryMatches(ctx, "GetTeamMatchesInSeason", where, teamID, season)
}

// GetHeadToHead devolve os jogos já encerrados entre os dois times, em qualquer
// competição e temporada, do mais recente para o mais antigo
func GetHeadToHead(ctx context.Context, team1, team2 int64) ([]models.Match, error) {
	where := `
    WHERE ((m.api_home_team_id = $1 AND m.api_away_team_id = $2)
        OR (m.api_home_team_id = $2 AND m.api_away_team_id = $1))
      AND m.status = 'FINISHED'
    ORDER BY m.match_date DESC`
	return queryMatches(ctx, "GetHeadToHead", where, team1, team2)
}

// GetFollowedMatches devolve, numa lista só, os jogos dos times, ligas e jogos que a pessoa
// segue: os de [from, to) e os que estão em andamento agora (um jogo que começou ontem à
// noite ainda aparece). Um jogo que entra por dois caminhos (o time e a liga) vem uma vez.
func GetFollowedMatches(ctx context.Context, teams []int64, leagues []string, matchIDs []int64, from, to time.Time) ([]models.Match, error) {
	if teams == nil {
		teams = []int64{}
	}
	if leagues == nil {
		leagues = []string{}
	}
	if matchIDs == nil {
		matchIDs = []int64{}
	}
	where := `
    WHERE (m.api_home_team_id = ANY($1) OR m.api_away_team_id = ANY($1)
           OR m.league = ANY($2) OR m.id = ANY($3))
      AND ((m.match_date >= $4::timestamp AND m.match_date < $5::timestamp)
           OR (m.status IN ('IN_PLAY', 'PAUSED')
               AND m.match_date > (now() AT TIME ZONE 'UTC') - INTERVAL '6 hours'))
    ORDER BY m.match_date ASC, m.id`
	const layout = "2006-01-02 15:04:05" // match_date é UTC, sem fuso
	return queryMatches(ctx, "GetFollowedMatches", where, pq.Array(teams), pq.Array(leagues),
		pq.Array(matchIDs), from.UTC().Format(layout), to.UTC().Format(layout))
}

func queryMatches(ctx context.Context, name, where string, args ...interface{}) ([]models.Match, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, teamMatchesSelect+where, args...)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query %s: %v", name, err)
		return nil, err
	}
	defer rows.Close()

	matches := []models.Match{}
	for rows.Next() {
		var m models.Match
		err := rows.Scan(
			&m.MatchID,
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
			&m.HomePenalty,
			&m.AwayPenalty,
			&m.DateEvent,
			&m.Status,
			&m.Stage,
			&m.GroupName,
			&m.Winner,
			&m.HomeLogo,
			&m.AwayLogo,
		)
		if err != nil {
			utils.CustomLog("DB_ERRO", "Erro no Scan %s: %v", name, err)
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func GetCurrentRoundTeam(ctx context.Context, teamIDStr string) (int, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var round int

	query := `
        SELECT round FROM matches m
        WHERE (m.api_home_team_id = $1 OR m.api_away_team_id = $1) AND match_date <= NOW() 
        ORDER BY match_date DESC LIMIT 1
    `
	err := DB.QueryRowContext(ctx, query, teamIDStr).Scan(&round)

	if err != nil {
		queryFallback := `SELECT COALESCE(MAX(round), 1) FROM matches m WHERE m.api_home_team_id = $1 OR m.api_away_team_id = $1`
		errFallback := DB.QueryRowContext(ctx, queryFallback, teamIDStr).Scan(&round)
		if errFallback != nil {
			utils.CustomLog("DB_ERRO", "Erro no fallback GetCurrentRoundTeam: %v", errFallback)
			return 1, nil
		}
	}

	return round, nil
}
