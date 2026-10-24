package services

import (
	"App-Futebol/database"
	"reflect"
	"strings"
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
	c := computeTeamCards(jogos, cbfRule, nil)

	if c.MatchesCounted != 4 || c.MatchesWithoutData != 1 {
		t.Errorf("jogos: esperado 4 contados e 1 sem lances, veio %d e %d", c.MatchesCounted, c.MatchesWithoutData)
	}
	suspensos := map[string]string{}
	for _, p := range c.Suspended {
		suspensos[p.Name] = p.Reason
	}
	if want := map[string]string{"Terceiro": "3º amarelo", "Expulso": "segundo amarelo",
		"Dois Amarelos": "segundo amarelo", "VAR": "cartão vermelho"}; !reflect.DeepEqual(suspensos, want) {
		t.Errorf("suspensos: veio %v, esperado %v", suspensos, want)
	}
	if len(c.Hanging) != 1 || c.Hanging[0].Name != "Pendurado" || c.Hanging[0].YellowCards != 2 {
		t.Errorf("pendurados: esperado só Pendurado (2 amarelos), veio %+v", c.Hanging)
	}
	por := map[string]PlayerCards{}
	for _, p := range c.Players {
		por[p.Name] = p
	}
	for _, p := range c.Suspended {
		if p.MayBeLonger != (p.Reason == "cartão vermelho") {
			t.Errorf("só o vermelho direto pode render mais jogos de suspensão: %+v", p)
		}
	}
	if p := por["Cumpriu"]; p.YellowCards != 3 || p.Status != "" {
		t.Errorf("quem já cumpriu a suspensão não é suspenso nem pendurado: %+v", p)
	}
	if p := por["Expulso"]; p.YellowCards != 0 || p.TotalYellowCards != 1 || p.RedCards != 1 {
		t.Errorf("expulso: o amarelo do jogo da expulsão não conta para a suspensão (mas conta no total), veio %+v", p)
	}

	if c.Players[0].Name != "Cumpriu" && c.Players[0].Name != "Terceiro" {
		t.Errorf("a lista vem com quem tem mais amarelos primeiro: %+v", c.Players)
	}
}

func TestComputeTeamCardsSemJogos(t *testing.T) {
	c := computeTeamCards(nil, cbfRule, nil)
	if c.Suspended == nil || c.Hanging == nil || c.Players == nil || len(c.Players) != 0 {
		t.Errorf("sem jogos: listas vazias (e não null), veio %+v", c)
	}
}

// jogos monta os jogos do time, todos com os lances da ESPN, nas fases dadas
func jogos(fases ...string) []database.CardMatch {
	out := make([]database.CardMatch, len(fases))
	for i, f := range fases {
		out[i] = database.CardMatch{MatchID: int64(i + 1), Stage: f, HasData: true}
	}
	return out
}

func amarelo(j []database.CardMatch, jogo int, jogador string) {
	j[jogo-1].Cards = append(j[jogo-1].Cards, cartao(jogador, "Yellow Card"))
}

func situacao(c *TeamCards, jogador string) PlayerCards {
	for _, p := range c.Players {
		if p.Name == jogador {
			return p
		}
	}
	return PlayerCards{}
}

func repete(fase string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fase
	}
	return out
}

func TestObservacaoDaRegra(t *testing.T) {
	if c := computeTeamCards(nil, cardRules["PL"], nil); !strings.Contains(c.Note, "FA Cup") {
		t.Errorf("Premier League avisa que as copas inglesas contam: %q", c.Note)
	}
	if c := computeTeamCards(nil, cardRules["BSA"], nil); c.Note != "" {
		t.Errorf("Série A sem observação: %q", c.Note)
	}
}

