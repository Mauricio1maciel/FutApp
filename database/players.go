package database

import (
	"App-Futebol/models"
	"context"
)

func GetPlayersByLeague(ctx context.Context, league string) ([]models.Player, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		`SELECT 
            p.id, 
            p.api_id, 
            p.name, 
            COALESCE(p.position, ''), 
            COALESCE(p.date_of_birth::TEXT, ''), 
            COALESCE(p.nationality, ''), 
            p.team_id, 
            COALESCE(t.name, '') AS team_name, 
            p.league 
         FROM players p
         LEFT JOIN teams t ON p.team_id = t.api_id
         WHERE p.league=$1
         ORDER BY p.team_id ASC`,
		league,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []models.Player

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

		players = append(players, p)
	}

	if players == nil {
		players = []models.Player{}
	}

	return players, nil
}

func SavePlayer(ctx context.Context, p models.Player) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`INSERT INTO players (api_id, name, position, date_of_birth, nationality, team_id, league) 
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         ON CONFLICT (api_id) DO UPDATE 
         SET team_id = EXCLUDED.team_id,
             league = EXCLUDED.league
         WHERE players.team_id <> EXCLUDED.team_id`,
		p.ApiID,
		p.Name,
		p.Position,
		p.DateOfBirth,
		p.Nationality,
		p.TeamID,
		p.League,
	)

	return err
}
