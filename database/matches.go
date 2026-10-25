package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"log"
	"strconv"
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
// cima do resultado que a ESPN tinha trazido. Adiamento, cancelamento e mudança de data
// valem sempre.
const fdRegresses = `(date_trunc('minute', matches.match_date) = date_trunc('minute', EXCLUDED.match_date)
          AND (CASE EXCLUDED.status WHEN 'FINISHED' THEN 2 WHEN 'IN_PLAY' THEN 1 WHEN 'PAUSED' THEN 1
                                    WHEN 'SCHEDULED' THEN 0 WHEN 'TIMED' THEN 0 ELSE 99 END)
            < (CASE matches.status WHEN 'FINISHED' THEN 2 WHEN 'IN_PLAY' THEN 1 WHEN 'PAUSED' THEN 1
                                    WHEN 'SCHEDULED' THEN 0 WHEN 'TIMED' THEN 0 ELSE -1 END))`

// fdBehindESPN: o jogo está em andamento aqui e na football-data, e a ESPN o acompanha ao
// vivo. O placar ao vivo da football-data chega atrasado: gravado por cima do da ESPN, ele
// fazia o placar ir e voltar (1 x 0, 0 x 0, 1 x 0) e trazia de volta um gol que o VAR
// tinha anulado. O resultado final da football-data continua valendo.
const fdBehindESPN = `(matches.status IN ('IN_PLAY', 'PAUSED') AND EXCLUDED.status IN ('IN_PLAY', 'PAUSED')
          AND EXISTS (SELECT 1 FROM teams th, teams ta, espn_matches e
                      WHERE th.api_id = matches.api_home_team_id AND ta.api_id = matches.api_away_team_id
                        AND e.espn_home_team_id = th.espn_team_id AND e.espn_away_team_id = ta.espn_team_id
                        AND e.match_date::date = matches.match_date::date AND e.status = 'in'))`

// fdKeepsOurs: nos dois casos acima, e com o resultado travado (W.O., decisão do tribunal:
// migration 032), fica o que já temos (placar, pênaltis, situação e vencedor)
const fdKeepsOurs = `(matches.result_locked OR ` + fdRegresses + ` OR ` + fdBehindESPN + `)`

// matchUpsertConflict é o que acontece quando o jogo já existe, igual para um jogo
// (SaveMatch) e para a liga inteira (SaveMatches). O WHERE pula a gravação quando nada
// mudaria: a football-data manda a temporada inteira a cada volta, e regravar os mesmos
// valores eram ~270 mil atualizações por dia em matches (8 milhões em um mês, para 11 mil
// jogos)
const matchUpsertConflict = `
    ON CONFLICT (id_event, league)
    DO UPDATE SET
         home_score = CASE WHEN ` + fdKeepsOurs + ` THEN matches.home_score ELSE EXCLUDED.home_score END,
         away_score = CASE WHEN ` + fdKeepsOurs + ` THEN matches.away_score ELSE EXCLUDED.away_score END,
         home_penalty = CASE WHEN ` + fdKeepsOurs + ` THEN matches.home_penalty ELSE EXCLUDED.home_penalty END,
         away_penalty = CASE WHEN ` + fdKeepsOurs + ` THEN matches.away_penalty ELSE EXCLUDED.away_penalty END,
         match_date = EXCLUDED.match_date,
         season = EXCLUDED.season,
         status = CASE WHEN ` + fdKeepsOurs + ` THEN matches.status ELSE EXCLUDED.status END,
         round = EXCLUDED.round,
         api_home_team_id = EXCLUDED.api_home_team_id,
         api_away_team_id = EXCLUDED.api_away_team_id,
         stage = EXCLUDED.stage,
         group_name = EXCLUDED.group_name,
         winner = CASE WHEN ` + fdKeepsOurs + ` THEN matches.winner ELSE EXCLUDED.winner END
    WHERE (matches.match_date, matches.season, matches.round, matches.api_home_team_id,
           matches.api_away_team_id, matches.stage, matches.group_name)
          IS DISTINCT FROM
          (EXCLUDED.match_date, EXCLUDED.season, EXCLUDED.round, EXCLUDED.api_home_team_id,
           EXCLUDED.api_away_team_id, EXCLUDED.stage, EXCLUDED.group_name)
       OR (NOT COALESCE(` + fdKeepsOurs + `, false)
           AND (matches.home_score, matches.away_score, matches.home_penalty, matches.away_penalty,
                matches.status, matches.winner)
               IS DISTINCT FROM
               (EXCLUDED.home_score, EXCLUDED.away_score, EXCLUDED.home_penalty, EXCLUDED.away_penalty,
                EXCLUDED.status, EXCLUDED.winner))`

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
	_, err := saveMatch(ctx, MatchRow{IDEvent: idEvent, League: league, Season: season, Round: round,
		HomeTeamID: apiHomeTeamID, AwayTeamID: apiAwayTeamID, HomeScore: homeScore, AwayScore: awayScore,
		HomePenalty: homePenalty, AwayPenalty: awayPenalty, Date: date, Status: status, Stage: stage,
		Group: groupName, Winner: winner})
	return err
}

