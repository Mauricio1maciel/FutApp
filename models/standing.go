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
}
