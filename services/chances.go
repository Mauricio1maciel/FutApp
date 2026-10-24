package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"cmp"
	"context"
	"errors"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"
)

// Chances de título, de cada zona da tabela (vagas e rebaixamento) e de cada posição final
// numa liga de pontos corridos. Os jogos que faltam são simulados milhares de vezes. O
// placar de cada um sai de uma distribuição de Poisson com a força de ataque e de defesa
// dos dois times na temporada e o mando de campo da própria liga (services/strength.go).
//
// Desempate e vagas seguem a classificação: os critérios da liga e o AddZones, com os
// campeões de copa já definidos. O confronto direto fica de fora: empate em todos os
// outros critérios é sorteado.

// ErrNoLeagueTable: a competição não tem tabela de pontos corridos (copa, Liga das Nações)
var ErrNoLeagueTable = errors.New("competição sem tabela de pontos corridos")

type TeamChances struct {
	TeamID         int64              `json:"team_id"`
	TeamName       string             `json:"team_name"`
	CrestURL       string             `json:"crest_url"`
	Position       int                `json:"position"` // a de agora, igual à do /standings
	Points         int                `json:"points"`
	Played         int                `json:"played"`
	ExpectedPoints float64            `json:"expected_points"` // média dos pontos no fim
	Title          float64            `json:"title"`           // % de terminar em 1º
	Zones          map[string]float64 `json:"zones"`           // % de terminar em cada zona
	Positions      []float64          `json:"positions"`       // % de terminar em 1º, 2º, 3º...
}

type LeagueChances struct {
	League           string        `json:"league"`
	Season           string        `json:"season"`
	Simulations      int           `json:"simulations"`
	RemainingMatches int           `json:"remaining_matches"`
	Zones            []string      `json:"zones"` // as que aparecem, de cima para baixo da tabela
	Teams            []TeamChances `json:"teams"` // na ordem da classificação de agora
}

// LeagueChancesFor calcula as chances da temporada. Não grava nada.
func LeagueChancesFor(ctx context.Context, league, season string) (*LeagueChances, error) {
	if _, isCup := cupFormats[league]; isCup || league == "UNL" {
		return nil, ErrNoLeagueTable
	}
	matches, err := database.GetMatchesByLeague(ctx, league, season, database.MatchFilter{})
	if err != nil {
		return nil, err
	}
	rules, err := cachedTableRules(ctx, league, season)
	if err != nil {
		return nil, err
	}
	current, table, ok := buildTable(rules, league, season, matches, matches)
	if !ok || len(current) == 0 {
		return nil, ErrNoLeagueTable
	}
	crests := teamCrests(matches)
	for i := range current {
		if current[i].CrestURL == "" {
			current[i].CrestURL = crests[current[i].TeamID]
		}
	}
	return simulateChances(league, season, current, table, rules, time.Now()), nil
}

// simMatch é um jogo que falta: os times (índices na tabela), o placar que já tem (jogo em
// andamento) e e^(-média de gols) de cada lado, para sortear a Poisson
type simMatch struct {
	home, away         int
	baseHome, baseAway int
	expHome, expAway   float64
}