// saveMatch grava um jogo e diz se ele mudou (1) ou ficou igual (0)
func saveMatch(ctx context.Context, r MatchRow) (int64, error) {
	res, err := DB.ExecContext(ctx, `
    INSERT INTO matches
    (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, home_penalty, away_penalty, match_date, status, stage, group_name, winner)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11, '')::TIMESTAMP,$12,$13,$14,$15)`+matchUpsertConflict,
		r.IDEvent, r.League, r.Season, r.Round, r.HomeTeamID, r.AwayTeamID, r.HomeScore, r.AwayScore,
		r.HomePenalty, r.AwayPenalty, r.Date, r.Status, r.Stage, r.Group, r.Winner)
	if err != nil {
		log.Printf("Erro ao salvar jogo: %v", err)
		return 0, err
	}
	return res.RowsAffected()
}

// MatchRow é um jogo da football-data para o SaveMatches
type MatchRow struct {
	IDEvent                  int64
	League, Season           string
	Round                    int
	HomeTeamID, AwayTeamID   int64
	HomeScore, AwayScore     int
	HomePenalty, AwayPenalty *int
	Date                     string // ISO; vazio = sem data
	Status, Stage, Group     string
	Winner                   string
}

// SaveMatches grava a lista inteira num comando só: do Orange Pi ao banco, cada comando é
// uma ida e volta, e eram ~380 por liga a cada volta da football-data. Devolve quantos
// jogos mudaram (os iguais não são regravados). Se o comando falhar, grava um por um, para
// um jogo com problema não travar os outros.
func SaveMatches(ctx context.Context, rows []MatchRow) (changed int64, err error) {
	// O mesmo jogo duas vezes no mesmo comando é erro no Postgres: fica a última versão
	index := make(map[string]int, len(rows))
	unique := make([]MatchRow, 0, len(rows))
	for _, r := range rows {
		key := r.League + "|" + strconv.FormatInt(r.IDEvent, 10)
		if i, ok := index[key]; ok {
			unique[i] = r
			continue
		}
		index[key] = len(unique)
		unique = append(unique, r)
	}
	if len(unique) == 0 {
		return 0, nil
	}

	n := len(unique)
	ids, homes, aways := make([]int64, n), make([]int64, n), make([]int64, n)
	rounds, homeScores, awayScores := make([]int64, n), make([]int64, n), make([]int64, n)
	homePens, awayPens := make([]sql.NullInt64, n), make([]sql.NullInt64, n)
	dates := make([]sql.NullString, n)
	leagues, seasons, statuses := make([]string, n), make([]string, n), make([]string, n)
	stages, groups, winners := make([]string, n), make([]string, n), make([]string, n)
	for i, r := range unique {
		ids[i], leagues[i], seasons[i], rounds[i] = r.IDEvent, r.League, r.Season, int64(r.Round)
		homes[i], aways[i] = r.HomeTeamID, r.AwayTeamID
		homeScores[i], awayScores[i] = int64(r.HomeScore), int64(r.AwayScore)
		if r.HomePenalty != nil {
			homePens[i] = sql.NullInt64{Int64: int64(*r.HomePenalty), Valid: true}
		}
		if r.AwayPenalty != nil {
			awayPens[i] = sql.NullInt64{Int64: int64(*r.AwayPenalty), Valid: true}
		}
		dates[i] = sql.NullString{String: r.Date, Valid: r.Date != ""}
		statuses[i], stages[i], groups[i], winners[i] = r.Status, r.Stage, r.Group, r.Winner
	}

	batchCtx, cancel := withTxTimeout(ctx)
	defer cancel()
	res, err := DB.ExecContext(batchCtx, `
    INSERT INTO matches
    (id_event, league, season, round, api_home_team_id, api_away_team_id, home_score, away_score, home_penalty, away_penalty, match_date, status, stage, group_name, winner)
    SELECT * FROM unnest($1::bigint[], $2::varchar[], $3::varchar[], $4::int[], $5::bigint[], $6::bigint[],
                         $7::int[], $8::int[], $9::int[], $10::int[], $11::timestamp[], $12::varchar[],
                         $13::varchar[], $14::varchar[], $15::varchar[])`+matchUpsertConflict,
		pq.Array(ids), pq.Array(leagues), pq.Array(seasons), pq.Array(rounds), pq.Array(homes), pq.Array(aways),
		pq.Array(homeScores), pq.Array(awayScores), pq.Array(homePens), pq.Array(awayPens), pq.Array(dates),
		pq.Array(statuses), pq.Array(stages), pq.Array(groups), pq.Array(winners))
	if err == nil {
		return res.RowsAffected()
	}

	utils.CustomLog("DB_ERRO", "Gravação dos %d jogos num comando só falhou (%v): gravando um por um", n, err)
	var firstErr error
	for _, r := range unique {
		c, e := saveMatch(ctx, r)
		changed += c
		if e != nil && firstErr == nil {
			firstErr = e
		}
	}
	return changed, firstErr
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
