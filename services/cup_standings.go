package services

import (
	"App-Futebol/models"
	"App-Futebol/utils" // Adicione o import do utils
	"sort"
)

func BuildCupStandings(matches []models.Match, criteria []string) []models.Standing {
	utils.CustomLog("COPA", "🔥 Iniciando cálculo da Copa! Total de jogos recebidos: %d", len(matches))

	groupMatches := make(map[string][]models.Match)
	for _, m := range matches {
		if m.Stage == "GROUP_STAGE" && m.GroupName != "" {
			groupMatches[m.GroupName] = append(groupMatches[m.GroupName], m)
		}
	}

	allStandings := make(map[string][]models.Standing)
	var thirdPlacedTeams []models.Standing

	var groupNames []string
	for gName := range groupMatches {
		groupNames = append(groupNames, gName)
	}
	sort.Strings(groupNames)

	utils.CustomLog("COPA", "✅ Grupos identificados para cálculo: %v", groupNames)

	for _, gName := range groupNames {
		gMatches := groupMatches[gName]
		tableMap := CalculateStandings(gMatches)
		s := MapToSlice(tableMap)

		sort.SliceStable(s, func(i, j int) bool {
			return compareTeams(s[i], s[j], gMatches, criteria)
		})

		for i := range s {
			s[i].Position = i + 1
			s[i].GroupName = gName // Gravando o grupo!

			if s[i].Position == 3 {
				thirdPlacedTeams = append(thirdPlacedTeams, s[i])
			}
		}
		allStandings[gName] = s
	}

	utils.CustomLog("COPA", "⚠️ Total de terceiros colocados encontrados: %d", len(thirdPlacedTeams))

	sort.SliceStable(thirdPlacedTeams, func(i, j int) bool {
		if thirdPlacedTeams[i].Points != thirdPlacedTeams[j].Points {
			return thirdPlacedTeams[i].Points > thirdPlacedTeams[j].Points
		}
		if thirdPlacedTeams[i].GoalDiff != thirdPlacedTeams[j].GoalDiff {
			return thirdPlacedTeams[i].GoalDiff > thirdPlacedTeams[j].GoalDiff
		}
		return thirdPlacedTeams[i].GoalsFor > thirdPlacedTeams[j].GoalsFor
	})

	bestThirds := make(map[int64]bool)
	for i, t := range thirdPlacedTeams {
		if i < 8 {
			bestThirds[t.TeamID] = true
		}
	}

	var finalStandings []models.Standing
	for _, gName := range groupNames {
		for i := range allStandings[gName] {
			team := allStandings[gName][i]

			if team.Position == 1 || team.Position == 2 {
				team.Zone = "Classificado - Oitavas"
			} else if team.Position == 3 && bestThirds[team.TeamID] {
				team.Zone = "Classificado - Melhor 3º"
			} else {
				team.Zone = "Eliminado"
			}

			finalStandings = append(finalStandings, team)
		}
	}

	utils.CustomLog("COPA", "🏆 Tabela da Fase de Grupos gerada com sucesso! Total de times: %d", len(finalStandings))
	return finalStandings
}
