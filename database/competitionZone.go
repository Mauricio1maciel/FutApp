package database

import "App-Futebol/models"

func GetZonesByLeague(league string) ([]models.CompetitionZone, error) {

	rows, err := DB.Query(`
		SELECT league, zone_key, zone_name, priority
		FROM competition_zones
		WHERE league = $1
		ORDER BY priority
	`, league)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []models.CompetitionZone

	for rows.Next() {
		var z models.CompetitionZone

		err := rows.Scan(
			&z.League,
			&z.ZoneKey,
			&z.ZoneName,
			&z.Priority,
		)

		if err != nil {
			return nil, err
		}

		zones = append(zones, z)
	}

	return zones, nil
}
