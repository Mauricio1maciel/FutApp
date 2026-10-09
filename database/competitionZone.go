package database

import (
	"App-Futebol/models"
	"context"
)

func GetZonesByLeague(ctx context.Context, league string) ([]models.CompetitionZone, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
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
