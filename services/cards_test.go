package services

import (
	"App-Futebol/database"
	"reflect"
	"testing"
)

func cartao(jogador, tipo string) database.CardEvent {
	return database.CardEvent{Player: jogador, Type: tipo}
}

// Regra da CBF: 3º amarelo (6º, 9º...) e vermelho suspendem o jogo seguinte
func TestComputeTeamCards(t *testing.T) {
	jogos := []database.CardMatch{
		{MatchID: 1, HasData: true, Cards: []database.CardEvent{
			cartao("Pendurado", "Yellow Card"), cartao("Cumpriu", "Yellow Card"), cartao("Terceiro", "Yellow Card")}},
		{MatchID: 2, HasData: true, Cards: []database.CardEvent{
			cartao("Pendurado", "Yellow Card"), cartao("Cumpriu", "Yellow Card"), cartao("Terceiro", "Yellow Card")}},
		{MatchID: 3, HasData: true, Cards: []database.CardEvent{cartao("Cumpriu", "Yellow Card")}}, // suspenso aqui...
		{MatchID: 4, HasData: false}, // ...cumpriu aqui (sem lances)
		{MatchID: 5, HasData: true, Cards: []database.CardEvent{
			cartao("Terceiro", "Yellow Card"),
			cartao("Expulso", "Yellow Card"), cartao("Expulso", "Red Card"), // o amarelo do jogo da expulsão não conta
			cartao("Dois Amarelos", "Yellow Card"), cartao("Dois Amarelos", "Yellow Card"), // sem o vermelho gravado
			cartao("VAR", "VAR - (Red) Card Upgrade")}},
	}
	c := computeTeamCards(jogos, 3)

	if c.MatchesCounted != 4 || c.MatchesWithoutData != 1 {
		t.Errorf("jogos: esperado 4 contados e 1 sem lances, veio %d e %d", c.MatchesCounted, c.MatchesWithoutData)
	}
	suspensos := map[string]string{}
	for _, p := range c.Suspended {
		suspensos[p.Name] = p.Reason
	}
	if want := map[string]string{"Terceiro": "3º amarelo", "Expulso": "cartão vermelho",
		"Dois Amarelos": "cartão vermelho", "VAR": "cartão vermelho"}; !reflect.DeepEqual(suspensos, want) {
		t.Errorf("suspensos: veio %v, esperado %v", suspensos, want)
	}
	if len(c.Hanging) != 1 || c.Hanging[0].Name != "Pendurado" || c.Hanging[0].YellowCards != 2 {
		t.Errorf("pendurados: esperado só Pendurado (2 amarelos), veio %+v", c.Hanging)
	}
	por := map[string]PlayerCards{}
	for _, p := range c.Players {
		por[p.Name] = p
	}
	if p := por["Cumpriu"]; p.YellowCards != 3 || p.Status != "" {
		t.Errorf("quem já cumpriu a suspensão não é suspenso nem pendurado: %+v", p)
	}
	if p := por["Expulso"]; p.YellowCards != 0 || p.RedCards != 1 {
		t.Errorf("expulso: o amarelo do jogo da expulsão não conta, veio %+v", p)
	}
	if c.Players[0].Name != "Cumpriu" && c.Players[0].Name != "Terceiro" {
		t.Errorf("a lista vem com quem tem mais amarelos primeiro: %+v", c.Players)
	}
}

func TestComputeTeamCardsSemJogos(t *testing.T) {
	c := computeTeamCards(nil, 3)
	if c.Suspended == nil || c.Hanging == nil || c.Players == nil || len(c.Players) != 0 {
		t.Errorf("sem jogos: listas vazias (e não null), veio %+v", c)
	}
}
