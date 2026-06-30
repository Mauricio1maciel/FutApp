package services

import (
	"App-Futebol/models"
	"sort"
)

func CalculateStandings(matches []models.Match) map[int64]*models.Standing {

	table := make(map[int64]*models.Standing)

	for _, m := range matches {

		// 1. 🔥 SEMPRE REGISTRA OS TIMES NA TABELA (mesmo que o jogo seja no futuro)
		home := table[m.APIHomeTeamID]
		if home == nil {
			home = &models.Standing{
				TeamID:   m.APIHomeTeamID,
				TeamName: m.HomeTeam,
			}
			table[m.APIHomeTeamID] = home
		}

		away := table[m.APIAwayTeamID]
		if away == nil {
			away = &models.Standing{
				TeamID:   m.APIAwayTeamID,
				TeamName: m.AwayTeam,
			}
			table[m.APIAwayTeamID] = away
		}

		// 2. 🔥 SÓ SOMA PONTOS E GOLS SE O JOGO JÁ ROLOU (Ou está rolando)
		if m.Status != "FINISHED" && m.Status != "IN_PLAY" && m.Status != "PAUSED" {
			continue
		}

		// --- Daqui para baixo é igual ---
		home.Played++
		away.Played++

		home.GoalsFor += m.HomeScore
		home.GoalsAgainst += m.AwayScore

		away.GoalsFor += m.AwayScore
		away.GoalsAgainst += m.HomeScore

		if m.HomeScore > m.AwayScore {
			home.Wins++
			home.Points += 3
			away.Losses++
		} else if m.HomeScore < m.AwayScore {
			away.Wins++
			away.Points += 3
			home.Losses++
		} else {
			home.Draws++
			away.Draws++
			home.Points++
			away.Points++
		}
	}

	for _, t := range table {
		t.GoalDiff = t.GoalsFor - t.GoalsAgainst
	}

	return table
}

func MapToSlice(table map[int64]*models.Standing) []models.Standing {
	var s []models.Standing
	for _, v := range table {
		s = append(s, *v)
	}
	return s
}

func canApplyHeadToHead(group []models.Standing, matches []models.Match) bool {

	teams := map[int64]bool{}
	for _, t := range group {
		teams[t.TeamID] = true // Usando ID numérico
	}

	expected := len(group) * (len(group) - 1)
	count := 0

	for _, m := range matches {
		if teams[m.APIHomeTeamID] && teams[m.APIAwayTeamID] {

			if m.Status != "FINISHED" {
				return false
			}
			count++
		}
	}

	return count >= expected
}

func applyHeadToHead(group []models.Standing, matches []models.Match) []models.Standing {

	if !canApplyHeadToHead(group, matches) {
		return group
	}

	teams := map[int64]bool{}
	for _, t := range group {
		teams[t.TeamID] = true
	}

	type mini struct {
		TeamID    int64
		TeamName  string
		Points    int
		GoalDiff  int
		GoalsFor  int
		AwayGoals int
	}

	stats := map[int64]*mini{}

	for _, t := range group {
		stats[t.TeamID] = &mini{TeamID: t.TeamID, TeamName: t.TeamName}
	}

	for _, m := range matches {

		if !teams[m.APIHomeTeamID] || !teams[m.APIAwayTeamID] {
			continue
		}

		if m.Status != "FINISHED" {
			continue
		}

		home := stats[m.APIHomeTeamID]
		away := stats[m.APIAwayTeamID]

		home.GoalsFor += m.HomeScore
		home.GoalDiff += m.HomeScore - m.AwayScore

		away.GoalsFor += m.AwayScore
		away.GoalDiff += m.AwayScore - m.HomeScore
		away.AwayGoals += m.AwayScore

		if m.HomeScore > m.AwayScore {
			home.Points += 3
		} else if m.HomeScore < m.AwayScore {
			away.Points += 3
		} else {
			home.Points++
			away.Points++
		}
	}

	var miniSlice []mini
	for _, v := range stats {
		miniSlice = append(miniSlice, *v)
	}

	sort.SliceStable(miniSlice, func(i, j int) bool {

		if miniSlice[i].Points != miniSlice[j].Points {
			return miniSlice[i].Points > miniSlice[j].Points
		}
		if miniSlice[i].GoalDiff != miniSlice[j].GoalDiff {
			return miniSlice[i].GoalDiff > miniSlice[j].GoalDiff
		}
		if miniSlice[i].GoalsFor != miniSlice[j].GoalsFor {
			return miniSlice[i].GoalsFor > miniSlice[j].GoalsFor
		}
		return miniSlice[i].AwayGoals > miniSlice[j].AwayGoals
	})

	rank := map[int64]int{}
	for i, t := range miniSlice {
		rank[t.TeamID] = i
	}

	sort.SliceStable(group, func(i, j int) bool {
		return rank[group[i].TeamID] < rank[group[j].TeamID]
	})

	return group
}

