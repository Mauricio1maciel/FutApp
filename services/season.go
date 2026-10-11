package services

import (
	"App-Futebol/database"
	"context"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
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
func CurrentSeason(ctx context.Context, league string) string {
	return SeasonFromDate(time.Now().Format(time.RFC3339), database.GetLeagueSeasonFormat(ctx, league))
}

// Temporada mais recente de cada liga, guardada por 5 minutos: quase todo pedido sem
// season pergunta isso ao banco, e a resposta só muda na virada de temporada. Cada
// consulta economizada é uma ida e volta do Render (EUA) ao banco (São Paulo).
var latestSeasonCache = cache.New(5*time.Minute, 10*time.Minute)

// ResolveSeason usa a temporada pedida; se vier vazia, a mais recente com jogos no banco;
// se não houver jogos, a temporada de hoje
func ResolveSeason(ctx context.Context, league string, season string) string {
	if season != "" {
		return season
	}
	if v, ok := latestSeasonCache.Get(league); ok {
		return v.(string)
	}
	if latest := database.GetLatestSeason(ctx, league); latest != "" {
		latestSeasonCache.SetDefault(league, latest)
		return latest
	}
	return CurrentSeason(ctx, league)
}
