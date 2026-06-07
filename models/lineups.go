package models

type MatchLineup struct {
	MatchID      string `json:"match_id"`
	ESPNTeamID   int    `json:"espn_team_id"`
	ESPNPlayerID int    `json:"espn_player_id"`
	PlayerName   string `json:"player_name"`
	Jersey       string `json:"jersey"`
	Position     string `json:"position"`
	IsStarter    bool   `json:"is_starter"`
	Formation    string `json:"formation,omitempty"`
}

type AppPlayer struct {
	Name     string `json:"name"`
	Jersey   string `json:"jersey"`
	Position string `json:"position"`
}

type AppLineup struct {
	TeamName  string      `json:"team_name"`
	Logo      string      `json:"logo"`
	Formation string      `json:"formation"`
	Starters  []AppPlayer `json:"starters"`
	Bench     []AppPlayer `json:"bench"`
}

type AppEvent struct {
	Minute     string `json:"minute"`
	Type       string `json:"type"`
	PlayerName string `json:"player_name"`
	TeamName   string `json:"team_name"`
	TeamLogo   string `json:"team_logo"`
}

type AppMatchSummary struct {
	Lineups  []AppLineup `json:"lineups"`
	Timeline []AppEvent  `json:"timeline"`
}

type ESPNSummaryResponse struct {
	Header struct {
		Competitions []struct {
			Date   string `json:"date"`
			Status struct {
				Type struct {
					State string `json:"state"`
				} `json:"type"`
			} `json:"status"`
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Score    string `json:"score"`
				Team     struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
					Logos       []struct {
						Href string `json:"href"`
					} `json:"logos"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"header"`

	Rosters []struct {
		Team struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"team"`
		Formation string `json:"formation"`
		Roster    []struct {
			Athlete struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"athlete"`
			Jersey   string `json:"jersey"`
			Starter  bool   `json:"starter"`
			Position struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
		} `json:"roster"`
	} `json:"rosters"`

	KeyEvents []struct {
		Clock struct {
			DisplayValue string `json:"displayValue"`
		} `json:"clock"`
		Team struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"team"`
		Type struct {
			Text string `json:"text"`
		} `json:"type"`
		Participants []struct {
			Athlete struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"athlete"`
		} `json:"participants"`
		Text string `json:"text"`
	} `json:"keyEvents"`
}

type ESPNKeyEvent struct {
	Clock struct {
		DisplayValue string `json:"displayValue"`
	} `json:"clock"`
	Type struct {
		Text string `json:"text"`
	} `json:"type"`
	Team struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Logo        string `json:"logo"`
	} `json:"team"`
	Participants []struct {
		Athlete struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"athlete"`
	} `json:"participants"`
}

type ESPNRoster struct {
	Team struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		Logo        string `json:"logo"`
	} `json:"team"`
	Formation string       `json:"formation"`
	Roster    []ESPNPlayer `json:"roster"`
}

type ESPNPlayer struct {
	Starter bool   `json:"starter"`
	Jersey  string `json:"jersey"`
	Athlete struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"athlete"`
	Position struct {
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"displayName"`
	} `json:"position"`
}

type ESPNHeader struct {
	Competitions []struct {
		Date   string `json:"date"`
		Status struct {
			Type struct {
				State string `json:"state"`
			} `json:"type"`
		} `json:"status"`
		Competitors []struct {
			HomeAway string `json:"homeAway"`
			Score    string `json:"score"`
			Team     struct {
				DisplayName string `json:"displayName"`
			} `json:"team"`
		} `json:"competitors"`
	} `json:"competitions"`
}
