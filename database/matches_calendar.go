package database

import (
	"App-Futebol/utils"

	"github.com/lib/pq"
)

type CalendarDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

func GetCalendarCounts(leagues []string, month string, year string) ([]CalendarDay, error) {
	query := `
		SELECT TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'YYYY-MM-DD') AS day, COUNT(*)
		FROM matches
		WHERE league = ANY($3)
		  AND TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'MM') = $1
		  AND TO_CHAR(match_date AT TIME ZONE 'America/Sao_Paulo', 'YYYY') = $2
		GROUP BY day
	`

	rows, err := DB.Query(query, month, year, pq.Array(leagues))
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
	if days == nil {
		days = []CalendarDay{}
	}
	return days, nil
}
