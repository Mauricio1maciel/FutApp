package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"context"
	"sort"
)

// LiveStandings monta a classificação como estaria se os jogos em andamento acabassem
// agora: o placar atual conta como final. Não grava nada. Cada time em jogo vem com
// Live = true, e PositionChange diz quantas posições ele ganhou (+) ou perdeu (-) em
// relação à tabela sem os jogos em andamento.
//
// A tabela gravada (/standings) também conta os jogos em andamento, mas só é recalculada
// a cada ~10 minutos; esta é montada a cada pedido (cache de 20 s) e traz as marcações.
func LiveStandings(ctx context.Context, league, season string) ([]models.Standing, error) {
	matches, err := database.GetMatchesByLeague(ctx, league, season, database.MatchFilter{})
	if err != nil {
		return nil, err
	}
	asIfFinished, playing := liveAsFinished(matches)

	live, ok := computeStandings(ctx, league, season, asIfFinished, matches)
	if !ok {
		return []models.Standing{}, nil
	}
	official := live
	if len(playing) > 0 {
		// Base da variação: a tabela antes dos jogos em andamento
		official, _ = computeStandings(ctx, league, season, withoutLive(matches), matches)
	}
	markLive(live, official, playing, teamCrests(matches))
	return live, nil
}

var liveStatuses = map[string]bool{"IN_PLAY": true, "PAUSED": true, "LIVE": true}

// liveAsFinished devolve os jogos com os em andamento marcados como encerrados (no placar
// de agora) e os times que estão jogando
func liveAsFinished(matches []models.Match) ([]models.Match, map[int64]bool) {
	out := make([]models.Match, len(matches))
	playing := map[int64]bool{}
	for i, m := range matches {
		out[i] = m
		if liveStatuses[m.Status] {
			out[i].Status = "FINISHED"
			playing[m.APIHomeTeamID], playing[m.APIAwayTeamID] = true, true
		}
	}
	return out, playing
}

func withoutLive(matches []models.Match) []models.Match {
	out := make([]models.Match, 0, len(matches))
	for _, m := range matches {
		if !liveStatuses[m.Status] {
			out = append(out, m)
		}
	}
	return out
}

// markLive marca quem está jogando, a variação de posição e o escudo (a tabela calculada
// não traz o escudo; a gravada pega do cadastro do time), e ordena como a rota gravada:
// por grupo e posição
func markLive(live, official []models.Standing, playing map[int64]bool, crests map[int64]string) {
	type key struct {
		group string
		team  int64
	}
	officialPos := make(map[key]int, len(official))
	for _, s := range official {
		officialPos[key{s.GroupName, s.TeamID}] = s.Position
	}
	for i := range live {
		s := &live[i]
		s.Live = playing[s.TeamID]
		if p, ok := officialPos[key{s.GroupName, s.TeamID}]; ok {
			s.PositionChange = p - s.Position
		}
		if s.CrestURL == "" {
			s.CrestURL = crests[s.TeamID]
		}
	}
	sort.SliceStable(live, func(i, j int) bool {
		if live[i].GroupName != live[j].GroupName {
			return live[i].GroupName < live[j].GroupName
		}
		return live[i].Position < live[j].Position
	})
}

func teamCrests(matches []models.Match) map[int64]string {
	crests := map[int64]string{}
	for _, m := range matches {
		if m.HomeLogo != "" {
			crests[m.APIHomeTeamID] = m.HomeLogo
		}
		if m.AwayLogo != "" {
			crests[m.APIAwayTeamID] = m.AwayLogo
		}
	}
	return crests
}
