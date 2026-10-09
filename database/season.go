package database

import "context"

func GetAvailableSeasons(ctx context.Context, league string) ([]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `SELECT DISTINCT season FROM matches WHERE league = $1 AND COALESCE(season, '') <> '' ORDER BY season DESC`
	rows, err := DB.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		seasons = append(seasons, s)
	}
	if seasons == nil {
		seasons = []string{}
	}
	return seasons, nil
}
