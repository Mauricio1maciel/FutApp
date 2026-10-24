package services

import (
	"App-Futebol/database"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
)

// Pendurados e suspensos, com a regra de cartões de cada competição (temporada 2026/27 nas
// europeias e 2026 nas sul-americanas e na Copa do Mundo), conferida nos regulamentos e no
// noticiário em outubro de 2026.
//
// Em todas: a expulsão (vermelho direto, ou o segundo amarelo) suspende o jogo seguinte; o
// tribunal pode aumentar a pena, e isso não aparece nos dados da ESPN. No jogo da expulsão,
// os amarelos não entram na conta do acúmulo; dois amarelos no mesmo jogo contam como
// expulsão (a ESPN às vezes não grava o vermelho).

// ErrNoCardRule: a regra de cartões da competição não está cadastrada
var ErrNoCardRule = errors.New("regra de cartões da competição não cadastrada")

// cardRule é a regra de suspensão por cartões de uma competição
type cardRule struct {
	description string // para mostrar no app
	note        string // limite dos nossos dados para esta regra, para o app avisar
	firstBan    int    // o primeiro amarelo que suspende
	// ban diz por quantos jogos suspende o amarelo número n (contado desde o último
	// zeramento), recebido no jogo número match do time na competição (1 = o primeiro).
	// 0 = não suspende
	ban func(n, match int) int
	// Fases depois das quais os amarelos que não suspenderam zeram
	resetAfter []string
	// O zeramento também cancela a suspensão por amarelo ainda não cumprida
	resetCancelsPending bool
}

func (r cardRule) resetsAfter(stage string) bool { return slices.Contains(r.resetAfter, stage) }

// everyN: o amarelo n, 2n, 3n... suspende um jogo
func everyN(n int) func(int, int) int {
	return func(yellows, _ int) int {
		if yellows%n == 0 {
			return 1
		}
		return 0
	}
}

// Premier League: 5 amarelos até o 19º jogo do time = 1 jogo; 10 até o 32º = 2 jogos;
// 15 = 3 jogos
func premierLeagueBan(n, match int) int {
	switch {
	case n == 5 && match <= 19:
		return 1
	case n == 10 && match <= 32:
		return 2
	case n == 15:
		return 3
	}
	return 0
}

// Serie A: o 5º, o 10º, o 14º e o 17º amarelo e, do 19º em diante, cada amarelo
func serieABan(n, _ int) int {
	if n == 5 || n == 10 || n == 14 || n == 17 || n >= 19 {
		return 1
	}
	return 0
}

// Competições de clubes da UEFA desde 2026/27: o 4º amarelo e, depois, um a cada 2
func uefaClubBan(n, _ int) int {
	if n == 4 || (n > 4 && n%2 == 0) {
		return 1
	}
	return 0
}

var (
	cbfRule      = cardRule{description: "3º, 6º, 9º... amarelo: 1 jogo", firstBan: 3, ban: everyN(3)}
	conmebolRule = cardRule{description: "3º, 6º, 9º... amarelo: 1 jogo; os amarelos zeram antes da semifinal",
		firstBan: 3, ban: everyN(3), resetAfter: []string{"QUARTER_FINALS"}}
	fiveYellowsRule = cardRule{description: "5º, 10º, 15º... amarelo: 1 jogo", firstBan: 5, ban: everyN(5)}
)

