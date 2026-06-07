package database

import (
	"App-Futebol/models"
)

func ClearStandings(league string, season string) error {
	_, err := DB.Exec(
		"DELETE FROM standings WHERE league = $1 AND season = $2",
		league,
		season,
	)

	return err
}

func SaveStandings(league string, season string, standings []models.Standing) error {

	for _, s := range standings {

		_, err := DB.Exec(`
            INSERT INTO standings
            (league, position, team_id, played, wins, draws, losses, goals_for, goals_against, goal_diff, points, zone, season)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
        `,
			league,
			s.Position,
			s.TeamID,
			s.Played,
			s.Wins,
			s.Draws,
			s.Losses,
			s.GoalsFor,
			s.GoalsAgainst,
			s.GoalDiff,
			s.Points,
			s.Zone,
			season,
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func GetStandingsByLeague(league string, season string) ([]models.Standing, error) {

	rows, err := DB.Query(`
    SELECT 
        s.position,
        COALESCE(t.name, ''),
        s.played,
        s.wins,
        s.draws,
        s.losses,
        s.goals_for,
        s.goals_against,
        s.goal_diff,
        s.points,
        s.season,
        COALESCE(t.crest_url, ''),
        COALESCE(s.zone, '')
    FROM standings s
    LEFT JOIN teams t ON s.team_id = t.api_id
    WHERE s.league = $1 AND s.season = $2
    ORDER BY s.position ASC
`, league, season)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var standings []models.Standing

	for rows.Next() {

		var s models.Standing

		err := rows.Scan(
			&s.Position,
			&s.TeamName,
			&s.Played,
			&s.Wins,
			&s.Draws,
			&s.Losses,
			&s.GoalsFor,
			&s.GoalsAgainst,
			&s.GoalDiff,
			&s.Points,
			&s.Season,
			&s.CrestURL,
			&s.Zone,
		)

		if err != nil {
			return nil, err
		}

		standings = append(standings, s)
	}

	if standings == nil {
		standings = []models.Standing{}
	}

	return standings, nil
}