func TestTodosOsCampeonatosTemRegra(t *testing.T) {
	for _, league := range []string{"BSA", "BSB", "CDB", "CLI", "CSU", "PL", "PD", "SA", "BL1", "FL1", "CL", "WC", "UNL"} {
		if !HasCardRule(league) || cardRules[league].description == "" || cardRules[league].firstBan == 0 {
			t.Errorf("%s sem regra de cartões completa", league)
		}
	}
}

// Premier League: o 5º amarelo só suspende até o 19º jogo do time; o 10º, até o 32º, por 2
func TestRegraPremierLeague(t *testing.T) {
	j := jogos(repete("REGULAR_SEASON", 30)...)
	for _, n := range []int{1, 2, 3, 4, 5} { // 5º amarelo no 5º jogo: 1 jogo
		amarelo(j, n, "Cedo")
	}
	for _, n := range []int{11, 13, 15, 17, 20} { // 5º amarelo no 20º jogo: nada
		amarelo(j, n, "Tarde")
	}
	for _, n := range []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 29} { // 10º no 29º jogo: 2 jogos
		amarelo(j, n, "Dez")
	}
	c := computeTeamCards(j, cardRules["PL"], nil)
	if p := situacao(c, "Cedo"); p.Status != "" || p.YellowCards != 5 {
		t.Errorf("já cumpriu a suspensão do 5º amarelo: %+v", p)
	}
	if p := situacao(c, "Tarde"); p.Status != "" {
		t.Errorf("5º amarelo depois do 19º jogo não suspende (nem deixa pendurado): %+v", p)
	}
	if p := situacao(c, "Dez"); p.Status != "suspended" || p.MatchesBanned != 1 || p.Reason != "10º amarelo" {
		t.Errorf("10º amarelo no 29º jogo: 2 jogos, falta 1 depois do 30º, veio %+v", p)
	}
	amarelo(j, 30, "Nove") // 9 amarelos até o 30º: pendurado, e pega 2 jogos
	for _, n := range []int{1, 3, 5, 7, 9, 11, 13, 15} {
		amarelo(j, n, "Nove")
	}
	c = computeTeamCards(j, cardRules["PL"], nil)
	if p := situacao(c, "Nove"); p.Status != "hanging" || p.BanIfBooked != 2 {
		t.Errorf("9 amarelos antes do 32º jogo: pendurado, pegando 2 jogos, veio %+v", p)
	}
}

func TestRegraSerieA(t *testing.T) {
	var suspende []int
	for n := 1; n <= 21; n++ {
		if serieABan(n, 0) > 0 {
			suspende = append(suspende, n)
		}
	}
	if want := []int{5, 10, 14, 17, 19, 20, 21}; !reflect.DeepEqual(suspende, want) {
		t.Errorf("Serie A: suspende no %v, esperado %v", suspende, want)
	}
}

// Champions: 4º amarelo e depois a cada 2; os amarelos zeram depois das quartas, mas a
// suspensão já gerada é cumprida na semifinal
func TestRegraChampions(t *testing.T) {
	fases := append(repete("LEAGUE_STAGE", 8), "LAST_16", "LAST_16", "QUARTER_FINALS", "QUARTER_FINALS")
	j := jogos(fases...)
	for _, n := range []int{1, 2, 3, 12} { // 4º amarelo no último jogo das quartas
		amarelo(j, n, "Quarto")
	}
	for _, n := range []int{1, 2, 3} { // 3 amarelos: pendurado... até as quartas acabarem
		amarelo(j, n, "Tres")
	}
	semi := &database.NextMatch{Stage: "SEMI_FINALS"}
	c := computeTeamCards(j, cardRules["CL"], semi)
	if p := situacao(c, "Quarto"); p.Status != "suspended" || p.Reason != "4º amarelo" {
		t.Errorf("4º amarelo nas quartas: cumpre na semifinal, veio %+v", p)
	}
	if p := situacao(c, "Tres"); p.Status != "" || p.YellowCards != 0 {
		t.Errorf("na semifinal os amarelos zeraram: não é mais pendurado, veio %+v", p)
	}
	if c := computeTeamCards(j, cardRules["CL"], &database.NextMatch{Stage: "QUARTER_FINALS"}); situacao(c, "Tres").Status != "hanging" {
		t.Errorf("antes de as quartas acabarem, 3 amarelos é pendurado: %+v", situacao(c, "Tres"))
	}
}

