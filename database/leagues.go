package database

import "context"

func GetLeagueSeasonFormat(ctx context.Context, leagueCode string) string {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var format string

	query := `SELECT season_format FROM leagues WHERE code_api = $1 LIMIT 1`
	err := DB.QueryRowContext(ctx, query, leagueCode).Scan(&format)

	if err != nil {
		return "european"
	}

	return format
}

// GetESPNOnlyLeagues devolve as ligas que vêm só da ESPN (leagues.espn_only), como a
// Liga das Nações e a Sul-Americana
func GetESPNOnlyLeagues(ctx context.Context) ([]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `SELECT code_api FROM leagues WHERE espn_only ORDER BY code_api`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leagues []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		leagues = append(leagues, l)
	}
	return leagues, rows.Err()
}
