package services

import (
	"App-Futebol/database"
	"context"
	"errors"
	"fmt"
	"sort"
)

// Pendurados e suspensos. Regra das competições da CBF (Séries A e B): o 3º amarelo (e o
// 6º, o 9º...) e o cartão vermelho tiram o jogador da partida seguinte da competição. No
// jogo da expulsão, os amarelos não entram na conta do acúmulo (o segundo amarelo vira o
// vermelho); dois amarelos no mesmo jogo contam como expulsão (a ESPN às vezes não grava o
// vermelho). Gancho do STJD, de mais de um jogo, não aparece nos dados da ESPN.

// ErrNoCardRule: a regra de cartões da competição ainda não está cadastrada
var ErrNoCardRule = errors.New("regra de cartões da competição não cadastrada")

// Amarelos que suspendem, por competição. As ligas europeias têm regras próprias (5
// amarelos, zerando no meio da temporada) e ficam para depois
var yellowsForSuspension = map[string]int{"BSA": 3, "BSB": 3}

type PlayerCards struct {
	Name         string `json:"name"`
	ESPNPlayerID int64  `json:"espn_player_id,omitempty"` // o mesmo do /player/stats
	YellowCards  int    `json:"yellow_cards"`             // os que contam para o acúmulo
	RedCards     int    `json:"red_cards"`
	Status       string `json:"status,omitempty"` // suspended | hanging
	Reason       string `json:"reason,omitempty"` // da suspensão: "3º amarelo" | "cartão vermelho"
}

type TeamCards struct {
	TeamID               int64               `json:"team_id"`
	League               string              `json:"league"`
	Season               string              `json:"season"`
	YellowsForSuspension int                 `json:"yellows_for_suspension"`
	MatchesCounted       int                 `json:"matches_counted"`      // jogos encerrados com os lances da ESPN
	MatchesWithoutData   int                 `json:"matches_without_data"` // jogos encerrados sem os lances (cartões fora da conta)
	NextMatch            *database.NextMatch `json:"next_match"`           // o jogo da suspensão; null sem jogo marcado
	Suspended            []PlayerCards       `json:"suspended"`
	Hanging              []PlayerCards       `json:"hanging"`
	Players              []PlayerCards       `json:"players"` // todos com cartão na competição
}

// HasCardRule diz se a competição tem a regra de cartões cadastrada
func HasCardRule(league string) bool {
	_, ok := yellowsForSuspension[league]
	return ok
}

// TeamCardsFor calcula os pendurados e suspensos do time na competição
func TeamCardsFor(ctx context.Context, league, season string, teamID int64) (*TeamCards, error) {
	n, ok := yellowsForSuspension[league]
	if !ok {
		return nil, ErrNoCardRule
	}
	matches, next, err := database.GetTeamCards(ctx, league, season, teamID)
	if err != nil {
		return nil, err
	}
	out := computeTeamCards(matches, n)
	out.TeamID, out.League, out.Season, out.NextMatch = teamID, league, season, next
	return out, nil
}

// computeTeamCards aplica a regra aos jogos encerrados, em ordem. Suspenso é quem completou
// a suspensão no último jogo do time: cumpre no próximo
func computeTeamCards(matches []database.CardMatch, n int) *TeamCards {
	type player struct {
		cards       PlayerCards
		suspendedAt int // índice do jogo que gerou a última suspensão (-1: nenhum)
		reason      string
	}
	players := map[string]*player{}
	var order []string
	out := &TeamCards{YellowsForSuspension: n, Suspended: []PlayerCards{}, Hanging: []PlayerCards{}, Players: []PlayerCards{}}

	for i, m := range matches {
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
			switch {
			case pm.red || pm.yellows >= 2:
				p.cards.RedCards++
				p.suspendedAt, p.reason = i, "cartão vermelho"
			case pm.yellows == 1:
				p.cards.YellowCards++
				if p.cards.YellowCards%n == 0 {
					p.suspendedAt, p.reason = i, fmt.Sprintf("%dº amarelo", p.cards.YellowCards)
				}
			}
		}
	}

	last := len(matches) - 1
	for _, name := range order {
		p := players[name]
		c := p.cards
		switch {
		case p.suspendedAt >= 0 && p.suspendedAt == last:
			c.Status, c.Reason = "suspended", p.reason
			out.Suspended = append(out.Suspended, c)
		case c.YellowCards%n == n-1:
			c.Status = "hanging"
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
		if a.YellowCards != b.YellowCards {
			return a.YellowCards > b.YellowCards
		}
		if a.RedCards != b.RedCards {
			return a.RedCards > b.RedCards
		}
		return a.Name < b.Name
	})
	return out
}
