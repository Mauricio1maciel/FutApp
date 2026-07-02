package database

import (
	"App-Futebol/models"
	"fmt"
)

// 🔥 NOVA FUNÇÃO (A que faltava!): Garante que o jogador fantasma seja criado na base
func UpsertESPNPlayerGeneric(playerID int64, name string, headshot string, teamID int64) error {
	query := `
    INSERT INTO espn_players (espn_id, name, headshot_url, espn_team_id)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (espn_id) 
    DO UPDATE SET
        name = EXCLUDED.name,
        headshot_url = EXCLUDED.headshot_url,
        espn_team_id = EXCLUDED.espn_team_id
    `
	_, err := DB.Exec(query, playerID, name, headshot, teamID)
	return err
}

// Salva estritamente as estatísticas (Gols, Assistências, Partidas)
func UpsertPlayerStat(playerID int64, espnTeamID int64, league, season string, goals, assists, matches int) error {
	query := `
    INSERT INTO player_stats (espn_player_id, espn_team_id, league, season, goals, assists, matches)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    ON CONFLICT (espn_player_id, league, season) 
    DO UPDATE SET
        espn_team_id = EXCLUDED.espn_team_id,
        goals = GREATEST(player_stats.goals, EXCLUDED.goals),
        assists = GREATEST(player_stats.assists, EXCLUDED.assists),
        matches = GREATEST(player_stats.matches, EXCLUDED.matches)
    `
	_, err := DB.Exec(query, playerID, espnTeamID, league, season, goals, assists, matches)
	return err
}

// Busca os dados montando o ranking com as tabelas normalizadas
// Busca os dados montando o ranking com as tabelas normalizadas
func GetTopStats(league string, season string, statType string) ([]models.PlayerStat, error) {
	// 🔥 A MÁGICA DO DESEMPATE:
	// Para Gols: Ordena por Gols -> depois Assistências -> depois Menos Jogos
	orderBy := "ps.goals DESC, ps.assists DESC, ps.matches ASC"
	whereClause := "ps.goals > 0"

	if statType == "assists" {
		// Para Assistências: Ordena por Assistências -> depois Gols -> depois Menos Jogos
		orderBy = "ps.assists DESC, ps.goals DESC, ps.matches ASC"
		whereClause = "ps.assists > 0"
	}

	query := fmt.Sprintf(`
        SELECT 
            ps.espn_player_id::TEXT, 
            COALESCE(ep.name, 'Jogador ' || ps.espn_player_id::TEXT), 
            COALESCE(t.name, 'Time ' || ps.espn_team_id::TEXT), 
            COALESCE(t.crest_url, ''), 
            COALESCE(ep.headshot_url, ''), 
            ps.%s::TEXT as value
        FROM player_stats ps
        LEFT JOIN espn_players ep ON ps.espn_player_id = ep.espn_id
        LEFT JOIN teams t ON ps.espn_team_id = CAST(t.espn_team_id AS BIGINT) 
        WHERE ps.league = $1 AND ps.season = $2 AND %s
        ORDER BY %s, ep.name ASC
        LIMIT 20
    `, statType, whereClause, orderBy)

	rows, err := DB.Query(query, league, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PlayerStat
	rank := 1
	for rows.Next() {
		var p models.PlayerStat
		err := rows.Scan(&p.PlayerID, &p.Name, &p.Team, &p.TeamLogo, &p.Photo, &p.Value)
		if err == nil {
			p.Rank = rank
			stats = append(stats, p)
			rank++
		}
	}
	return stats, nil
}
