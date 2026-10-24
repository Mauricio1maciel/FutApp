package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"sync"
)

// Prévia do jogo: as chances de cada resultado e o que o torcedor olha antes do apito
// (posição, forma recente, confronto direto, suspensos e pendurados, artilheiros), numa
// chamada só.

type ScoreChance struct {
	Home int     `json:"home"`
	Away int     `json:"away"`
	Pct  float64 `json:"pct"`
}

// MatchOdds são as chances do jogo, pelo mesmo modelo das chances da tabela
type MatchOdds struct {
	HomeWin           float64       `json:"home_win"` // %
	Draw              float64       `json:"draw"`
	AwayWin           float64       `json:"away_win"`
	ExpectedHomeGoals float64       `json:"expected_home_goals"`
	ExpectedAwayGoals float64       `json:"expected_away_goals"`
	LikelyScores      []ScoreChance `json:"likely_scores"`    // os 5 placares mais prováveis
	BasedOnMatches    int           `json:"based_on_matches"` // jogos da competição usados
}

type PreviewTeam struct {
	TeamID     int64                 `json:"team_id"`
	Name       string                `json:"name"`
	CrestURL   string                `json:"crest_url"`
	Position   int                   `json:"position,omitempty"` // na tabela (0 = sem tabela)
	Points     int                   `json:"points,omitempty"`
	Played     int                   `json:"played,omitempty"`
	GroupName  string                `json:"group_name,omitempty"`
	Form       string                `json:"form"`        // últimos 5 na competição, do mais antigo ao mais recente
	Suspended  []PlayerCards         `json:"suspended"`   // só Séries A e B
	Hanging    []PlayerCards         `json:"hanging"`     // só Séries A e B
	TopScorers []database.TeamScorer `json:"top_scorers"` // na competição, até 3
}

type PreviewH2H struct {
	Played   int            `json:"played"`
	HomeWins int            `json:"home_wins"` // vitórias do mandante deste jogo, em qualquer campo
	Draws    int            `json:"draws"`
	AwayWins int            `json:"away_wins"`
	Last     []models.Match `json:"last"` // os 5 mais recentes
}

type MatchPreview struct {
	Match         models.Match `json:"match"`
	Probabilities MatchOdds    `json:"probabilities"`
	Home          PreviewTeam  `json:"home"`
	Away          PreviewTeam  `json:"away"`
	H2H           PreviewH2H   `json:"h2h"`
}

