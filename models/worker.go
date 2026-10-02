// Arquivo: models/worker.go

package models

type WorkerMatch struct {
	IDEvent       string
	League        string
	MatchTimeUnix int64
	Status        string
}

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
