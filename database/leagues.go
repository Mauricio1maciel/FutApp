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
