package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func GetTeamByApiID(apiID int64) (models.Team, error) {
	var t models.Team
	utils.CustomLog("DB_INFO", "Iniciando busca de detalhes para o ID: %d", apiID)

	err := DB.QueryRow(
		`SELECT id, api_id, name, tl.league, stadium, crest_url 
         FROM teams t
         join team_leagues tl on t.api_id  = tl.team_api_id
         WHERE api_id=$1 
         LIMIT 1`,
		apiID,
	).Scan(
		&t.ID,
		&t.ApiID,
		&t.Name,
		&t.League,
		&t.Stadium,
		&t.Crest,
	)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro ao buscar detalhes do jogador ID %d: %v\n", apiID, err)
	}

	return t, err
}

func GetPlayerByApiID(apiID int64) (models.Player, error) {
	utils.CustomLog("DB_INFO", "Iniciando busca de detalhes para o ID: %d", apiID)

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
                COALESCE(ep.jersey_number, 0) AS jersey_number,
                'ESPN' AS source,          
                1 AS priority
            FROM espn_players ep
            LEFT JOIN teams t ON ep.espn_team_id = CAST(t.espn_team_id AS BIGINT) 
            WHERE CAST(ep.espn_id AS BIGINT) = $1

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
                0 AS jersey_number,
                'DATA',          
                2 AS priority
            FROM players p
            LEFT JOIN teams t ON p.team_id = t.api_id
            -- Procuramos tanto no ID interno quanto no api_id para garantir que o Fallback ache!
            WHERE CAST(p.id AS BIGINT) = $1 OR CAST(p.api_id AS BIGINT) = $1
        )
        SELECT 
            id, api_id, name, position, date_of_birth, nationality, 
            team_id, team_name, league, headshot_url, jersey_number, source
        FROM combined_results
        ORDER BY priority ASC
        LIMIT 1
    `

	var p models.Player

	err := DB.QueryRow(sqlQuery, apiID).Scan(
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
		&p.JerseyNumber,
		&p.Source,
	)

	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro ao buscar detalhes do jogador ID %d: %v\n", apiID, err)
	}

	return p, err
}
