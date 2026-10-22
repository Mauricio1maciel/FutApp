package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"log"
)

// MatchFilter escolhe quais jogos de uma liga e temporada devolver.
// Campo vazio (ou Round nil) não filtra. Date tem prioridade sobre Stage e Round.
type MatchFilter struct {
	Stage string // fase, ex: "SEMI_FINALS", "GROUP_STAGE"
	Round *int   // rodada; nil = todas
	Date  string // "YYYY-MM-DD", no horário de Brasília
}

func GetMatchesByLeague(ctx context.Context, league string, season string, f MatchFilter) ([]models.Match, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `
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
    WHERE m.league = $1 AND m.season = $2
    `
	args := []interface{}{league, season}

	if f.Date != "" {
		args = append(args, f.Date)
		query += fmt.Sprintf(` AND (m.match_date AT TIME ZONE 'UTC' AT TIME ZONE 'America/Sao_Paulo')::DATE = $%d::DATE`, len(args))
	} else {
		if f.Stage != "" {
			args = append(args, f.Stage)
			query += fmt.Sprintf(` AND m.stage = $%d`, len(args))
		}
		if f.Round != nil {
			args = append(args, *f.Round)
			query += fmt.Sprintf(` AND m.round = $%d`, len(args))
		}
	}

	query += ` ORDER BY m.match_date ASC`

	rows, err := DB.QueryContext(ctx, query, args...)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetMatchesByLeague: %v", err)
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
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
			utils.CustomLog("DB_ERRO", "Erro no Scan GetMatchesByLeague: %v", err)
			return nil, err
		}
		matches = append(matches, m)
	}

	if matches == nil {
		matches = []models.Match{}
	}
	return matches, nil
}

// fdRegresses: a football-data mandou um estado anterior ao que já temos para o mesmo
// horário (marcado < em andamento < encerrado). Ela às vezes fica parada: a Libertadores
// de 2026 parou de ser atualizada em 22/09, e a cada volta regravava "marcado, 0 x 0" em
// cima do resultado que a ESPN tinha trazido. Nesse caso fica o que já temos (placar,
// pênaltis, situação e vencedor). Adiamento, cancelamento e mudança de data valem sempre.
const fdRegresses = `(date_trunc('minute', matches.match_date) = date_trunc('minute', EXCLUDED.match_date)
          AND (CASE EXCLUDED.status WHEN 'FINISHED' THEN 2 WHEN 'IN_PLAY' THEN 1 WHEN 'PAUSED' THEN 1
                                    WHEN 'SCHEDULED' THEN 0 WHEN 'TIMED' THEN 0 ELSE 99 END)
            < (CASE matches.status WHEN 'FINISHED' THEN 2 WHEN 'IN_PLAY' THEN 1 WHEN 'PAUSED' THEN 1
                                    WHEN 'SCHEDULED' THEN 0 WHEN 'TIMED' THEN 0 ELSE -1 END))`

func SaveMatch(ctx context.Context,
	idEvent int64,
	league string,
	season string,
	round int,
	apiHomeTeamID int64,
	apiAwayTeamID int64,
	homeScore int,
	awayScore int,
	homePenalty *int,
	awayPenalty *int,
	date string,
	status string,
	stage string,
	groupName string,
	winner string,
) error {

	query := `
    INSERT INTO matches
    (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, home_penalty, away_penalty, match_date, status, stage, group_name, winner)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11, '')::TIMESTAMP,$12,$13,$14,$15)
    ON CONFLICT (id_event, league) 
    DO UPDATE SET
         home_score = CASE WHEN ` + fdRegresses + ` THEN matches.home_score ELSE EXCLUDED.home_score END,
         away_score = CASE WHEN ` + fdRegresses + ` THEN matches.away_score ELSE EXCLUDED.away_score END,
         home_penalty = CASE WHEN ` + fdRegresses + ` THEN matches.home_penalty ELSE EXCLUDED.home_penalty END,
         away_penalty = CASE WHEN ` + fdRegresses + ` THEN matches.away_penalty ELSE EXCLUDED.away_penalty END,
         match_date = EXCLUDED.match_date,
         season = EXCLUDED.season,
         status = CASE WHEN ` + fdRegresses + ` THEN matches.status ELSE EXCLUDED.status END,
         round = EXCLUDED.round,
         api_home_team_id = EXCLUDED.api_home_team_id,
         api_away_team_id = EXCLUDED.api_away_team_id,
         stage = EXCLUDED.stage,
         group_name = EXCLUDED.group_name,
		 winner = CASE WHEN ` + fdRegresses + ` THEN matches.winner ELSE EXCLUDED.winner END
    `

	_, err := DB.ExecContext(ctx,
		query,
		idEvent,
		league,
		season,
		round,
		apiHomeTeamID,
		apiAwayTeamID,
		homeScore,
		awayScore,
		homePenalty,
		awayPenalty,
		date,
		status,
		stage,
		groupName,
		winner,
	)

	if err != nil {
		log.Printf("Erro ao salvar jogo: %v", err)
	}

	return err
}

