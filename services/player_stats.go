package services

import (
	"App-Futebol/database"
	"context"
	"sort"
	"strconv"
	"strings"
)

// Estatísticas do jogador, calculadas das escalações e dos lances dos jogos que a ESPN
// detalha (temporada atual das ligas acompanhadas). Assistência: no lance de gol, details
// é quem deu o passe. Os eventos guardam o nome do jogador;
// dentro de um jogo ele bate 100% com o nome na escalação, que tem o ID.

type PlayerTotals struct {
	Appearances int `json:"appearances"`
	Starts      int `json:"starts"`
	Minutes     int `json:"minutes"` // aproximado: 90 por jogo (120 com prorrogação), menos o tempo fora
	Goals       int `json:"goals"`
	Penalties   int `json:"penalties"` // gols de pênalti (já contados em goals)
	OwnGoals    int `json:"own_goals"`
	Assists     int `json:"assists"`
	YellowCards int `json:"yellow_cards"`
	RedCards    int `json:"red_cards"`
}

type PlayerCompetitionStats struct {
	League     string           `json:"league"`
	LeagueName string           `json:"league_name"`
	Season     string           `json:"season"`
	Team       database.TeamRef `json:"team"`
	PlayerTotals
}

type PlayerMatchLine struct {
	ESPNMatchID  int64            `json:"espn_match_id"`
	League       string           `json:"league"`
	Season       string           `json:"season"`
	Date         string           `json:"date"`
	Team         database.TeamRef `json:"team"`
	Opponent     database.TeamRef `json:"opponent"`
	Home         bool             `json:"home"`
	GoalsFor     int              `json:"goals_for"`
	GoalsAgainst int              `json:"goals_against"`
	Result       string           `json:"result"` // V, E ou D (decisão por pênaltis conta como E)
	Started      bool             `json:"started"`
	Minutes      int              `json:"minutes"`
	Goals        int              `json:"goals"`
	Penalties    int              `json:"penalties"`
	OwnGoals     int              `json:"own_goals"`
	Assists      int              `json:"assists"`
	YellowCards  int              `json:"yellow_cards"`
	RedCards     int              `json:"red_cards"`
	SubbedIn     string           `json:"subbed_in,omitempty"`  // minuto em que entrou
	SubbedOut    string           `json:"subbed_out,omitempty"` // minuto em que saiu
}

type PlayerStats struct {
	PlayerID     int64                    `json:"player_id"`
	Current      PlayerTotals             `json:"current"` // soma da temporada atual de cada competição
	Competitions []PlayerCompetitionStats `json:"competitions"`
	LastMatches  []PlayerMatchLine        `json:"last_matches"` // até 10, do mais recente
}

// GetPlayerStats calcula as estatísticas do jogador (ID da ESPN). season vazio = todas.
func GetPlayerStats(ctx context.Context, espnPlayerID int64, season string) (PlayerStats, error) {
	stats := PlayerStats{PlayerID: espnPlayerID, Competitions: []PlayerCompetitionStats{}, LastMatches: []PlayerMatchLine{}}
	apps, err := database.GetPlayerAppearances(ctx, espnPlayerID, season)
	if err != nil || len(apps) == 0 {
		return stats, err
	}
	ids := make([]int64, 0, len(apps))
	teamIDs := []int64{}
	for _, a := range apps {
		ids = append(ids, a.ESPNMatchID)
		teamIDs = append(teamIDs, a.HomeESPNID, a.AwayESPNID)
	}
	events, err := database.GetStatEvents(ctx, ids)
	if err != nil {
		return stats, err
	}
	byMatch := map[int64][]database.MatchEvent{}
	for _, e := range events {
		byMatch[e.ESPNMatchID] = append(byMatch[e.ESPNMatchID], e)
	}
	teams, err := database.GetTeamsByESPNIDs(ctx, teamIDs)
	if err != nil {
		return stats, err
	}
	leagueNames, _ := database.GetLeagueNames(ctx)

	type compKey struct {
		league, season string
		team           int64
	}
	comps := map[compKey]*PlayerCompetitionStats{}
	var order []compKey
	var lines []PlayerMatchLine
	for _, a := range apps {
		line, played := playerMatchLine(a, byMatch[a.ESPNMatchID])
		if !played {
			continue // ficou no banco
		}
		opp := a.AwayESPNID
		if !line.Home {
			opp = a.HomeESPNID
		}
		line.Team, line.Opponent = teams[a.ESPNTeamID], teams[opp]
		lines = append(lines, line)

		k := compKey{a.League, a.Season, a.ESPNTeamID}
		c, ok := comps[k]
		if !ok {
			c = &PlayerCompetitionStats{League: a.League, LeagueName: leagueNames[a.League], Season: a.Season, Team: teams[a.ESPNTeamID]}
			comps[k] = c
			order = append(order, k)
		}
		addLine(&c.PlayerTotals, line)
	}

	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Date > lines[j].Date })
	if len(lines) > 10 {
		lines = lines[:10]
	}
	stats.LastMatches = lines

	for _, k := range order {
		c := *comps[k]
		stats.Competitions = append(stats.Competitions, c)
		if c.Season == ResolveSeason(ctx, c.League, "") {
			sumTotals(&stats.Current, c.PlayerTotals)
		}
	}
	sort.SliceStable(stats.Competitions, func(i, j int) bool {
		a, b := stats.Competitions[i], stats.Competitions[j]
		if a.Season != b.Season {
			return a.Season > b.Season
		}
		return a.Appearances > b.Appearances
	})
	return stats, nil
}

