// Arquivo: database/worker_queries.go
package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

// func GetTodayMatches() ([]models.WorkerMatch, error) {
// 	// 🔥 A MÁGICA DO MAPEAMENTO:
// 	// O JOIN passa pelos 'teams' para descobrir qual é o ID da equipe na ESPN,
// 	// e depois cruza com a 'espn_matches' para pegar o ID correto do jogo na ESPN!
// 	query := `
// 		SELECT
// 			e.espn_match_id::TEXT,
// 			m.league,
// 			EXTRACT(EPOCH FROM m.match_date) AS match_time_unix,
// 			m.status
// 		FROM matches m
// 		INNER JOIN teams th ON m.api_home_team_id = th.api_id
// 		INNER JOIN teams ta ON m.api_away_team_id = ta.api_id
// 		LEFT JOIN espn_matches e ON (
// 			th.espn_team_id = e.espn_home_team_id
// 			AND ta.espn_team_id = e.espn_away_team_id
// 			AND m.match_date::DATE = e.match_date::DATE
// 		)
// 		WHERE m.status IN ('pre', 'in')
// 		  AND m.match_date >= (NOW() AT TIME ZONE 'UTC') - INTERVAL '4 hours'
// 		  AND m.match_date <= (NOW() AT TIME ZONE 'UTC') + INTERVAL '24 hours'
// 	`

// 	rows, err := DB.Query(query)
// 	if err != nil {
// 		utils.CustomLog("DB_ERRO", "Falha ao buscar jogos para o Worker: %v", err)
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var matches []models.WorkerMatch
// 	for rows.Next() {
// 		var m models.WorkerMatch
// 		var unixTime float64 // O PostgreSQL retorna o EPOCH como float

// 		err := rows.Scan(&m.IDEvent, &m.League, &unixTime, &m.Status)

// 		// Segurança extra: só adiciona se realmente encontrou um ID válido da ESPN
// 		if err == nil && m.IDEvent != "" && m.IDEvent != "0" {
// 			m.MatchTimeUnix = int64(unixTime)
// 			matches = append(matches, m)
// 		}
// 	}

// 	return matches, nil
// }
// Arquivo: database/worker_queries.go

func GetTodayMatches() ([]models.WorkerMatch, error) {
	// 🔥 O WORKER AGORA LÊ DA FONTE DA VERDADE DA ESPN!
	// Já não precisamos de JOINs. Lemos direto a espn_matches para ver quem está vivo.
	query := `
        SELECT 
            espn_match_id::TEXT, 
            league, 
            EXTRACT(EPOCH FROM match_date) AS match_time_unix, 
            status
        FROM espn_matches
        WHERE status IN ('pre', 'in')
          AND match_date >= (NOW() AT TIME ZONE 'UTC') - INTERVAL '4 hours'
          AND match_date <= (NOW() AT TIME ZONE 'UTC') + INTERVAL '24 hours'
    `

	rows, err := DB.Query(query)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Falha ao buscar jogos para o Worker: %v", err)
		return nil, err
	}
	defer rows.Close()

	var matches []models.WorkerMatch
	for rows.Next() {
		var m models.WorkerMatch
		var unixTime float64

		err := rows.Scan(&m.IDEvent, &m.League, &unixTime, &m.Status)

		if err == nil && m.IDEvent != "" && m.IDEvent != "0" {
			m.MatchTimeUnix = int64(unixTime)
			matches = append(matches, m)
		}
	}

	return matches, nil
}

func GetActiveLeaguesToday() []string {
	// Agora enviamos os códigos curtos (Chaves do seu ESPNLeagueMap)
	// O banco de dados fica feliz porque todos têm menos de 10 caracteres!
	return []string{"BSA", "PL", "UNL", "PD"}
}

// 1. Descobre quais ligas TÊM jogos hoje (Lendo a tabela principal)
// func GetActiveLeaguesToday() []string {
// 	query := `
// 		SELECT DISTINCT league FROM matches
// 		WHERE status IN ('pre', 'in')
// 		  AND match_date >= (NOW() AT TIME ZONE 'UTC') - INTERVAL '12 hours'
// 		  AND match_date <= (NOW() AT TIME ZONE 'UTC') + INTERVAL '24 hours'
// 	`
// 	rows, err := DB.Query(query)
// 	if err != nil {
// 		return []string{}
// 	}
// 	defer rows.Close()

// 	var leagues []string
// 	for rows.Next() {
// 		var lg string
// 		if err := rows.Scan(&lg); err == nil {
// 			leagues = append(leagues, lg)
// 		}
// 	}
// 	return leagues
// }

// 2. Salva a ponte de ligação entre ESPN e nosso Banco
func SaveESPNMatchMapping(matchID string, homeID string, awayID string, matchDate string) error {
	query := `
		INSERT INTO espn_matches (espn_match_id, espn_home_team_id, espn_away_team_id, match_date)
		VALUES ($1, $2, $3, NULLIF($4, '')::TIMESTAMP)
		ON CONFLICT (espn_match_id) DO UPDATE SET
			espn_home_team_id = EXCLUDED.espn_home_team_id,
			espn_away_team_id = EXCLUDED.espn_away_team_id,
			match_date = EXCLUDED.match_date
	`
	_, err := DB.Exec(query, matchID, homeID, awayID, matchDate)
	return err
}