// MatchPreviewFor monta a prévia do jogo (ID do nosso banco). Depois de achar o jogo, as
// consultas vão ao banco ao mesmo tempo
func MatchPreviewFor(ctx context.Context, matchID int64) (*MatchPreview, error) {
	match, err := database.GetMatchByID(ctx, matchID)
	if err != nil {
		return nil, err
	}
	home, away := match.APIHomeTeamID, match.APIAwayTeamID
	espnHome, _ := strconv.ParseInt(match.ESPNHomeTeamID, 10, 64)
	espnAway, _ := strconv.ParseInt(match.ESPNAwayTeamID, 10, 64)

	var (
		wg                         sync.WaitGroup
		leagueMatches, h2h         []models.Match
		standings                  []models.Standing
		homeCards, awayCards       *TeamCards
		homeScorers, awayScorers   []database.TeamScorer
		errMatches, errStand, errH error
	)
	run := func(f func()) {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	run(func() {
		leagueMatches, errMatches = database.GetMatchesByLeague(ctx, match.League, match.Season, database.MatchFilter{})
	})
	run(func() { standings, errStand = database.GetStandingsByLeague(ctx, match.League, match.Season) })
	run(func() { h2h, errH = database.GetHeadToHead(ctx, home, away) })
	if HasCardRule(match.League) {
		// Cartões e artilheiros são complemento: sem eles, a prévia sai assim mesmo
		run(func() { homeCards, _ = TeamCardsFor(ctx, match.League, match.Season, home) })
		run(func() { awayCards, _ = TeamCardsFor(ctx, match.League, match.Season, away) })
	}
	if espnHome != 0 {
		run(func() { homeScorers, _ = database.GetTeamScorers(ctx, espnHome) })
	}
	if espnAway != 0 {
		run(func() { awayScorers, _ = database.GetTeamScorers(ctx, espnAway) })
	}
	wg.Wait()
	if err := errors.Join(errMatches, errStand, errH); err != nil {
		return nil, err
	}

	strength := estimateStrength(leagueMatches)
	lh, la := strength.expected(home, away)
	odds := matchOdds(lh, la)
	odds.BasedOnMatches = strength.finished

	form := recentForm(leagueMatches)
	team := func(id int64, name, crest string, cards *TeamCards, scorers []database.TeamScorer) PreviewTeam {
		t := PreviewTeam{TeamID: id, Name: name, CrestURL: crest, Form: form[id],
			Suspended: []PlayerCards{}, Hanging: []PlayerCards{}, TopScorers: []database.TeamScorer{}}
		for _, s := range standings {
			if s.TeamID == id {
				t.Position, t.Points, t.Played, t.GroupName = s.Position, s.Points, s.Played, s.GroupName
				break
			}
		}
		if cards != nil {
			t.Suspended, t.Hanging = cards.Suspended, cards.Hanging
		}
		for _, s := range scorers {
			if s.League == match.League && s.Season == match.Season && len(t.TopScorers) < 3 {
				t.TopScorers = append(t.TopScorers, s)
			}
		}
		return t
	}

	return &MatchPreview{
		Match:         *match,
		Probabilities: odds,
		Home:          team(home, match.HomeTeam, match.HomeLogo, homeCards, homeScorers),
		Away:          team(away, match.AwayTeam, match.AwayLogo, awayCards, awayScorers),
		H2H:           summarizeH2H(h2h, home),
	}, nil
}

// matchOdds: as chances de cada resultado e os placares mais prováveis, com os gols de cada
// lado numa Poisson (somando os placares de 0 a 10 gols, onde está quase toda a chance)
func matchOdds(lh, la float64) MatchOdds {
	const maxGoals = 10
	pmf := func(l float64) []float64 {
		p := make([]float64, maxGoals+1)
		p[0] = math.Exp(-l)
		for k := 1; k <= maxGoals; k++ {
			p[k] = p[k-1] * l / float64(k)
		}
		return p
	}
	ph, pa := pmf(lh), pmf(la)
	var homeWin, draw, awayWin, total float64
	var scores []ScoreChance
	for h := 0; h <= maxGoals; h++ {
		for a := 0; a <= maxGoals; a++ {
			p := ph[h] * pa[a]
			total += p
			switch {
			case h > a:
				homeWin += p
			case h < a:
				awayWin += p
			default:
				draw += p
			}
			scores = append(scores, ScoreChance{Home: h, Away: a, Pct: p})
		}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].Pct > scores[j].Pct })
	pct := func(p float64) float64 { return math.Round(p/total*1000) / 10 }
	odds := MatchOdds{
		HomeWin: pct(homeWin), Draw: pct(draw), AwayWin: pct(awayWin),
		ExpectedHomeGoals: math.Round(lh*100) / 100, ExpectedAwayGoals: math.Round(la*100) / 100,
	}
	for _, s := range scores[:5] {
		odds.LikelyScores = append(odds.LikelyScores, ScoreChance{s.Home, s.Away, pct(s.Pct)})
	}
	return odds
}

// summarizeH2H resume o confronto direto do ponto de vista do mandante deste jogo
func summarizeH2H(matches []models.Match, home int64) PreviewH2H {
	h := PreviewH2H{Last: []models.Match{}}
	for _, m := range matches {
		h.Played++
		homeGoals, awayGoals := m.HomeScore, m.AwayScore
		if m.APIHomeTeamID != home {
			homeGoals, awayGoals = awayGoals, homeGoals
		}
		switch {
		case homeGoals > awayGoals:
			h.HomeWins++
		case homeGoals < awayGoals:
			h.AwayWins++
		default:
			h.Draws++
		}
		if len(h.Last) < 5 {
			h.Last = append(h.Last, m)
		}
	}
	return h
}
