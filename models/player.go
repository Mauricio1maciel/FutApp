package models

type Player struct {
	ID           int    `json:"id"`
	ApiID        int    `json:"api_id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name"`
	Position     string `json:"position"`
	JerseyNumber int    `json:"jersey_number"`
	DateOfBirth  string `json:"date_of_birth"`
	Nationality  string `json:"nationality"`
	TeamID       int    `json:"team_id"`
	TeamName     string `json:"team_name"`
	HeadshotURL  string `json:"headshot_url"`
	Source       string `json:"source"`
	League       string `json:"league"`
}
