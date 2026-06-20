package database

func GetLeagueSeasonFormat(leagueCode string) string {
	var format string

	query := `SELECT season_format FROM leagues WHERE code_api = $1 LIMIT 1`
	err := DB.QueryRow(query, leagueCode).Scan(&format)

	if err != nil {
		return "european"
	}

	return format
}
