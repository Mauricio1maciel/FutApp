package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"time"
)

func StartBackgroundUpdater() {
	go func() {
		leagues := []string{"BSA", "PL", "PD"}
		index := 0

		for {
			currentLeague := leagues[index]

			utils.CustomLog("WORKER", "=================================================")
			utils.CustomLog("WORKER", "Iniciando ciclo para a liga [%s]", currentLeague)

			utils.CustomLog("WORKER", ">> [1/2] Buscando Ao Vivo na ESPN...")
			updateESPN(currentLeague)

			utils.CustomLog("WORKER", ">> [2/2] Atualizando Classificação na Football-Data...")
			updateFootballData(currentLeague)

			utils.CustomLog("WORKER", "Ciclo de [%s] finalizado.", currentLeague)
			utils.CustomLog("WORKER", "=================================================\n")

			index = (index + 1) % len(leagues)

			utils.CustomLog("WORKER", " Dormindo por 2 minutos...")
			time.Sleep(2 * time.Minute)
			utils.CustomLog("WORKER", ">> [3/3] Sincronizando jogos passados sem detalhes para %s", currentLeague)

			missing, err := database.GetMissingMatches(currentLeague)
			if err == nil {
				for _, m := range missing {

					UpdateMatchFromESPN(m["home"], m["away"], m["date"], currentLeague)
					time.Sleep(5 * time.Second)
				}
			}

			index = (index + 1) % len(leagues)
			time.Sleep(2 * time.Minute)

		}
	}()
}

func updateESPN(leagueCode string) {
	matches, err := GetLiveScoreboard(leagueCode, "")
	if err != nil {
		utils.CustomLog("WORKER", " Erro na ESPN: %v", err)
		return
	}

	jogosAoVivo := 0
	for _, m := range matches {
		if m.State == "in" {
			jogosAoVivo++
			utils.CustomLog("WORKER", " Ao Vivo ESPN: %s x %s", m.HomeTeam, m.AwayTeam)

			matchDB, lineups, events, err := FetchAndParseESPNMatch(m.MatchID, leagueCode)
			if err == nil {
				database.SaveFullMatchHistory(matchDB, lineups, events)
			}
		}
	}

	if jogosAoVivo == 0 {
		utils.CustomLog("WORKER", "ESPN: Nenhum jogo rolando agora.")
	} else {
		utils.CustomLog("WORKER", " ESPN: %d jogo(s) atualizado(s) no banco.", jogosAoVivo)
	}
}

func updateFootballData(leagueCode string) {

	err := UpdateStandingsInDBOrCache(leagueCode)

	if err != nil {
		utils.CustomLog("WORKER", " Erro na Football-Data: %v", err)
	} else {
		utils.CustomLog("WORKER", " Football-Data: Tabela de Classificação atualizada.")
	}
}

func UpdateStandingsInDBOrCache(leagueCode string) error {
	return nil
}
