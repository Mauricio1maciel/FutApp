package database

import (
	"App-Futebol/models"
	"context"
	"database/sql"
	"errors"
)

func GetCompetitionRule(ctx context.Context, league string, season string) (*models.CompetitionRule, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var rule models.CompetitionRule

	// Temporada sem regra cadastrada usa a mais recente anterior a ela
	row := DB.QueryRowContext(ctx, `
		SELECT season, libertadores, pre_libertadores, sul_americana, rebaixamento
		FROM competition_rules
		WHERE league = $1 AND season <= $2
		ORDER BY season DESC
		LIMIT 1
	`, league, season)

	err := row.Scan(
		&rule.Season,
		&rule.Libertadores,
		&rule.PreLibertadores,
		&rule.SulAmericana,
		&rule.Rebaixamento,
	)

	if err != nil {
		return nil, err
	}

	return &rule, nil
}

func GetWinnersBySeasonAndSeason(ctx context.Context, league string, season string) ([]models.Winner, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
		SELECT competition, team_name
		FROM competition_winners
		WHERE league = $1 AND season = $2
	`, league, season)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var winners []models.Winner

	for rows.Next() {
		var w models.Winner

		err := rows.Scan(&w.Competition, &w.TeamName)
		if err != nil {
			return nil, err
		}

		winners = append(winners, w)
	}

	return winners, nil
}

func GetTieBreakers(ctx context.Context, league string, season string) ([]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Temporada sem critérios cadastrados usa a mais recente anterior a ela
	rows, err := DB.QueryContext(ctx, `
		SELECT criterion
		FROM competition_tiebreakers
		WHERE league = $1
		  AND season = (
		      SELECT MAX(season) FROM competition_tiebreakers
		      WHERE league = $1 AND season <= $2
		  )
		ORDER BY priority
	`, league, season)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var criteria []string

	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		criteria = append(criteria, c)
	}

	return criteria, rows.Err()
}

// SetCompetitionWinner anota o título de um time na liga e temporada (ex: campeão da Copa
// do Brasil na classificação do Brasileirão). Só grava se o time está nessa liga naquela
// temporada; senão, apaga o que estivesse anotado para o título (campeão estrangeiro não
// dá vaga). Sem mudança, não escreve nada.
func SetCompetitionWinner(ctx context.Context, league, season, competition string, teamID int64) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name sql.NullString
	if err := tx.QueryRowContext(ctx, `
        SELECT t.name FROM teams t
        WHERE t.api_id = $1
          AND EXISTS (SELECT 1 FROM team_leagues tl
                      WHERE tl.team_api_id = t.api_id AND tl.league = $2 AND tl.season = $3)`,
		teamID, league, season).Scan(&name); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
        DELETE FROM competition_winners
        WHERE league = $1 AND season = $2 AND competition = $3 AND team_name IS DISTINCT FROM $4::varchar`,
		league, season, competition, name); err != nil {
		return err
	}
	if name.Valid {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO competition_winners (league, season, competition, team_name)
            SELECT $1::varchar, $2::varchar, $3::varchar, $4::varchar
            WHERE NOT EXISTS (SELECT 1 FROM competition_winners
                              WHERE league = $1::varchar AND season = $2::varchar AND competition = $3::varchar)`,
			league, season, competition, name.String); err != nil {
			return err
		}
	}
	return tx.Commit()
}
