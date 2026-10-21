package database

import "context"

func GetMissingMatches(ctx context.Context, league string) ([]map[string]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

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
            AND m.match_date::DATE = e.match_date::DATE
        )
        WHERE m.league = $1
          AND m.status = 'FINISHED'
          AND (e.espn_match_id IS NULL
               -- Tem o jogo da ESPN, mas sem escalação (o worker perdeu o detalhe). Só na
               -- primeira semana: há jogos que a ESPN nunca detalha, e eles seriam
               -- procurados de novo a cada volta
               OR (m.match_date > (now() AT TIME ZONE 'UTC') - INTERVAL '7 days'
                   AND NOT EXISTS (SELECT 1 FROM espn_match_lineups l
                                   WHERE l.espn_match_id = e.espn_match_id)))
          -- Só a temporada atual: o banco também guarda temporadas antigas (confronto
          -- direto), e buscar a escalação de cada jogo antigo na ESPN levaria dias
          AND m.season = (SELECT max(season) FROM matches
                          WHERE league = $1 AND COALESCE(season, '') <> '')
        ORDER BY m.match_date DESC
        LIMIT 10`

	rows, err := DB.QueryContext(ctx, query, league)
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