// Liga das Nações: depois das quartas, até a suspensão por amarelo pendente é cancelada
func TestRegraLigaDasNacoes(t *testing.T) {
	j := jogos("league-phase", "league-phase", "QUARTER_FINALS", "QUARTER_FINALS")
	amarelo(j, 2, "Ze")
	amarelo(j, 4, "Ze") // 2º amarelo no último jogo das quartas
	if c := computeTeamCards(j, cardRules["UNL"], &database.NextMatch{Stage: "SEMI_FINALS"}); situacao(c, "Ze").Status != "" {
		t.Errorf("nas finais, a suspensão por amarelo das quartas não vale: %+v", situacao(c, "Ze"))
	}
	j = jogos("league-phase", "league-phase", "league-phase")
	amarelo(j, 1, "Ze")
	amarelo(j, 3, "Ze")
	if c := computeTeamCards(j, cardRules["UNL"], &database.NextMatch{Stage: "league-phase"}); situacao(c, "Ze").Status != "suspended" {
		t.Errorf("2º amarelo na fase de liga suspende: %+v", situacao(c, "Ze"))
	}
}

// Copa do Mundo: os amarelos zeram depois da fase de grupos (a suspensão já gerada vale)
func TestRegraCopaDoMundo(t *testing.T) {
	j := jogos("GROUP_STAGE", "GROUP_STAGE", "GROUP_STAGE", "LAST_32")
	amarelo(j, 1, "Um")
	amarelo(j, 4, "Um") // um nos grupos e um no mata-mata: só 1 conta
	amarelo(j, 2, "Dois")
	amarelo(j, 3, "Dois") // 2º amarelo no último jogo dos grupos: cumpre no mata-mata
	c := computeTeamCards(j[:3], cardRules["WC"], &database.NextMatch{Stage: "LAST_32"})
	if p := situacao(c, "Dois"); p.Status != "suspended" {
		t.Errorf("2º amarelo nos grupos: cumpre no primeiro jogo do mata-mata, veio %+v", p)
	}
	if p := situacao(c, "Um"); p.Status != "" || p.YellowCards != 0 {
		t.Errorf("1 amarelo nos grupos zera no mata-mata: %+v", p)
	}
	c = computeTeamCards(j, cardRules["WC"], &database.NextMatch{Stage: "LAST_16"})
	if p := situacao(c, "Um"); p.Status != "hanging" || p.YellowCards != 1 || p.TotalYellowCards != 2 {
		t.Errorf("1 amarelo no mata-mata (o dos grupos zerou): pendurado, veio %+v", p)
	}
}

// Libertadores: 3º amarelo, e os amarelos zeram antes da semifinal
func TestRegraLibertadores(t *testing.T) {
	j := jogos("GROUP_STAGE", "PLAY_OFFS", "QUARTER_FINALS", "QUARTER_FINALS")
	amarelo(j, 1, "Pendurado")
	amarelo(j, 3, "Pendurado")
	if c := computeTeamCards(j, cardRules["CLI"], &database.NextMatch{Stage: "SEMI_FINALS"}); situacao(c, "Pendurado").Status != "" {
		t.Errorf("na semifinal, 2 amarelos zeraram: %+v", situacao(c, "Pendurado"))
	}
	if c := computeTeamCards(j[:3], cardRules["CLI"], &database.NextMatch{Stage: "QUARTER_FINALS"}); situacao(c, "Pendurado").Status != "hanging" {
		t.Errorf("antes do jogo de volta das quartas, 2 amarelos é pendurado: %+v", situacao(c, "Pendurado"))
	}
}
