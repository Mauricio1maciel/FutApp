package models

type Match struct {
	IDEvent    string `json:"id_event"`
	League     string `json:"league"`
	LeagueName string `json:"league_name"`
	LeagueLogo string `json:"league_logo"`
	Season     string `json:"season"`
	Round      int    `json:"round"`

	APIHomeTeamID  int64  `json:"api_home_team_id"`
	HomeTeam       string `json:"home_team"`
	ESPNHomeTeamID string `json:"espn_home_team_id"`

	APIAwayTeamID  int64  `json:"api_away_team_id"`
	AwayTeam       string `json:"away_team"`
	ESPNAwayTeamID string `json:"espn_away_team_id"`

	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
	DateEvent string `json:"date_event"`
	Status    string `json:"status"`

	Stage     string `json:"stage"`
	GroupName string `json:"group_name"`

	HomeLogo string `json:"home_logo"`
	AwayLogo string `json:"away_logo"`
}
