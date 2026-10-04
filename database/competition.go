package database

import "App-Futebol/models"

func GetCompetitionRule(league string, season string) (*models.CompetitionRule, error) {

	var rule models.CompetitionRule

	// Temporada sem regra cadastrada usa a mais recente anterior a ela
	row := DB.QueryRow(`
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

func GetWinnersBySeasonAndSeason(league string, season string) ([]models.Winner, error) {

	rows, err := DB.Query(`
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

func GetTieBreakers(league string, season string) ([]string, error) {

	// Temporada sem critérios cadastrados usa a mais recente anterior a ela
	rows, err := DB.Query(`
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
