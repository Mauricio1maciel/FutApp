package models

type FullMatchHistory struct {
	Match   ESPNMatchDB    `json:"match"`
	Lineups []ESPNLineupDB `json:"lineups"`
	Events  []ESPNEventDB  `json:"events"`
}

type ESPNMatchDB struct {
	MatchID   string `json:"espn_match_id"`
	League    string `json:"league"`
	MatchDate string `json:"match_date"`
	HomeLogo  string `json:"home_logo"`
	AwayLogo  string `json:"away_logo"`
	HomeScore string `json:"home_score"`
	AwayScore string `json:"away_score"`
	Status    string `json:"status"`

	ESPNHomeTeamID int64  `json:"espn_home_team_id"`
	ESPNAwayTeamID int64  `json:"espn_away_team_id"`
	APIHomeTeamID  int64  `json:"api_home_team_id"`
	APIAwayTeamID  int64  `json:"api_away_team_id"`
	HomeTeam       string `json:"home_team"`
	AwayTeam       string `json:"away_team"`
}

type ESPNLineupDB struct {
	MatchID      string `json:"match_id"`
	ESPNTeamID   int64  `json:"espn_team_id"`
	ESPNPlayerID int64  `json:"espn_player_id"`
	PlayerName   string `json:"player_name"`
	Jersey       string `json:"jersey"`
	Position     string `json:"position"`
	IsStarter    bool   `json:"is_starter"`
	Formation    string `json:"formation"`
	HeadshotURL  string `json:"headshot_url"`
}

type ESPNEventDB struct {
	MatchID    string `json:"match_id"`
	Minute     string `json:"minute"`
	EventType  string `json:"event_type"`
	ESPNTeamID int64  `json:"espn_team_id"`
	PlayerName string `json:"player_name"`
	Details    string `json:"details"`
}
