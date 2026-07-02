package models

// Estrutura que enviamos para o App (Mantém-se igual)
type PlayerStat struct {
	Rank     int    `json:"rank"`
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Team     string `json:"team"`
	TeamLogo string `json:"team_logo"`
	Photo    string `json:"photo"`
	Value    string `json:"value"`
}

type LeagueStatsResponse struct {
	TopScorers []PlayerStat `json:"top_scorers"`
	TopAssists []PlayerStat `json:"top_assists"`
}

// 🔥 Estrutura crua ATUALIZADA para capturar o ID do time da competição
type ESPNStatisticsResponse struct {
	Stats []struct {
		Name    string `json:"name"`
		Leaders []struct {
			Athlete struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"` // Necessário para atualizar o cadastro base
				Headshot    struct {
					Href string `json:"href"` // Necessário para atualizar a foto
				} `json:"headshot"`
				Team struct {
					ID string `json:"id"` // 🔥 AQUI: O ID da França (478) em vez do Real Madrid
				} `json:"team"`
				Statistics []struct {
					Name  string  `json:"name"`
					Value float64 `json:"value"`
				} `json:"statistics"`
			} `json:"athlete"`
		} `json:"leaders"`
	} `json:"stats"`
}
