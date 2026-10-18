package models

type Standing struct {
	Position     int    `json:"position"`
	GroupName    string `json:"group_name"`
	TeamID       int64  `json:"team_id"`
	TeamName     string `json:"team_name"`
	Played       int    `json:"played"`
	Wins         int    `json:"wins"`
	Draws        int    `json:"draws"`
	Losses       int    `json:"losses"`
	GoalsFor     int    `json:"goals_for"`
	GoalsAgainst int    `json:"goals_against"`
	GoalDiff     int    `json:"goal_diff"`
	Points       int    `json:"points"`
	CrestURL     string `json:"crest_url"`
	Zone         string `json:"zone"`
	Season       string `json:"season"`
	// Últimos 5 resultados na competição, do mais antigo para o mais recente: V, E ou D
	Form string `json:"form"`

	// Só na classificação ao vivo (/standings?live=true)
	Live           bool `json:"live,omitempty"`            // o time está jogando agora
	PositionChange int  `json:"position_change,omitempty"` // posições ganhas (+) ou perdidas (-) com os jogos em andamento
}