// simulateChances simula o que falta da tabela. current é a classificação de agora (dá a
// ordem e os times); table, os jogos da tabela, encerrados e por jogar.
func simulateChances(league, season string, current []models.Standing, table []models.Match, rules tableRules, now time.Time) *LeagueChances {
	n := len(current)
	index := make(map[int64]int, n)
	for i, s := range current {
		index[s.TeamID] = i
	}
	// Em ordem de ID: jogos no mesmo horário podem vir do banco em qualquer ordem, e o
	// sorteio precisa ser o mesmo para a mesma tabela
	table = slices.Clone(table)
	slices.SortFunc(table, func(a, b models.Match) int { return cmp.Compare(a.MatchID, b.MatchID) })

	// Ponto de partida: os jogos encerrados. Os em andamento são simulados a partir do
	// placar de agora, só no tempo que falta
	pts, wins, gf, ga := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	type pending struct {
		m    models.Match
		h, a int
		left float64 // fração do jogo que falta
	}
	var todo []pending
	for _, m := range table {
		h, okH := index[m.APIHomeTeamID]
		a, okA := index[m.APIAwayTeamID]
		if !okH || !okA {
			continue
		}
		switch {
		case m.Status == "FINISHED":
			addResult(pts, wins, gf, ga, h, a, m.HomeScore, m.AwayScore)
		case liveStatuses[m.Status]:
			todo = append(todo, pending{m, h, a, leftToPlay(m, now)})
		case m.Status == "CANCELLED" || m.Status == "CANCELED" || m.Status == "AWARDED":
			// não vai ser jogado (e não conta na classificação)
		default: // marcado, adiado ou suspenso: ainda vai ser jogado
			todo = append(todo, pending{m, h, a, 1})
		}
	}

	strength := estimateStrength(table)
	remaining := make([]simMatch, len(todo))
	for i, p := range todo {
		r := simMatch{home: p.h, away: p.a}
		if liveStatuses[p.m.Status] {
			r.baseHome, r.baseAway = p.m.HomeScore, p.m.AwayScore
		}
		lh, la := strength.expected(current[p.h].TeamID, current[p.a].TeamID)
		r.expHome = math.Exp(-lh * p.left)
		r.expAway = math.Exp(-la * p.left)
		remaining[i] = r
	}

	positions := make([][]int, n) // positions[time][posição final]: quantas vezes
	for i := range positions {
		positions[i] = make([]int, n)
	}
	zoneHits := map[string][]int{} // zona → quantas vezes cada time terminou nela
	hitZone := func(zone string, team int) {
		if zone == "" || zone == "neutro" {
			return
		}
		if zoneHits[zone] == nil {
			zoneHits[zone] = make([]int, n)
		}
		zoneHits[zone][team]++
	}
	pointsSum := make([]int, n)

	sims := 1
	if len(remaining) == 0 {
		// Acabou: a classificação de agora é a final (com o confronto direto, que a
		// simulação não usa)
		for i, s := range current {
			positions[i][s.Position-1]++
			pointsSum[i] = s.Points
			hitZone(s.Zone, i)
		}
	} else {
		sims = simulationsFor(len(remaining))
		rng := rand.New(rand.NewPCG(tableSeed(league, season, table), 0x5eed))
		crit := statCriteria(withDefaultCriteria(rules.criteria))
		sp, sw, sgf, sga := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
		keys := make([]uint64, n)
		order := make([]int, n)
		final := make([]models.Standing, n)
		byKey := func(x, y int) int { return cmp.Compare(keys[y], keys[x]) }

		for range sims {
			copy(sp, pts)
			copy(sw, wins)
			copy(sgf, gf)
			copy(sga, ga)
			for _, r := range remaining {
				addResult(sp, sw, sgf, sga, r.home, r.away,
					r.baseHome+poisson(rng, r.expHome), r.baseAway+poisson(rng, r.expAway))
			}
			for i := range order {
				order[i] = i
				keys[i] = rankKey(crit, sp[i], sw[i], sgf[i]-sga[i], sgf[i])<<16 | uint64(rng.IntN(1<<16))
			}
			slices.SortFunc(order, byKey)
			for p, i := range order {
				positions[i][p]++
				pointsSum[i] += sp[i]
				final[p] = models.Standing{TeamID: current[i].TeamID, TeamName: current[i].TeamName, Position: p + 1}
			}
			AddZones(final, rules.winners, rules.rule, rules.zones)
			for p, i := range order {
				hitZone(final[p].Zone, i)
			}
		}
	}

	out := &LeagueChances{League: league, Season: season, Simulations: sims,
		RemainingMatches: len(remaining), Zones: []string{}, Teams: make([]TeamChances, 0, n)}
	for _, key := range []string{"lib", "pre", "sul", "reb"} { // a ordem do AddZones, de cima para baixo
		for _, z := range rules.zones {
			if z.ZoneKey == key && zoneHits[z.ZoneName] != nil && !slices.Contains(out.Zones, z.ZoneName) {
				out.Zones = append(out.Zones, z.ZoneName)
			}
		}
	}
	pct := func(count int) float64 { return math.Round(float64(count)*1000/float64(sims)) / 10 }
	for i, s := range current {
		tc := TeamChances{
			TeamID: s.TeamID, TeamName: s.TeamName, CrestURL: s.CrestURL,
			Position: s.Position, Points: s.Points, Played: s.Played,
			ExpectedPoints: math.Round(float64(pointsSum[i])*10/float64(sims)) / 10,
			Title:          pct(positions[i][0]),
			Zones:          make(map[string]float64, len(out.Zones)),
			Positions:      make([]float64, n),
		}
		for p, c := range positions[i] {
			tc.Positions[p] = pct(c)
		}
		for _, z := range out.Zones {
			tc.Zones[z] = pct(zoneHits[z][i])
		}
		out.Teams = append(out.Teams, tc)
	}
	return out
}

