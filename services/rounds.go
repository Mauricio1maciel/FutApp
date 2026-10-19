package services

import (
	"App-Futebol/models"
	"sort"
	"time"
)

// assignLeagueRounds calcula a rodada de cada jogo de uma liga de pontos corridos que vem
// sem rodada (a ESPN não informa, ex: Série B). A regra que sustenta o cálculo: em pontos
// corridos, um time nunca joga duas vezes na mesma rodada.
//
//  1. A data típica de cada rodada é a mediana dos jogos em que os dois times estão no
//     mesmo k-ésimo jogo (os deslocados são poucos e não mexem na mediana).
//  2. Cada jogo vai, a princípio, para a rodada de data típica mais próxima da sua.
//  3. Um jogo adiado ou antecipado cai na semana de outra rodada, e então OS DOIS times
//     dele ficam com dois jogos naquela rodada. Nos jogos normais da semana, só um dos
//     times fica com conflito. Os jogos com os dois times em conflito vão para a rodada
//     livre para ambos: o "buraco" que o deslocamento deixou.
//
// Devolve match_id → rodada. Rodada 0 = fora das 38: jogo depois do turno e returno (os
// playoffs, veja beyondRoundRobin), jogo com time indefinido (a ESPN marca com "TBD") ou
// jogo que não cabe em nenhuma rodada livre dos dois times.
func assignLeagueRounds(matches []models.Match) map[int64]int {
	extra := beyondRoundRobin(matches)
	ms := make([]models.Match, 0, len(matches))
	teams := map[int64]bool{}
	rounds := map[int64]int{}
	for _, m := range matches {
		if m.APIHomeTeamID == 0 || m.APIAwayTeamID == 0 || extra[m.MatchID] {
			rounds[m.MatchID] = 0
			continue
		}
		ms = append(ms, m)
		teams[m.APIHomeTeamID], teams[m.APIAwayTeamID] = true, true
	}
	sortByDate(ms)

	// 1. Data típica de cada rodada
	count := map[int64]int{}
	maxRound := 2 * (len(teams) - 1) // turno e returno
	dates := map[int][]time.Time{}
	for _, m := range ms {
		count[m.APIHomeTeamID]++
		count[m.APIAwayTeamID]++
		kh, ka := count[m.APIHomeTeamID], count[m.APIAwayTeamID]
		maxRound = max(maxRound, kh, ka)
		if t, ok := parseMatchDate(m.DateEvent); ok && kh == ka {
			dates[kh] = append(dates[kh], t)
		}
	}
	median := map[int]time.Time{}
	for r, ts := range dates {
		sort.Slice(ts, func(i, j int) bool { return ts[i].Before(ts[j]) })
		median[r] = ts[len(ts)/2]
	}
	distance := func(date time.Time, hasDate bool, r int) time.Duration {
		med, ok := median[r]
		if !ok || !hasDate {
			return time.Duration(1<<62 - 1) // sem referência: última opção
		}
		if d := date.Sub(med); d >= 0 {
			return d
		}
		return med.Sub(date)
	}

	// 2. Rodada pela data
	guess := make([]int, len(ms))
	slots := map[int64]map[int]int{} // time → rodada → quantos jogos
	addSlot := func(team int64, r, n int) {
		if slots[team] == nil {
			slots[team] = map[int]int{}
		}
		slots[team][r] += n
	}
	for i, m := range ms {
		date, hasDate := parseMatchDate(m.DateEvent)
		best := 0
		for r := 1; r <= maxRound; r++ {
			if best == 0 || distance(date, hasDate, r) < distance(date, hasDate, best) {
				best = r
			}
		}
		guess[i] = best
		addSlot(m.APIHomeTeamID, best, 1)
		addSlot(m.APIAwayTeamID, best, 1)
	}

	// 3. Deslocados: os dois times com dois jogos na mesma rodada
	var displaced []int
	for i, m := range ms {
		if slots[m.APIHomeTeamID][guess[i]] > 1 && slots[m.APIAwayTeamID][guess[i]] > 1 {
			displaced = append(displaced, i)
			addSlot(m.APIHomeTeamID, guess[i], -1)
			addSlot(m.APIAwayTeamID, guess[i], -1)
			continue
		}
		rounds[m.MatchID] = guess[i]
	}
	for _, i := range displaced {
		m := ms[i]
		date, hasDate := parseMatchDate(m.DateEvent)
		best := 0
		for r := 1; r <= maxRound; r++ {
			if slots[m.APIHomeTeamID][r] > 0 || slots[m.APIAwayTeamID][r] > 0 {
				continue
			}
			if best == 0 || distance(date, hasDate, r) < distance(date, hasDate, best) {
				best = r
			}
		}
		rounds[m.MatchID] = best // 0 se nenhuma estiver livre para os dois
		if best == 0 {
			continue
		}
		addSlot(m.APIHomeTeamID, best, 1)
		addSlot(m.APIAwayTeamID, best, 1)
	}
	return rounds
}

// beyondRoundRobin devolve os jogos que passam do turno e returno. Em pontos corridos cada
// time joga 2×(n-1) vezes (38, com 20 times); o que vem depois, pela data, não é da fase
// de pontos corridos: são os playoffs (Série B 2026: 3º x 6º e 4º x 5º, em ida e volta).
// Jogos com time indefinido ficam de fora da conta (não são de time nenhum ainda).
func beyondRoundRobin(matches []models.Match) map[int64]bool {
	ms := make([]models.Match, 0, len(matches))
	teams := map[int64]bool{}
	for _, m := range matches {
		if m.APIHomeTeamID != 0 && m.APIAwayTeamID != 0 {
			ms = append(ms, m)
			teams[m.APIHomeTeamID], teams[m.APIAwayTeamID] = true, true
		}
	}
	sortByDate(ms)

	capacity := 2 * (len(teams) - 1)
	played := map[int64]int{}
	extra := map[int64]bool{}
	for _, m := range ms {
		played[m.APIHomeTeamID]++
		played[m.APIAwayTeamID]++
		if played[m.APIHomeTeamID] > capacity || played[m.APIAwayTeamID] > capacity {
			extra[m.MatchID] = true
		}
	}
	return extra
}

func sortByDate(ms []models.Match) {
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].DateEvent != ms[j].DateEvent {
			return ms[i].DateEvent < ms[j].DateEvent
		}
		return ms[i].MatchID < ms[j].MatchID
	})
}

func parseMatchDate(s string) (time.Time, bool) {
	if len(s) >= 19 {
		if t, err := time.Parse("2006-01-02 15:04:05", s[:19]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
