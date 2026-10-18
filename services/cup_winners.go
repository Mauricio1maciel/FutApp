package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"sort"
)

// Títulos que mexem nas vagas do Brasileirão (AddZones): os campeões da Libertadores, da
// Sul-Americana e da Copa do Brasil têm vaga garantida na Libertadores; o vice da Copa do
// Brasil ganha uma vaga a mais na pré-Libertadores.
var brazilianCupTitles = []struct {
	league   string // a copa
	champion string // competition_winners.competition do campeão
	runnerUp string // e do vice ("" = o vice não ganha vaga)
}{
	{"CLI", "libertadores", ""},
	{"CSU", "sul_americana", ""},
	{"CDB", "copa_brasil", "vice_copa_brasil"},
}

// syncBrazilianCupWinners lê o vencedor da final de cada copa da temporada e grava em
// competition_winners, para a classificação do Brasileirão marcar as vagas sem cadastro
// manual. Campeão estrangeiro (ou de fora da Série A) tira a vaga que estivesse anotada.
// Final ainda não disputada: nada muda.
func syncBrazilianCupWinners(ctx context.Context, season string) {
	for _, c := range brazilianCupTitles {
		finals, err := database.GetMatchesByLeague(ctx, c.league, season, database.MatchFilter{Stage: "FINAL"})
		if err != nil {
			utils.CustomLog("JOBS", "[%s %s] Erro ao ler a final: %v", c.league, season, err)
			continue
		}
		winner, runnerUp, ok := finalWinner(finals)
		if !ok {
			continue
		}
		if err := database.SetCompetitionWinner(ctx, "BSA", season, c.champion, winner); err != nil {
			utils.CustomLog("JOBS", "[%s %s] Erro ao gravar o campeão: %v", c.league, season, err)
		}
		if c.runnerUp != "" {
			if err := database.SetCompetitionWinner(ctx, "BSA", season, c.runnerUp, runnerUp); err != nil {
				utils.CustomLog("JOBS", "[%s %s] Erro ao gravar o vice: %v", c.league, season, err)
			}
		}
	}
}

// finalWinner decide a final: jogo único ou ida e volta (soma dos gols), com pênaltis no
// último jogo se empatar. ok = false enquanto a final não terminou ou não tem os times.
func finalWinner(finals []models.Match) (winner, runnerUp int64, ok bool) {
	if len(finals) == 0 {
		return 0, 0, false
	}
	for _, m := range finals {
		if m.Status != "FINISHED" || m.APIHomeTeamID == 0 || m.APIAwayTeamID == 0 {
			return 0, 0, false
		}
	}
	finals = append([]models.Match{}, finals...)
	sort.SliceStable(finals, func(i, j int) bool { return finals[i].DateEvent < finals[j].DateEvent })

	a, b := finals[0].APIHomeTeamID, finals[0].APIAwayTeamID
	goals := map[int64]int{}
	for _, m := range finals {
		if !(m.APIHomeTeamID == a && m.APIAwayTeamID == b) && !(m.APIHomeTeamID == b && m.APIAwayTeamID == a) {
			return 0, 0, false // jogos de finais diferentes: dado estranho, não arrisca
		}
		goals[m.APIHomeTeamID] += m.HomeScore
		goals[m.APIAwayTeamID] += m.AwayScore
	}
	switch {
	case goals[a] > goals[b]:
		return a, b, true
	case goals[b] > goals[a]:
		return b, a, true
	}
	last := finals[len(finals)-1]
	if last.HomePenalty != nil && last.AwayPenalty != nil && *last.HomePenalty != *last.AwayPenalty {
		if *last.HomePenalty > *last.AwayPenalty {
			return last.APIHomeTeamID, last.APIAwayTeamID, true
		}
		return last.APIAwayTeamID, last.APIHomeTeamID, true
	}
	return 0, 0, false
}
