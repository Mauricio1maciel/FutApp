package models

type ESPNScoreboard struct {
	Leagues []struct {
		Name  string `json:"name"`
		Logos []struct {
			Href string `json:"href"`
		} `json:"logos"`
	} `json:"leagues"`
	Events []ESPNEvent `json:"events"`
}

type ESPNEvent struct {
	ID           string            `json:"id"`
	Date         string            `json:"date"`
	Competitions []ESPNCompetition `json:"competitions"`
}

type ESPNCompetition struct {
	Status      ESPNStatus       `json:"status"`
	Competitors []ESPNCompetitor `json:"competitors"`
}

type ESPNStatus struct {
	DisplayClock string `json:"displayClock"`
	Type         struct {
		State string `json:"state"`
	} `json:"type"`
}

type ESPNCompetitor struct {
	HomeAway string `json:"homeAway"`
	Score    string `json:"score"`
	Team     struct {
		ID           string `json:"id"`
		DisplayName  string `json:"displayName"`
		Abbreviation string `json:"abbreviation"`
		Logo         string `json:"logo"`
	} `json:"team"`
}

type AppLiveMatch struct {
	MatchID        string `json:"match_id"`
	LeagueName     string `json:"league_name"`
	LeagueLogo     string `json:"league_logo"`
	MatchDate      string `json:"match_date"`
	State          string `json:"state"`
	Clock          string `json:"clock"`
	ESPNHomeTeamID string `json:"espn_home_team_id"`
	HomeTeam       string `json:"home_team"`
	HomeLogo       string `json:"home_logo"`
	HomeScore      string `json:"home_score"`
	ESPNAwayTeamID string `json:"espn_away_team_id"`
	AwayTeam       string `json:"away_team"`
	AwayLogo       string `json:"away_logo"`
	AwayScore      string `json:"away_score"`
	LastEvent      string `json:"last_event,omitempty"`
}
