// // Arquivo: models/worker.go

package models

// WorkerMatch contém apenas os dados cruciais para o motor do Orange Pi
type WorkerMatch struct {
	IDEvent       string
	League        string
	MatchTimeUnix int64
	Status        string
}

// 🔥 Estrutura para ler o Scoreboard diário da ESPN
type ESPNScoreboardResponse struct {
	Events []struct {
		ID           string `json:"id"`
		Date         string `json:"date"`
		Competitions []struct {
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Team     struct {
					ID string `json:"id"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"events"`
}
