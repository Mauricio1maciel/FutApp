package database

func GetMissingMatches(league string) ([]map[string]string, error) {
	query := `
        SELECT 
            COALESCE(th.name, ''), 
            COALESCE(ta.name, ''), 
            COALESCE(m.match_date::TEXT, '') 
        FROM matches m
        LEFT JOIN teams th ON m.api_home_team_id = th.api_id
        LEFT JOIN teams ta ON m.api_away_team_id = ta.api_id
        LEFT JOIN espn_matches e ON (
            th.espn_team_id = e.espn_home_team_id 
            AND ta.espn_team_id = e.espn_away_team_id 
            AND m.match_date = e.match_date::DATE
        )
        WHERE m.league = $1 AND e.espn_match_id IS NULL
        LIMIT 10`

	rows, err := DB.Query(query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]string
	for rows.Next() {
		var h, a, d string
		err := rows.Scan(&h, &a, &d)
		if err == nil {
			results = append(results, map[string]string{"home": h, "away": a, "date": d})
		}
	}
	if results == nil {
		results = []map[string]string{}
	}

	return results, nil
}