func addLine(t *PlayerTotals, l PlayerMatchLine) {
	t.Appearances++
	if l.Started {
		t.Starts++
	}
	t.Minutes += l.Minutes
	t.Goals += l.Goals
	t.Penalties += l.Penalties
	t.OwnGoals += l.OwnGoals
	t.Assists += l.Assists
	t.YellowCards += l.YellowCards
	t.RedCards += l.RedCards
}

func sumTotals(t *PlayerTotals, o PlayerTotals) {
	t.Appearances += o.Appearances
	t.Starts += o.Starts
	t.Minutes += o.Minutes
	t.Goals += o.Goals
	t.Penalties += o.Penalties
	t.OwnGoals += o.OwnGoals
	t.Assists += o.Assists
	t.YellowCards += o.YellowCards
	t.RedCards += o.RedCards
}

// playerMatchLine monta a linha do jogador num jogo. played = false se ele ficou no banco.
// Minutos: titular conta do 0; reserva, do minuto em que entrou; até 90 (120 com
// prorrogação), ou até sair (substituído ou expulso).
func playerMatchLine(a database.PlayerAppearance, events []database.MatchEvent) (PlayerMatchLine, bool) {
	line := PlayerMatchLine{
		ESPNMatchID: a.ESPNMatchID, League: a.League, Season: a.Season, Date: a.MatchDate,
		Home: a.ESPNTeamID == a.HomeESPNID, Started: a.Starter,
	}
	line.GoalsFor, line.GoalsAgainst = a.HomeScore, a.AwayScore
	if !line.Home {
		line.GoalsFor, line.GoalsAgainst = a.AwayScore, a.HomeScore
	}
	switch {
	case line.GoalsFor > line.GoalsAgainst:
		line.Result = "V"
	case line.GoalsFor < line.GoalsAgainst:
		line.Result = "D"
	default:
		line.Result = "E"
	}

	start, out, end := -1, 0, 90
	if a.Starter {
		start = 0
	}
	for _, e := range events {
		switch {
		case e.Type == "Start Extra Time":
			end = 120
		case e.Type == "Substitution" && e.Player == a.PlayerName:
			start, line.SubbedIn = minuteOf(e.Minute), e.Minute
		case e.Type == "Substitution" && e.Details == a.PlayerName:
			out, line.SubbedOut = minuteOf(e.Minute), e.Minute
		case isAssistedGoal(e.Type) && e.Details == a.PlayerName:
			line.Assists++ // no gol, details é quem deu a assistência
		case e.Player != a.PlayerName:
		case e.Type == "Own Goal":
			line.OwnGoals++
		case e.Type == "Penalty - Scored":
			line.Goals++
			line.Penalties++
		case e.Type == "Goal" || strings.HasPrefix(e.Type, "Goal - "):
			line.Goals++
		case e.Type == "Yellow Card":
			line.YellowCards++
		case e.Type == "Red Card":
			line.RedCards++
			if m := minuteOf(e.Minute); out == 0 || m < out {
				out = m
			}
		}
	}
	if start < 0 {
		return line, false
	}
	stop := end
	if out > 0 && out < stop {
		stop = out
	}
	if stop > start {
		line.Minutes = stop - start
	}
	return line, true
}

// minuteOf converte "67'" em 67 e "45'+2'" em 47
func minuteOf(s string) int {
	total := 0
	for _, part := range strings.Split(s, "+") {
		n, _ := strconv.Atoi(strings.Trim(strings.TrimSpace(part), "'"))
		total += n
	}
	return total
}