var cardRules = map[string]cardRule{
	"BSA": cbfRule,
	"BSB": cbfRule,
	"CDB": {description: cbfRule.description, firstBan: 3, ban: everyN(3),
		note: "Regra da edição de 2025 (3º amarelo, sem zerar antes da final): a de 2026 não foi encontrada. " +
			"Os jogos das primeiras fases, sem os lances da ESPN, ficam fora da conta."},
	"CLI": conmebolRule,
	"CSU": conmebolRule,
	"PL": {description: "5 amarelos até o 19º jogo: 1 jogo; 10 até o 32º: 2 jogos; 15: 3 jogos",
		firstBan: 5, ban: premierLeagueBan,
		note: "Na Inglaterra, os amarelos da FA Cup e da Copa da Liga também contam, e esses jogos não estão no app: " +
			"a conta pode ficar abaixo da real."},
	"PD":  fiveYellowsRule,
	"BL1": fiveYellowsRule,
	"FL1": fiveYellowsRule, // desde 2025/26 (antes eram 3 amarelos em 10 jogos)
	"SA": {description: "5º, 10º, 14º e 17º amarelo e, do 19º em diante, cada amarelo: 1 jogo",
		firstBan: 5, ban: serieABan},
	"CL": {description: "4º amarelo e depois a cada 2 (6º, 8º...): 1 jogo; os amarelos zeram depois das quartas",
		firstBan: 4, ban: uefaClubBan, resetAfter: []string{"QUARTER_FINALS"}},
	"WC": {description: "2º, 4º... amarelo: 1 jogo; os amarelos zeram depois da fase de grupos e das quartas",
		firstBan: 2, ban: everyN(2), resetAfter: []string{"GROUP_STAGE", "QUARTER_FINALS"}},
	"UNL": {description: "2º, 4º... amarelo: 1 jogo; amarelos e suspensões por amarelo zeram depois das quartas",
		firstBan: 2, ban: everyN(2), resetAfter: []string{"QUARTER_FINALS"}, resetCancelsPending: true},
}

type PlayerCards struct {
	Name             string `json:"name"`
	ESPNPlayerID     int64  `json:"espn_player_id,omitempty"` // o mesmo do /player/stats
	YellowCards      int    `json:"yellow_cards"`             // os que contam para a suspensão (desde o último zeramento)
	TotalYellowCards int    `json:"total_yellow_cards"`       // todos na competição
	RedCards         int    `json:"red_cards"`
	Status           string `json:"status,omitempty"`         // suspended | hanging
	Reason           string `json:"reason,omitempty"`         // da suspensão: "3º amarelo", "segundo amarelo", "cartão vermelho"
	MatchesBanned    int    `json:"matches_banned,omitempty"` // suspenso: jogos que ainda vai cumprir
	// Vermelho direto: a suspensão automática é de 1 jogo, mas o tribunal costuma dar mais
	// (na Inglaterra, 3 jogos por jogada violenta), e isso não aparece nos dados da ESPN
	MayBeLonger bool `json:"may_be_longer,omitempty"`
	BanIfBooked int  `json:"ban_if_booked,omitempty"` // pendurado: jogos de suspensão se levar o próximo amarelo
}

type TeamCards struct {
	TeamID               int64               `json:"team_id"`
	League               string              `json:"league"`
	Season               string              `json:"season"`
	Rule                 string              `json:"rule"`
	Note                 string              `json:"note,omitempty"`
	YellowsForSuspension int                 `json:"yellows_for_suspension"` // o primeiro amarelo que suspende
	MatchesCounted       int                 `json:"matches_counted"`        // jogos encerrados com os lances da ESPN
	MatchesWithoutData   int                 `json:"matches_without_data"`   // jogos encerrados sem os lances (cartões fora da conta)
	NextMatch            *database.NextMatch `json:"next_match"`             // o jogo da suspensão; null sem jogo marcado
	Suspended            []PlayerCards       `json:"suspended"`
	Hanging              []PlayerCards       `json:"hanging"`
	Players              []PlayerCards       `json:"players"` // todos com cartão na competição
}

// HasCardRule diz se a competição tem a regra de cartões cadastrada
func HasCardRule(league string) bool {
	_, ok := cardRules[league]
	return ok
}

// TeamCardsFor calcula os pendurados e suspensos do time na competição
func TeamCardsFor(ctx context.Context, league, season string, teamID int64) (*TeamCards, error) {
	rule, ok := cardRules[league]
	if !ok {
		return nil, ErrNoCardRule
	}
	matches, next, err := database.GetTeamCards(ctx, league, season, teamID)
	if err != nil {
		return nil, err
	}
	out := computeTeamCards(matches, rule, next)
	out.TeamID, out.League, out.Season, out.NextMatch = teamID, league, season, next
	return out, nil
}

