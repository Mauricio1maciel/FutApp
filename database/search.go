package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func SearchTeamsGlobal(query string) ([]models.Team, error) {
	rows, err := DB.Query(
		`SELECT id, api_id, name, league, stadium, crest_url 
         FROM teams 
         WHERE unaccent(name) ILIKE unaccent('%' || $1 || '%') 
         LIMIT 5`,
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		err := rows.Scan(&t.ID, &t.ApiID, &t.Name, &t.League, &t.Stadium, &t.Crest)
		if err == nil {
			teams = append(teams, t)
		}
	}

	if teams == nil {
		teams = []models.Team{}
	}
	return teams, nil
}

func SearchPlayersGlobal(query string) ([]models.Player, error) {
	utils.CustomLog("DB_ERRO", " Iniciando busca de jogadores para o termo: '%s'", query)

	sqlQuery := `
        WITH combined_results AS (
            SELECT 
                CAST(ep.espn_id AS INTEGER) AS id,           
                0 AS api_id, 
                ep.name, 
                COALESCE(ep.position, '') AS position, 
                COALESCE(CAST(ep.date_of_birth AS TEXT), '') AS date_of_birth, 
                COALESCE(ep.nationality, '') AS nationality, 
                CAST(COALESCE(t.api_id, 0) AS INTEGER) AS team_id, 
                COALESCE(t.name, '') AS team_name, 
                COALESCE(t.league, '') AS league,
                COALESCE(ep.headshot_url, '') AS headshot_url,
                'ESPN' AS source,          
                1 AS priority
            FROM espn_players ep
            LEFT JOIN teams t ON ep.espn_team_id = CAST(t.espn_team_id AS BIGINT) 
            WHERE unaccent(ep.name) ILIKE '%' || unaccent($1) || '%'

            UNION ALL

            SELECT 
                CAST(p.id AS INTEGER),                       
                CAST(p.api_id AS INTEGER), 
                p.name, 
                COALESCE(p.position, ''), 
                COALESCE(CAST(p.date_of_birth AS TEXT), ''), 
                COALESCE(p.nationality, ''), 
                CAST(COALESCE(p.team_id, 0) AS INTEGER), 
                COALESCE(t.name, ''), 
                COALESCE(p.league, ''),
                '',        
                'DATA',          
                2 AS priority
            FROM players p
            LEFT JOIN teams t ON p.team_id = t.api_id
            WHERE unaccent(p.name) ILIKE '%' || unaccent($1) || '%'
        ),
        deduplicated_results AS (
            SELECT 
                *,
                ROW_NUMBER() OVER (
                    PARTITION BY unaccent(name), team_id 
                    ORDER BY priority ASC                
                ) as row_num
            FROM combined_results
        )
        SELECT 
            id, api_id, name, position, date_of_birth, nationality, 
            team_id, team_name, league, headshot_url, source
        FROM deduplicated_results
        WHERE row_num = 1
        ORDER BY priority, name
        LIMIT 15
    `

	rows, err := DB.Query(sqlQuery, query)
	if err != nil {
		utils.CustomLog("DB_ERRO", " ERRO SQL (Query falhou): %v\n", err)
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
			&p.Position,
			&p.DateOfBirth,
			&p.Nationality,
			&p.TeamID,
			&p.TeamName,
			&p.League,
			&p.HeadshotURL,
			&p.Source,
		)
		if err != nil {
			utils.CustomLog("DB_ERRO", " ERRO no Scan do jogador (Pulo para o próximo): %v\n", err)
			continue
		}
		players = append(players, p)
	}

	utils.CustomLog("DB_ERRO", " Busca concluída. Encontrou %d jogadores.", len(players))

	if players == nil {
		players = []models.Player{}
	}
	return players, nil
}
