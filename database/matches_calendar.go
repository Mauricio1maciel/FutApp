package database

import (
	"App-Futebol/utils"
	"strings"
)

type CalendarDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func GetCalendarCounts(leagues []string, month string, year string) ([]CalendarDay, error) {
	// Cria uma string segura para o IN clause: 'WC','BSA','PL'
	var leaguesFormatted []string
	for _, l := range leagues {
		leaguesFormatted = append(leaguesFormatted, "'"+l+"'")
	}
	leaguesIn := strings.Join(leaguesFormatted, ",")

	query := `
		SELECT TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'YYYY-MM-DD') AS day, COUNT(*) 
		FROM matches 
		WHERE league IN (` + leaguesIn + `)
		  AND TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'MM') = $1
		  AND TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'YYYY') = $2
		GROUP BY day
	`

	rows, err := DB.Query(query, month, year)
	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetCalendarCounts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var days []CalendarDay
	for rows.Next() {
		var c CalendarDay
		if err := rows.Scan(&c.Date, &c.Count); err != nil {
			return nil, err
		}
		days = append(days, c)
	}
	return days, nil
}