// computeTeamCards aplica a regra aos jogos encerrados, em ordem. Suspenso é quem ainda tem
// jogo de suspensão a cumprir depois do último jogo do time; pendurado, quem pega suspensão
// se levar o próximo amarelo (no próximo jogo, depois de um zeramento, se houver)
func computeTeamCards(matches []database.CardMatch, rule cardRule, next *database.NextMatch) *TeamCards {
	type player struct {
		cards       PlayerCards
		active      int // amarelos que contam desde o último zeramento
		suspendedAt int // jogo que gerou a última suspensão (-1: nenhuma)
		ban         int // jogos dessa suspensão
		byYellow    bool
		reason      string
	}
	players := map[string]*player{}
	var order []string
	out := &TeamCards{Rule: rule.description, Note: rule.note, YellowsForSuspension: rule.firstBan,
		Suspended: []PlayerCards{}, Hanging: []PlayerCards{}, Players: []PlayerCards{}}
	reset := func() {
		for _, p := range players {
			p.active = 0
			if rule.resetCancelsPending && p.byYellow {
				p.ban = 0
			}
		}
	}

	for i, m := range matches {
		if i > 0 && m.Stage != matches[i-1].Stage && rule.resetsAfter(matches[i-1].Stage) {
			reset()
		}
		if !m.HasData {
			out.MatchesWithoutData++
			continue
		}
		out.MatchesCounted++

		// Cartões do jogo por jogador
		type inMatch struct {
			yellows int
			red     bool
			id      int64
		}
		byPlayer := map[string]*inMatch{}
		var names []string
		for _, c := range m.Cards {
			pm := byPlayer[c.Player]
			if pm == nil {
				pm = &inMatch{}
				byPlayer[c.Player] = pm
				names = append(names, c.Player)
			}
			if c.PlayerID != 0 {
				pm.id = c.PlayerID
			}
			if c.Type == "Yellow Card" {
				pm.yellows++
			} else {
				pm.red = true
			}
		}

		for _, name := range names {
			pm := byPlayer[name]
			p := players[name]
			if p == nil {
				p = &player{cards: PlayerCards{Name: name}, suspendedAt: -1}
				players[name] = p
				order = append(order, name)
			}
			if pm.id != 0 {
				p.cards.ESPNPlayerID = pm.id
			}
			p.cards.TotalYellowCards += pm.yellows
			switch {
			case pm.red || pm.yellows >= 2:
				p.cards.RedCards++
				p.suspendedAt, p.ban, p.byYellow, p.reason = i, 1, false, "cartão vermelho"
				if pm.yellows >= 1 {
					p.reason = "segundo amarelo"
				}
			case pm.yellows == 1:
				p.active++
				if b := rule.ban(p.active, i+1); b > 0 {
					p.suspendedAt, p.ban, p.byYellow, p.reason = i, b, true, fmt.Sprintf("%dº amarelo", p.active)
				}
			}
		}
	}

	// O próximo jogo é de uma fase depois de uma que zera os amarelos
	last := len(matches) - 1
	if next != nil && last >= 0 && next.Stage != matches[last].Stage && rule.resetsAfter(matches[last].Stage) {
		reset()
	}

	nextMatch := len(matches) + 1
	for _, name := range order {
		p := players[name]
		c := p.cards
		c.YellowCards = p.active
		remaining := 0
		if p.suspendedAt >= 0 {
			remaining = p.ban - (last - p.suspendedAt)
		}
		if remaining > 0 {
			c.Status, c.Reason, c.MatchesBanned = "suspended", p.reason, remaining
			c.MayBeLonger = p.reason == "cartão vermelho"
			out.Suspended = append(out.Suspended, c)
		} else if b := rule.ban(p.active+1, nextMatch); b > 0 {
			c.Status, c.BanIfBooked = "hanging", b
			out.Hanging = append(out.Hanging, c)
		}
		out.Players = append(out.Players, c)
	}
	byName := func(list []PlayerCards) {
		sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	}
	byName(out.Suspended)
	byName(out.Hanging)
	sort.SliceStable(out.Players, func(i, j int) bool {
		a, b := out.Players[i], out.Players[j]
		if a.TotalYellowCards != b.TotalYellowCards {
			return a.TotalYellowCards > b.TotalYellowCards
		}
		if a.RedCards != b.RedCards {
			return a.RedCards > b.RedCards
		}
		return a.Name < b.Name
	})
	return out
}
