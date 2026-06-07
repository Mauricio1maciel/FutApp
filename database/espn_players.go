package database

import (
	"App-Futebol/models"
	"log"
)

func UpsertESPNPlayer(player models.Player) error {
	query := `
	INSERT INTO espn_players (
		espn_id, name, short_name, position, jersey_number, headshot_url, espn_team_id, nationality,date_of_birth
	) 
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (espn_id) 
	DO UPDATE SET 
		name = EXCLUDED.name,
		short_name = EXCLUDED.short_name,
		position = EXCLUDED.position,
		jersey_number = EXCLUDED.jersey_number,
		headshot_url = EXCLUDED.headshot_url,
		espn_team_id = EXCLUDED.espn_team_id,
		nationality = EXCLUDED.nationality,
		date_of_birth = EXCLUDED.date_of_birth
	`

	_, err := DB.Exec(
		query,
		player.ID,
		player.Name,
		player.ShortName,
		player.Position,
		player.JerseyNumber,
		player.HeadshotURL,
		player.TeamID,
		player.Nationality,
		player.DateOfBirth,
	)

	if err != nil {
		log.Printf("❌ Erro ao salvar jogador %s (ESPN ID: %d): %v\n", player.Name, player.ID, err)
	}

	return err
}

func GetESPNTeamID(apiTeamID int64) (string, error) {
	var espnID string
	query := `SELECT COALESCE(espn_team_id, 0) FROM teams WHERE api_id = $1`
	err := DB.QueryRow(query, apiTeamID).Scan(&espnID)
	if err != nil {
		return "", err
	}
	return espnID, nil
}

func GetESPNPlayersByTeamID(espnTeamID int) ([]models.Player, error) {
	query := `
        SELECT 
            ep.espn_id, 
            t.api_id AS team_api_id,     
            ep.name AS player_name,
            COALESCE(ep.short_name, ''), 
            COALESCE(ep.position, ''), 
            COALESCE(ep.jersey_number, 0), 
            COALESCE(ep.headshot_url, ''), 
			COALESCE(ep.date_of_birth, ''),
            COALESCE(ep.nationality, ''),
			COALESCE(t.league, '') 
        FROM espn_players ep
        JOIN teams t ON t.espn_team_id = ep.espn_team_id
        WHERE ep.espn_team_id = $1
        ORDER BY ep.position, ep.name
    `
	rows, err := DB.Query(query, espnTeamID)
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
			&p.ShortName,
			&p.Position,
			&p.JerseyNumber,
			&p.HeadshotURL,
			&p.DateOfBirth,
			&p.Nationality,
			&p.League,
		)
		if err != nil {
			return nil, err
		}

		p.TeamID = espnTeamID
		p.Source = "ESPN"
		players = append(players, p)
	}

	return players, nil
}
