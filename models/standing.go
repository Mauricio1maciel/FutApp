package models

type Standing struct {
	Position     int
	TeamID       int64
	TeamName     string
	Played       int
	Wins         int
	Draws        int
	Losses       int
	GoalsFor     int
	GoalsAgainst int
	GoalDiff     int
	Points       int
	CrestURL     string
	Zone         string
	Season       string
}
