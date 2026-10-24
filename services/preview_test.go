package services

import (
	"App-Futebol/models"
	"math"
	"testing"
)

// Com 1,4 x 1,1 gol esperado: mandante 43,8%, empate 26,6%, visitante 29,6%; 1 x 1 é o
// placar mais provável (12,6%)
func TestMatchOdds(t *testing.T) {
	o := matchOdds(1.4, 1.1)
	if math.Abs(o.HomeWin-43.8) > 0.15 || math.Abs(o.Draw-26.6) > 0.15 || math.Abs(o.AwayWin-29.6) > 0.15 {
		t.Errorf("chances: veio %.1f / %.1f / %.1f", o.HomeWin, o.Draw, o.AwayWin)
	}
	if s := o.HomeWin + o.Draw + o.AwayWin; math.Abs(s-100) > 0.2 {
		t.Errorf("as três chances somam %.1f%%", s)
	}
	want := []ScoreChance{{1, 1, 12.6}, {1, 0, 11.5}, {0, 1, 9}, {2, 1, 8.9}, {0, 0, 8.2}}
	for i, s := range o.LikelyScores {
		if s.Home != want[i].Home || s.Away != want[i].Away || math.Abs(s.Pct-want[i].Pct) > 0.15 {
			t.Errorf("%dº placar mais provável: veio %+v, esperado %+v", i+1, s, want[i])
		}
	}
	if o.ExpectedHomeGoals != 1.4 || o.ExpectedAwayGoals != 1.1 {
		t.Errorf("gols esperados: %v x %v", o.ExpectedHomeGoals, o.ExpectedAwayGoals)
	}
}

func TestSummarizeH2H(t *testing.T) {
	jogo := func(casa, fora int64, gc, gf int) models.Match {
		return models.Match{APIHomeTeamID: casa, APIAwayTeamID: fora, HomeScore: gc, AwayScore: gf, Status: "FINISHED"}
	}
	// Do ponto de vista do time 1: venceu em casa, venceu fora, empatou, perdeu
	h := summarizeH2H([]models.Match{jogo(1, 2, 2, 0), jogo(2, 1, 0, 1), jogo(1, 2, 1, 1), jogo(2, 1, 3, 0),
		jogo(1, 2, 1, 0), jogo(1, 2, 1, 0)}, 1)
	if h.Played != 6 || h.HomeWins != 4 || h.Draws != 1 || h.AwayWins != 1 || len(h.Last) != 5 {
		t.Errorf("confronto: veio %+v", h)
	}
}

func TestEstimateStrength(t *testing.T) {
	// Poucos jogos: médias típicas; o time que marcou muito fica acima da média, puxado
	// para o meio pelos cinco jogos "na média"
	s := estimateStrength([]models.Match{
		{APIHomeTeamID: 1, APIAwayTeamID: 2, HomeScore: 4, AwayScore: 0, Status: "FINISHED"},
		{APIHomeTeamID: 3, APIAwayTeamID: 1, HomeScore: 0, AwayScore: 3, Status: "FINISHED"},
		{APIHomeTeamID: 2, APIAwayTeamID: 3, HomeScore: 9, AwayScore: 9, Status: "TIMED"}, // não conta
	})
	if s.finished != 2 || s.muHome != 1.4 || s.muAway != 1.1 {
		t.Fatalf("esperado 2 jogos e as médias típicas, veio %+v", s)
	}
	if !(s.att[1] > 1.5 && s.att[1] < 2.5) || !(s.def[1] < 1) {
		t.Errorf("time 1 (7 gols, nenhum sofrido): ataque acima e defesa abaixo de 1, veio %.2f e %.2f", s.att[1], s.def[1])
	}
	if lh, la := s.expected(99, 98); lh != 1.4 || la != 1.1 {
		t.Errorf("times sem jogos ficam na média: veio %v x %v", lh, la)
	}
}
