// package servico

// import (
// 	"App-Futebol/database" // Adicionamos a importação do seu pacote de banco de dados
// 	"App-Futebol/utils"
// 	"log"
// 	"time"
// )

// // StartBackgroundScheduler inicia as tarefas repetitivas
// func StartBackgroundScheduler() {
// 	go func() {
// 		for {
// 			// 1. Calcular quanto tempo falta para as 03:00 da manhã seguinte
// 			agora := time.Now()
// 			proximaExecucao := time.Date(agora.Year(), agora.Month(), agora.Day(), 3, 0, 0, 0, agora.Location())

// 			// Se já passaram das 03:00 hoje, agenda para amanhã
// 			if agora.After(proximaExecucao) {
// 				proximaExecucao = proximaExecucao.Add(24 * time.Hour)
// 			}

// 			tempoDeEspera := time.Until(proximaExecucao)
// 			utils.CustomLog("SCHEDULER", "Próxima sincronização agendada para: %v", proximaExecucao)

// 			// 2. Dormir até à hora marcada
// 			time.Sleep(tempoDeEspera)

// 			// 3. Executar a sincronização
// 			utils.CustomLog("SCHEDULER", "Iniciando sincronização diária de equipas...")

//				// MUDANÇA AQUI: Chamando a função a partir do pacote database
//				err := database.SyncCrossAPITeams()
//				if err != nil {
//					log.Printf("[ERRO] Falha na sincronização diária: %v", err)
//				}
//			}
//		}()
//	}
package servico

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"log"
	"time"
)

func StartBackgroundScheduler() {
	go func() {
		for {
			agora := time.Now()
			proximaExecucao := time.Date(agora.Year(), agora.Month(), agora.Day(), 3, 0, 0, 0, agora.Location())

			if agora.After(proximaExecucao) {
				proximaExecucao = proximaExecucao.Add(24 * time.Hour)
			}

			tempoDeEspera := time.Until(proximaExecucao)
			utils.CustomLog("SCHEDULER", "Próxima sincronização agendada para: %v", proximaExecucao)

			time.Sleep(tempoDeEspera)

			utils.CustomLog("SCHEDULER", "Iniciando sincronização diária das 03:00...")

			mapaTimesESPN := make(map[string]int64)

			err := database.SyncCrossAPITeams(mapaTimesESPN)
			if err != nil {
				log.Printf("[ERRO] Falha na sincronização cruzada de times: %v", err)
			} else {
				utils.CustomLog("SCHEDULER", "Rotina diária finalizada com sucesso!")
			}
		}
	}()
}
