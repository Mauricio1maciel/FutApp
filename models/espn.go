package models

type ESPNScoreboard struct {
	Leagues []struct {
		Name  string `json:"name"`
		Logos []struct {
			Href string `json:"href"`
		} `json:"logos"`
		Season struct {
			Year        int    `json:"year"`
			DisplayName string `json:"displayName"` // ex: "2026-27 UEFA Nations League"
		} `json:"season"`
	} `json:"leagues"`
	Events []ESPNEvent `json:"events"`
}

type ESPNEvent struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	Season struct {
		Slug string `json:"slug"` // fase, ex: "league-phase"
	} `json:"season"`
	Competitions []ESPNCompetition `json:"competitions"`
}

type ESPNCompetition struct {
	Status      ESPNStatus       `json:"status"`
	Competitors []ESPNCompetitor `json:"competitors"`

	Type struct {
		Name         string `json:"name"`
		Abbreviation string `json:"abbreviation"`
	} `json:"type"`

	Group struct {
		Name string `json:"name"`
	} `json:"group"`
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
	// Os dois são o ID da ESPN: esta rota vem direto da ESPN. id_event tem o mesmo
	// nome e sentido das outras listas de jogos; match_id fica por compatibilidade
	// (em /matches, match_id é o ID do nosso banco)
	MatchID    string `json:"match_id"`
	IDEvent    string `json:"id_event"`
	LeagueName string `json:"league_name"`
	LeagueLogo string `json:"league_logo"`
	MatchDate  string `json:"match_date"`
	State      string `json:"state"`
	Clock      string `json:"clock"`

	Stage     string `json:"stage"`
	GroupName string `json:"group_name"`

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
