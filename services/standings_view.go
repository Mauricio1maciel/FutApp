package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"context"
	"errors"
	"sort"
)

// ErrUnknownView: o recorte pedido não existe
var ErrUnknownView = errors.New("recorte da tabela desconhecido")

// ViewStandings monta a classificação de um recorte da temporada: só os jogos em casa de
// cada time ("home"), só os de fora ("away"), o 1º turno ("first") ou o 2º ("second"). Os
// critérios de desempate são os da tabela. Não tem zonas: as vagas são da tabela
// completa. Só ligas de pontos corridos (inclusive a fase de liga da Champions).
func ViewStandings(ctx context.Context, league, season, view string) ([]models.Standing, error) {
	switch view {
	case "home", "away", "first", "second":
	default:
		return nil, ErrUnknownView
	}
	if _, isCup := cupFormats[league]; isCup || league == "UNL" {
		return nil, ErrNoLeagueTable
	}
	matches, err := database.GetMatchesByLeague(ctx, league, season, database.MatchFilter{})
	if err != nil {
		return nil, err
	}
	rules, err := cachedTableRules(ctx, league, season)
	if err != nil {
		return nil, err
	}
	table := leagueTableMatches(league, matches)
	if len(table) == 0 && len(matches) > 0 {
		return nil, ErrNoLeagueTable
	}
	if rules.espnOnly {
		table = withoutMatches(table, beyondRoundRobin(table))
	}

	side := ""
	switch view {
	case "home", "away":
		side = view
	default:
		table = turnMatches(table, view == "first")
	}

	s := MapToSlice(calculateStandingsSide(table, side))
	criteria := withDefaultCriteria(rules.criteria)
	sort.SliceStable(s, func(i, j int) bool { return compareTeams(s[i], s[j], table, criteria) })
	AddPositions(s)

	form := recentFormSide(table, side)
	crests := teamCrests(matches)
	for i := range s {
		s[i].Season = season
		s[i].Form = form[s[i].TeamID]
		s[i].CrestURL = crests[s[i].TeamID]
	}
	if s == nil {
		s = []models.Standing{}
	}
	return s, nil
}

// turnMatches separa um turno: com n times, cada turno tem n-1 rodadas (1ª a 19ª e 20ª a
// 38ª no Brasileirão). Jogo sem rodada fica de fora
func turnMatches(table []models.Match, first bool) []models.Match {
	teams := map[int64]bool{}
	for _, m := range table {
		teams[m.APIHomeTeamID], teams[m.APIAwayTeamID] = true, true
	}
	half := len(teams) - 1
	out := make([]models.Match, 0, len(table)/2+1)
	for _, m := range table {
		if m.Round <= 0 {
			continue
		}
		if (first && m.Round <= half) || (!first && m.Round > half) {
			out = append(out, m)
		}
	}
	return out
}
