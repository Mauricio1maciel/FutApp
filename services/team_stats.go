package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
)

// Estatísticas do time por competição: a campanha sai dos jogos (qualquer temporada); as
// médias de jogo e os artilheiros, dos jogos que a ESPN detalha (temporada atual).

type RecordSplit struct {
	Played       int `json:"played"`
	Wins         int `json:"wins"`
	Draws        int `json:"draws"`
	Losses       int `json:"losses"`
	GoalsFor     int `json:"goals_for"`
	GoalsAgainst int `json:"goals_against"`
}

type MatchRef struct {
	MatchID     int64  `json:"match_id"`
	ESPNMatchID string `json:"espn_match_id"`
	Date        string `json:"date"`
	HomeTeam    string `json:"home_team"`
	AwayTeam    string `json:"away_team"`
	HomeScore   int    `json:"home_score"`
	AwayScore   int    `json:"away_score"`
}

type TeamAverages struct {
	Matches       int     `json:"matches"` // jogos com estatísticas
	Possession    float64 `json:"possession"`
	Shots         float64 `json:"shots"`
	ShotsOnTarget float64 `json:"shots_on_target"`
	Corners       float64 `json:"corners"`
	Fouls         float64 `json:"fouls"`
	YellowCards   float64 `json:"yellow_cards"`
	RedCards      float64 `json:"red_cards"`
	PassAccuracy  float64 `json:"pass_accuracy"` // %
}

type TeamCompetitionStats struct {
	League     string `json:"league"`
	LeagueName string `json:"league_name"`
	Season     string `json:"season"`
	RecordSplit
	Home          RecordSplit           `json:"home"`
	Away          RecordSplit           `json:"away"`
	CleanSheets   int                   `json:"clean_sheets"`    // jogos sem sofrer gol
	FailedToScore int                   `json:"failed_to_score"` // jogos sem marcar
	BiggestWin    *MatchRef             `json:"biggest_win"`
	BiggestLoss   *MatchRef             `json:"biggest_loss"`
	Averages      *TeamAverages         `json:"averages"` // null sem jogos detalhados
	TopScorers    []database.TeamScorer `json:"top_scorers"`
}

type TeamStats struct {
	TeamID       int64                  `json:"team_id"`
	Competitions []TeamCompetitionStats `json:"competitions"`
}

// GetTeamStats calcula as estatísticas do time (api_id). season vazio = a temporada atual
// de cada competição em que ele joga.
func GetTeamStats(ctx context.Context, teamID int64, season string) (TeamStats, error) {
	stats := TeamStats{TeamID: teamID, Competitions: []TeamCompetitionStats{}}
	var matches []models.Match
	var err error
	if season == "" {
		matches, err = database.GetMatchesByTeamID(ctx, teamID, nil)
	} else {
		matches, err = database.GetTeamMatchesInSeason(ctx, teamID, season)
	}
	if err != nil {
		return stats, err
	}
	leagueNames, _ := database.GetLeagueNames(ctx)

	type key struct{ league, season string }
	comps := map[key]*TeamCompetitionStats{}
	for _, m := range matches {
		if m.Status != "FINISHED" {
			continue
		}
		k := key{m.League, m.Season}
		c, ok := comps[k]
		if !ok {
			c = &TeamCompetitionStats{League: m.League, LeagueName: leagueNames[m.League], Season: m.Season,
				TopScorers: []database.TeamScorer{}}
			comps[k] = c
		}
		addTeamMatch(c, teamID, m)
	}

	// Médias e artilheiros: pelos jogos detalhados pela ESPN
	if espnStr, err := database.GetESPNTeamID(ctx, teamID); err == nil && espnStr != "" {
		espnID, _ := strconv.ParseInt(espnStr, 10, 64)
		if rows, err := database.GetTeamMatchStatistics(ctx, espnID); err == nil {
			sums := map[key]*TeamAverages{}
			for _, r := range rows {
				if comps[key{r.League, r.Season}] == nil {
					continue
				}
				var s models.MatchStatistics
				if json.Unmarshal(r.Raw, &s) != nil {
					continue
				}
				side := s.Away
				if r.Home {
					side = s.Home
				}
				a := sums[key{r.League, r.Season}]
				if a == nil {
					a = &TeamAverages{}
					sums[key{r.League, r.Season}] = a
				}
				a.Matches++
				a.Possession += side["possessionPct"]
				a.Shots += side["totalShots"]
				a.ShotsOnTarget += side["shotsOnTarget"]
				a.Corners += side["wonCorners"]
				a.Fouls += side["foulsCommitted"]
				a.YellowCards += side["yellowCards"]
				a.RedCards += side["redCards"]
				if side["totalPasses"] > 0 {
					a.PassAccuracy += side["accuratePasses"] / side["totalPasses"] * 100
				}
			}
			for k, a := range sums {
				n := float64(a.Matches)
				for _, v := range []*float64{&a.Possession, &a.Shots, &a.ShotsOnTarget, &a.Corners, &a.Fouls,
					&a.YellowCards, &a.RedCards, &a.PassAccuracy} {
					*v = math.Round(*v/n*10) / 10
				}
				comps[k].Averages = a
			}
		}
		if scorers, err := database.GetTeamScorers(ctx, espnID); err == nil {
			for _, s := range scorers {
				if c := comps[key{s.League, s.Season}]; c != nil && len(c.TopScorers) < 5 {
					c.TopScorers = append(c.TopScorers, s)
				}
			}
		}
	}

	for _, c := range comps {
		stats.Competitions = append(stats.Competitions, *c)
	}
	sort.SliceStable(stats.Competitions, func(i, j int) bool {
		a, b := stats.Competitions[i], stats.Competitions[j]
		if a.Played != b.Played {
			return a.Played > b.Played
		}
		return a.League < b.League
	})
	return stats, nil
}

// addTeamMatch soma um jogo encerrado na campanha. Decisão por pênaltis conta como empate.
func addTeamMatch(c *TeamCompetitionStats, teamID int64, m models.Match) {
	home := m.APIHomeTeamID == teamID
	gf, ga := m.HomeScore, m.AwayScore
	split := &c.Home
	if !home {
		gf, ga = ga, gf
		split = &c.Away
	}
	for _, r := range []*RecordSplit{&c.RecordSplit, split} {
		r.Played++
		r.GoalsFor += gf
		r.GoalsAgainst += ga
		switch {
		case gf > ga:
			r.Wins++
		case gf < ga:
			r.Losses++
		default:
			r.Draws++
		}
	}
	if ga == 0 {
		c.CleanSheets++
	}
	if gf == 0 {
		c.FailedToScore++
	}
	ref := &MatchRef{MatchID: m.MatchID, ESPNMatchID: m.IDEvent, Date: m.DateEvent, HomeTeam: m.HomeTeam,
		AwayTeam: m.AwayTeam, HomeScore: m.HomeScore, AwayScore: m.AwayScore}
	diff := gf - ga
	if diff > 0 && (c.BiggestWin == nil || diff > scoreMargin(c.BiggestWin)) {
		c.BiggestWin = ref
	}
	if diff < 0 && (c.BiggestLoss == nil || -diff > scoreMargin(c.BiggestLoss)) {
		c.BiggestLoss = ref
	}
}

// scoreMargin devolve a diferença de gols do jogo guardado
func scoreMargin(r *MatchRef) int {
	d := r.HomeScore - r.AwayScore
	if d < 0 {
		d = -d
	}
	return d
}