// GetCurrentStage devolve a fase e a rodada atuais de uma liga: as do próximo jogo
// (ou do que está rolando); se a temporada já acabou, as do último jogo.
//
// Jogos adiados e cancelados não contam. Um jogo "a jogar" com data de mais de um
// dia atrás também não: é status desatualizado (ex: mata-mata da CL 2025-26) e
// prenderia a fase atual no passado.
//
// Sem nenhum jogo na temporada, devolve fase vazia e rodada 0.
func GetCurrentStage(ctx context.Context, league string, season string) (string, int, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var stage string
	var round int
	err := DB.QueryRowContext(ctx, `
        SELECT stage, round FROM (
            (SELECT COALESCE(stage, '') AS stage, COALESCE(round, 0) AS round, 0 AS ordem
               FROM matches
              WHERE league = $1 AND season = $2
                AND status IN ('SCHEDULED', 'TIMED', 'IN_PLAY', 'PAUSED')
                AND match_date >= NOW() - INTERVAL '1 day'
              ORDER BY match_date ASC
              LIMIT 1)
            UNION ALL
            (SELECT COALESCE(stage, ''), COALESCE(round, 0), 1
               FROM matches
              WHERE league = $1 AND season = $2 AND match_date IS NOT NULL
              ORDER BY match_date DESC
              LIMIT 1)
        ) atual
        ORDER BY ordem
        LIMIT 1`, league, season).Scan(&stage, &round)

	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetCurrentStage: %v", err)
		return "", 0, err
	}
	return stage, round, nil
}

func GetLatestSeason(ctx context.Context, league string) string {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var season string
	// Sem o filtro, um único jogo com season NULL vem primeiro no DESC e derruba a consulta
	query := `SELECT season FROM matches WHERE league = $1 AND COALESCE(season, '') <> '' ORDER BY season DESC LIMIT 1`
	err := DB.QueryRowContext(ctx, query, league).Scan(&season)
	if err != nil {
		return "" // Sem jogos: quem chama decide (services.ResolveSeason usa a temporada de hoje)
	}
	return season
}

// UpdateMatchRounds grava rodadas calculadas (match_id → rodada), só onde mudou
func UpdateMatchRounds(ctx context.Context, rounds map[int64]int) error {
	if len(rounds) == 0 {
		return nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	ids := make([]int64, 0, len(rounds))
	values := make([]int64, 0, len(rounds))
	for id, r := range rounds {
		ids = append(ids, id)
		values = append(values, int64(r))
	}
	_, err := DB.ExecContext(ctx, `
        UPDATE matches m SET round = v.round
        FROM unnest($1::int[], $2::int[]) AS v(id, round)
        WHERE m.id = v.id AND m.round IS DISTINCT FROM v.round`,
		pq.Array(ids), pq.Array(values))
	return err
}

// SetMatchesStage grava a fase dos jogos (ex: os playoffs da Série B), só onde mudou
func SetMatchesStage(ctx context.Context, ids []int64, stage string) error {
	if len(ids) == 0 {
		return nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx, `
        UPDATE matches SET stage = $2
        WHERE id = ANY($1::int[]) AND stage IS DISTINCT FROM $2`, pq.Array(ids), stage)
	return err
}
