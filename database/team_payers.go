package database

import "App-Futebol/models"

func GetTeamPlayersBy(teamID int64, league string) ([]models.Player, error) {
	rows, err := DB.Query(
		`SELECT 
            p.id, 
            p.api_id, 
            p.name, 
            p.position, 
            p.date_of_birth, 
            p.nationality, 
            p.team_id, 
            COALESCE(t.name, '') AS team_name, 
            p.league 
         FROM players p
         LEFT JOIN teams t ON p.team_id = t.api_id
         WHERE p.team_id = $1 AND p.league = $2
         ORDER BY p.name ASC`,
		teamID,
		league,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teamPlayers []models.Player

	for rows.Next() {
		var p models.Player

		err := rows.Scan(
			&p.ID,
			&p.ApiID,
			&p.Name,
			&p.Position,
			&p.DateOfBirth,
			&p.Nationality,
			&p.TeamID,
			&p.TeamName,
			&p.League,
		)

		if err != nil {
			return nil, err
		}

		teamPlayers = append(teamPlayers, p)
	}
	if teamPlayers == nil {
		teamPlayers = []models.Player{}
	}

	return teamPlayers, nil
}
