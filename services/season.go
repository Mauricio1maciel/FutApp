package services

import (
	"App-Futebol/database"
	"strconv"
	"time"
)

// SeasonFromDate converte a data de um jogo na temporada no formato da liga:
// "calendar" -> "2026" | "european" -> "2025-2026" (a temporada vira em julho)
func SeasonFromDate(dateStr string, format string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		t = time.Now()
	}

	year := t.Year()

	if format == "calendar" {
		return strconv.Itoa(year)
	}

	if t.Month() >= time.July {
		return strconv.Itoa(year) + "-" + strconv.Itoa(year+1)
	}
	return strconv.Itoa(year-1) + "-" + strconv.Itoa(year)
}

// CurrentSeason devolve a temporada de hoje no formato da liga
func CurrentSeason(league string) string {
	return SeasonFromDate(time.Now().Format(time.RFC3339), database.GetLeagueSeasonFormat(league))
}

// ResolveSeason usa a temporada pedida; se vier vazia, a mais recente com jogos no banco;
// se não houver jogos, a temporada de hoje
func ResolveSeason(league string, season string) string {
	if season != "" {
		return season
	}
	if latest := database.GetLatestSeason(league); latest != "" {
		return latest
	}
	return CurrentSeason(league)
}
