package database

func GetAvailableSeasons(league string) ([]string, error) {
	query := `SELECT DISTINCT season FROM matches WHERE league = $1 ORDER BY season DESC`
	rows, err := DB.Query(query, league)
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
	return seasons, nil
}
