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
	Clock        float64 `json:"clock"` // segundos de jogo (1200 = 20 minutos)
	DisplayClock string  `json:"displayClock"`
	Period       int     `json:"period"` // 1 e 2: tempos; 3 e 4: prorrogação; 5: pênaltis
	Type         struct {
		Name  string `json:"name"` // ex: STATUS_FIRST_HALF, STATUS_HALFTIME, STATUS_SHOOTOUT
		State string `json:"state"`
	} `json:"type"`
}

type ESPNCompetitor struct {
	HomeAway string `json:"homeAway"`
	Score    string `json:"score"`
	// Pênaltis convertidos na disputa (só existe em jogo decidido nos pênaltis)
	ShootoutScore *float64 `json:"shootoutScore"`
	Team          struct {
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

	// Relógio para o app contar sozinho entre uma atualização e outra: os segundos de
	// jogo na hora em que a ESPN foi lida (clock_at). Parado no intervalo e nos pênaltis
	ClockSeconds int    `json:"clock_seconds"`
	Period       int    `json:"period"`
	ClockRunning bool   `json:"clock_running"`
	ClockAt      string `json:"clock_at,omitempty"`
	// O relógio foi estimado pelo horário de início: a ESPN mandou o jogo rolando com o
	// relógio zerado (acontece quando o placar ao vivo dela trava)
	ClockEstimated bool `json:"clock_estimated,omitempty"`

	// Disputa de pênaltis, parcial durante a disputa (nil = não houve)
	HomePenalty *int `json:"home_penalty,omitempty"`
	AwayPenalty *int `json:"away_penalty,omitempty"`

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
