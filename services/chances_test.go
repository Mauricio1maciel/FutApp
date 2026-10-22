package services

import (
	"App-Futebol/models"
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"
)

var chancesRules = tableRules{
	rule: &models.CompetitionRule{Libertadores: 1, Rebaixamento: 1},
	zones: []models.CompetitionZone{
		{ZoneKey: "lib", ZoneName: "libertadores"},
		{ZoneKey: "reb", ZoneName: "rebaixamento"},
	},
	criteria: []string{"points", "wins", "goal_diff", "goals_for"},
}

func jogoTabela(id, casa, fora int64, status string, golsCasa, golsFora int) models.Match {
	nome := func(t int64) string { return "Time " + string(rune('0'+t)) }
	return models.Match{MatchID: id, Stage: "REGULAR_SEASON", Status: status, DateEvent: "2026-10-01 20:00:00",
		APIHomeTeamID: casa, HomeTeam: nome(casa), APIAwayTeamID: fora, AwayTeam: nome(fora),
		HomeScore: golsCasa, AwayScore: golsFora}
}

func simula(t *testing.T, rules tableRules, matches []models.Match, now time.Time) *LeagueChances {
	t.Helper()
	current, table, ok := buildTable(rules, "TST", "2026", matches, matches)
	if !ok {
		t.Fatal("liga de teste sem tabela")
	}
	return simulateChances("TST", "2026", current, table, rules, now)
}

func chancesDo(t *testing.T, c *LeagueChances, team int64) TeamChances {
	t.Helper()
	for _, tc := range c.Teams {
		if tc.TeamID == team {
			return tc
		}
	}
	t.Fatalf("time %d fora das chances", team)
	return TeamChances{}
}

// Time 1 com 9 pontos e os outros com no máximo 4, a uma rodada do fim: título decidido.
// Time 4, com 0 ponto, não alcança mais ninguém: rebaixado
func ligaQuaseNoFim() []models.Match {
	return []models.Match{
		jogoTabela(1, 1, 2, "FINISHED", 3, 0),
		jogoTabela(2, 1, 3, "FINISHED", 3, 0),
		jogoTabela(3, 1, 4, "FINISHED", 3, 0),
		jogoTabela(4, 2, 3, "FINISHED", 1, 1),
		jogoTabela(5, 2, 4, "FINISHED", 3, 0),
		jogoTabela(6, 3, 4, "FINISHED", 3, 0),
		jogoTabela(7, 4, 1, "TIMED", 0, 0),
		jogoTabela(8, 2, 3, "SCHEDULED", 0, 0),
	}
}

func TestChancesTituloERebaixamentoDecididos(t *testing.T) {
	c := simula(t, chancesRules, ligaQuaseNoFim(), time.Now())
	if c.RemainingMatches != 2 || c.Simulations != 10000 {
		t.Errorf("esperado 2 jogos por jogar e 10 mil simulações, veio %d e %d", c.RemainingMatches, c.Simulations)
	}
	if !reflect.DeepEqual(c.Zones, []string{"libertadores", "rebaixamento"}) {
		t.Errorf("zonas: veio %v", c.Zones)
	}
	if lider := chancesDo(t, c, 1); lider.Title != 100 || lider.Zones["libertadores"] != 100 || lider.Position != 1 {
		t.Errorf("o líder já é campeão: %+v", lider)
	}
	if lanterna := chancesDo(t, c, 4); lanterna.Zones["rebaixamento"] != 100 || lanterna.Positions[3] != 100 || lanterna.Title != 0 {
		t.Errorf("o lanterna já caiu: %+v", lanterna)
	}
	if c.Teams[0].TeamID != 1 || c.Teams[3].TeamID != 4 {
		t.Errorf("os times vêm na ordem da classificação de agora: %v", c.Teams)
	}
}

func TestChancesSomam100(t *testing.T) {
	c := simula(t, chancesRules, ligaQuaseNoFim(), time.Now())
	n := len(c.Teams)
	porPosicao := make([]float64, n)
	for _, tc := range c.Teams {
		soma := 0.0
		for p, v := range tc.Positions {
			soma += v
			porPosicao[p] += v
		}
		if math.Abs(soma-100) > 0.1*float64(n) {
			t.Errorf("as posições do time %d somam %.1f%%", tc.TeamID, soma)
		}
	}
	for p, soma := range porPosicao {
		if math.Abs(soma-100) > 0.1*float64(n) {
			t.Errorf("a %dª posição soma %.1f%% entre os times", p+1, soma)
		}
	}
}

