package services

import "App-Futebol/models"

// Força dos times, usada nas chances e na prévia do jogo (o modelo de Maher): gols marcados
// e sofridos por jogo de cada time, em relação à média da competição, e a média de gols do
// mandante e do visitante (o mando de campo da própria competição).
type teamStrength struct {
	att, def       map[int64]float64 // 1 = na média; ataque 1,3 = marca 30% a mais
	muHome, muAway float64           // gols por jogo do mandante e do visitante
	finished       int               // jogos encerrados usados
}

// estimateStrength calcula a força pelos jogos encerrados. Cinco jogos "na média" puxam
// para o meio quem jogou pouco; com menos de 20 jogos na competição, valem as médias de gols
// típicas (1,4 do mandante e 1,1 do visitante).
func estimateStrength(matches []models.Match) teamStrength {
	gf, ga, games := map[int64]int{}, map[int64]int{}, map[int64]int{}
	s := teamStrength{muHome: 1.4, muAway: 1.1}
	homeGoals, awayGoals := 0, 0
	for _, m := range matches {
		if m.Status != "FINISHED" || m.APIHomeTeamID == 0 || m.APIAwayTeamID == 0 {
			continue
		}
		gf[m.APIHomeTeamID] += m.HomeScore
		ga[m.APIHomeTeamID] += m.AwayScore
		gf[m.APIAwayTeamID] += m.AwayScore
		ga[m.APIAwayTeamID] += m.HomeScore
		games[m.APIHomeTeamID]++
		games[m.APIAwayTeamID]++
		homeGoals += m.HomeScore
		awayGoals += m.AwayScore
		s.finished++
	}
	if s.finished >= 20 && homeGoals+awayGoals > 0 {
		s.muHome = float64(homeGoals) / float64(s.finished)
		s.muAway = float64(awayGoals) / float64(s.finished)
	}
	perTeam := (s.muHome + s.muAway) / 2
	const prior = 5.0
	s.att, s.def = make(map[int64]float64, len(games)), make(map[int64]float64, len(games))
	for team, n := range games {
		g := (float64(n) + prior) * perTeam
		s.att[team] = (float64(gf[team]) + prior*perTeam) / g
		s.def[team] = (float64(ga[team]) + prior*perTeam) / g
	}
	return s
}

// expected devolve a média de gols do mandante e do visitante no jogo. Time sem jogos na
// competição fica na média
func (s teamStrength) expected(home, away int64) (float64, float64) {
	get := func(m map[int64]float64, team int64) float64 {
		if v, ok := m[team]; ok {
			return v
		}
		return 1
	}
	return s.muHome * get(s.att, home) * get(s.def, away), s.muAway * get(s.att, away) * get(s.def, home)
}