func addResult(pts, wins, gf, ga []int, h, a, hg, ag int) {
	gf[h] += hg
	ga[h] += ag
	gf[a] += ag
	ga[a] += hg
	switch {
	case hg > ag:
		pts[h] += 3
		wins[h]++
	case hg < ag:
		pts[a] += 3
		wins[a]++
	default:
		pts[h]++
		pts[a]++
	}
}

// poisson sorteia um número de gols (método de Knuth); expNeg = e^(-média)
func poisson(rng *rand.Rand, expNeg float64) int {
	k, p := 0, rng.Float64()
	for p > expNeg {
		k++
		p *= rng.Float64()
	}
	return k
}

// statCriteria: os critérios de desempate que a simulação aplica, na ordem e sem repetir
// (o confronto direto fica de fora)
func statCriteria(criteria []string) []string {
	var out []string
	for _, c := range criteria {
		switch c {
		case "points", "wins", "goal_diff", "goals_for":
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// rankKey junta os critérios num número só, do mais importante para o menos: 10 bits cada
// (o saldo entra deslocado, para ficar positivo), até 40 bits, e o sorteio vai embaixo
func rankKey(criteria []string, pts, wins, goalDiff, goalsFor int) uint64 {
	var key uint64
	for _, c := range criteria {
		v := 0
		switch c {
		case "points":
			v = pts
		case "wins":
			v = wins
		case "goal_diff":
			v = goalDiff + 512
		case "goals_for":
			v = goalsFor
		}
		key = key<<10 | uint64(min(max(v, 0), 1023))
	}
	return key
}

// simulationsFor: 10 mil simulações (as chances saem com ±1 ponto percentual), menos no
// começo da temporada, quando cada uma tem centenas de jogos
func simulationsFor(remaining int) int {
	return min(10000, max(2000, 1_000_000/max(remaining, 1)))
}

// leftToPlay: quanto falta do jogo em andamento (1 = o jogo todo), pelo horário de início,
// contando 15 minutos de intervalo
func leftToPlay(m models.Match, now time.Time) float64 {
	if m.Status == "PAUSED" {
		return 0.5
	}
	start, ok := parseMatchDate(m.DateEvent)
	if !ok {
		return 0.5
	}
	minutes := now.Sub(start).Minutes()
	if minutes > 60 {
		minutes -= 15
	}
	return min(max(1-minutes/90, 0), 1)
}

// tableSeed: a mesma tabela dá sempre o mesmo sorteio, então as chances só mudam quando um
// resultado muda, e não a cada vez que o cache expira
func tableSeed(league, season string, table []models.Match) uint64 {
	h := fnv.New64a()
	h.Write([]byte(league + "|" + season))
	for _, m := range table {
		h.Write([]byte(strconv.FormatInt(m.MatchID, 10) + m.Status +
			strconv.Itoa(m.HomeScore) + "-" + strconv.Itoa(m.AwayScore) + ";"))
	}
	return h.Sum64()
}
