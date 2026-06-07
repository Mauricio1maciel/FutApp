package database

import "App-Futebol/models"

func GetCompetitionRule(league string, season string) (*models.CompetitionRule, error) {

	var rule models.CompetitionRule

	row := DB.QueryRow(`
		SELECT season, libertadores, pre_libertadores, sul_americana, rebaixamento
		FROM competition_rules
		WHERE league = $1 AND season = $2
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
		SELECT league , season, competition, team_name
		FROM competition_winners
		WHERE league = $1 AND season = $2
	`, season, league)

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

	rows, err := DB.Query(`
		SELECT criterion
		FROM competition_tiebreakers
		WHERE league = $1 AND season = $2
		ORDER BY priority
	`, league, season)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var criteria []string

	for rows.Next() {
		var c string
		rows.Scan(&c)
		criteria = append(criteria, c)
	}

	return criteria, nil
}
