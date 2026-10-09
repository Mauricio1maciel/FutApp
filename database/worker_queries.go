// Arquivo: database/worker_queries.go
package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func GetTodayMatches() ([]models.WorkerMatch, error) {
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
	return []string{"BSA", "PL", "PD", "PD", "SA", "CL", "BL1", "FL1", "CLI", "CSU", "WC", "UNL"}
}