func compareTeams(
	a, b models.Standing,
	matches []models.Match,
	criteria []string,
) bool {

	for _, c := range criteria {

		switch c {

		case "points":
			if a.Points != b.Points {
				return a.Points > b.Points
			}

		case "wins":
			if a.Wins != b.Wins {
				return a.Wins > b.Wins
			}

		case "goal_diff":
			if a.GoalDiff != b.GoalDiff {
				return a.GoalDiff > b.GoalDiff
			}

		case "goals_for":
			if a.GoalsFor != b.GoalsFor {
				return a.GoalsFor > b.GoalsFor
			}

		case "head_to_head",
			"head_to_head_points",
			"head_to_head_goal_diff",
			"head_to_head_away_goals":

			if !canApplyHeadToHead([]models.Standing{a, b}, matches) {
				continue
			}

			pontosA, pontosB := 0, 0
			saldoA, saldoB := 0, 0

			for _, m := range matches {
				if m.Status == "FINISHED" {
					if m.APIHomeTeamID == a.TeamID && m.APIAwayTeamID == b.TeamID {
						saldoA += m.HomeScore - m.AwayScore
						saldoB += m.AwayScore - m.HomeScore
						if m.HomeScore > m.AwayScore {
							pontosA += 3
						} else if m.HomeScore < m.AwayScore {
							pontosB += 3
						} else {
							pontosA++
							pontosB++
						}
					} else if m.APIHomeTeamID == b.TeamID && m.APIAwayTeamID == a.TeamID {
						saldoA += m.AwayScore - m.HomeScore
						saldoB += m.HomeScore - m.AwayScore
						if m.HomeScore < m.AwayScore {
							pontosA += 3
						} else if m.HomeScore > m.AwayScore {
							pontosB += 3
						} else {
							pontosA++
							pontosB++
						}
					}
				}
			}

			if pontosA != pontosB || saldoA != saldoB {
				h2h := applyHeadToHead([]models.Standing{a, b}, matches)
				if len(h2h) == 2 {
					return h2h[0].TeamID == a.TeamID
				}
			}
		}
	}

	return a.TeamName < b.TeamName
}

func AddPositions(s []models.Standing) {
	for i := range s {
		s[i].Position = i + 1
	}
}

func AddZones(
	s []models.Standing,
	winners []models.Winner,
	rule *models.CompetitionRule,
	zones []models.CompetitionZone,
) {

	if rule == nil {
		return
	}

	zoneMap := map[string]string{}
	for _, z := range zones {
		zoneMap[z.ZoneKey] = z.ZoneName
	}

	lib := rule.Libertadores
	pre := rule.PreLibertadores
	sul := rule.SulAmericana
	reb := rule.Rebaixamento

	guaranteedLib := map[string]bool{}
	guaranteedPre := map[string]bool{}

	for _, w := range winners {
		switch w.Competition {
		case "libertadores", "copa_brasil", "sul_americana":
			guaranteedLib[w.TeamName] = true
		case "vice_copa_brasil":
			guaranteedPre[w.TeamName] = true
			pre++
		}
	}

	rebStart := len(s) - reb + 1

	libQuota := lib
	preQuota := pre
	sulQuota := sul

	for i := range s {

		team := s[i].TeamName
		pos := s[i].Position

		if pos >= rebStart {
			if guaranteedLib[team] {
				s[i].Zone = zoneMap["lib"]
			} else if guaranteedPre[team] {
				s[i].Zone = zoneMap["pre"]
			} else {
				s[i].Zone = zoneMap["reb"]
			}
			continue
		}

		if guaranteedLib[team] {
			s[i].Zone = zoneMap["lib"]
		} else if libQuota > 0 {
			s[i].Zone = zoneMap["lib"]
			libQuota--
		} else if guaranteedPre[team] {
			s[i].Zone = zoneMap["pre"]
		} else if preQuota > 0 {
			s[i].Zone = zoneMap["pre"]
			preQuota--
		} else if sulQuota > 0 {
			s[i].Zone = zoneMap["sul"]
			sulQuota--
		} else {
			s[i].Zone = "neutro"
		}
	}
}

func BuildStandings(
	matches []models.Match,
	winners []models.Winner,
	rule *models.CompetitionRule,
	zones []models.CompetitionZone,
	criteria []string,
) []models.Standing {

	table := CalculateStandings(matches)
	s := MapToSlice(table)

	sort.SliceStable(s, func(i, j int) bool {
		return compareTeams(s[i], s[j], matches, criteria)
	})

	AddPositions(s)
	AddZones(s, winners, rule, zones)

	return s
}
