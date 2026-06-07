package models

type Player struct {
	ID           int    `json:"id"`
	ApiID        int    `json:"api_id"`
	Name         string `json:"name"`
	ShortName    string `json:"short_name,omitempty"`
	Position     string `json:"position"`
	JerseyNumber int    `json:"jersey_number,omitempty"`
	DateOfBirth  string `json:"dateOfBirth,omitempty"`
	Nationality  string `json:"nationality,omitempty"`
	TeamID       int    `json:"team_id"`
	TeamName     string `json:"team_name,omitempty"`
	HeadshotURL  string `json:"headshot_url,omitempty"`
	Source       string `json:"source"`
	League       string `json:"league"`
}