// A mesma tabela dá as mesmas chances, mesmo com os jogos vindo em outra ordem
func TestChancesMesmaTabelaMesmoResultado(t *testing.T) {
	jogos := ligaQuaseNoFim()
	a := simula(t, chancesRules, jogos, time.Now())
	slices.Reverse(jogos)
	b := simula(t, chancesRules, jogos, time.Now())
	if !reflect.DeepEqual(a, b) {
		t.Errorf("a mesma tabela deu chances diferentes:\n%+v\n%+v", a.Teams, b.Teams)
	}
}

// Dois times iguais e um jogo: quem joga em casa leva vantagem. Com as médias padrão (1,4 x
// 1,1 gol), o mandante vence 43,8% e empata 26,6% (o empate em tudo é sorteado): 57%
func TestChancesMandoDeCampo(t *testing.T) {
	c := simula(t, chancesRules, []models.Match{jogoTabela(1, 1, 2, "TIMED", 0, 0)}, time.Now())
	casa, fora := chancesDo(t, c, 1), chancesDo(t, c, 2)
	if casa.Title < 54 || casa.Title > 60 {
		t.Errorf("mandante: esperado ~57%% de título, veio %.1f%%", casa.Title)
	}
	if math.Abs(casa.Title+fora.Title-100) > 0.11 {
		t.Errorf("título soma %.1f%%", casa.Title+fora.Title)
	}
}

// Jogo em andamento: parte do placar de agora e só simula o tempo que falta
func TestChancesJogoEmAndamento(t *testing.T) {
	jogo := jogoTabela(1, 1, 2, "IN_PLAY", 3, 0)
	agora := time.Date(2026, 10, 1, 23, 0, 0, 0, time.UTC) // 3 h depois do início: não falta nada
	c := simula(t, chancesRules, []models.Match{jogo}, agora)
	if casa := chancesDo(t, c, 1); casa.Title != 100 || casa.ExpectedPoints != 3 {
		t.Errorf("3 x 0 sem tempo para virar: esperado título certo e 3 pontos, veio %+v", casa)
	}

	jogo.HomeScore = 0
	c = simula(t, chancesRules, []models.Match{jogo}, time.Date(2026, 10, 1, 20, 45, 0, 0, time.UTC))
	if casa := chancesDo(t, c, 1); casa.Title > 57 || casa.Title < 50 {
		// 0 x 0 no meio do 1º tempo: menos tempo para o mando de campo pesar do que no jogo todo
		t.Errorf("0 x 0 aos 45 min: esperado entre 50%% e 57%%, veio %.1f%%", casa.Title)
	}
}

// Temporada encerrada: a classificação de agora é a final, com o confronto direto
func TestChancesTemporadaEncerrada(t *testing.T) {
	jogos := ligaQuaseNoFim()
	jogos[6].Status, jogos[6].HomeScore, jogos[6].AwayScore = "FINISHED", 0, 1
	jogos[7].Status, jogos[7].HomeScore, jogos[7].AwayScore = "FINISHED", 2, 0
	c := simula(t, chancesRules, jogos, time.Now())
	if c.Simulations != 1 || c.RemainingMatches != 0 {
		t.Errorf("sem jogos por jogar: esperado 1 simulação, veio %d (faltam %d)", c.Simulations, c.RemainingMatches)
	}
	for _, tc := range c.Teams {
		if tc.Positions[tc.Position-1] != 100 || tc.ExpectedPoints != float64(tc.Points) {
			t.Errorf("time %d: a posição de agora é a final, veio %+v", tc.TeamID, tc)
		}
	}
}

// Campeão de copa tem vaga garantida, onde estiver na tabela
func TestChancesCampeaoDeCopaTemVaga(t *testing.T) {
	rules := chancesRules
	rules.winners = []models.Winner{{Competition: "copa_brasil", TeamName: "Time 3"}}
	c := simula(t, rules, ligaQuaseNoFim(), time.Now())
	if campeao := chancesDo(t, c, 3); campeao.Zones["libertadores"] != 100 {
		t.Errorf("campeão da copa: esperado 100%% de vaga, veio %+v", campeao.Zones)
	}
}

func TestChancesCopaNaoTemTabela(t *testing.T) {
	for _, league := range []string{"CLI", "WC", "UNL"} {
		if _, err := LeagueChancesFor(context.Background(), league, "2026"); !errors.Is(err, ErrNoLeagueTable) {
			t.Errorf("%s: esperado ErrNoLeagueTable, veio %v", league, err)
		}
	}
}
