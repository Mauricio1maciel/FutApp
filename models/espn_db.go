package models

type FullMatchHistory struct {
	Match      ESPNMatchDB    `json:"match"`
	Lineups    []ESPNLineupDB `json:"lineups"`
	Events     []ESPNEventDB  `json:"events"`
	Statistics []MatchStat    `json:"statistics"` // vazio antes do jogo ou se a ESPN não informar
}

type ESPNMatchDB struct {
	MatchID   string `json:"espn_match_id"`
	League    string `json:"league"`
	Season    string `json:"season"`
	MatchDate string `json:"match_date"`
	HomeLogo  string `json:"home_logo"`
	AwayLogo  string `json:"away_logo"`
	HomeScore string `json:"home_score"`
	AwayScore string `json:"away_score"`
	// Disputa de pênaltis (nil = não houve)
	HomePenalty *int   `json:"home_penalty,omitempty"`
	AwayPenalty *int   `json:"away_penalty,omitempty"`
	Status      string `json:"status"`
	Clock       string `json:"clock,omitempty"`

	Stage     string `json:"stage"`
	GroupName string `json:"group_name"`

	ESPNHomeTeamID int64  `json:"espn_home_team_id"`
	ESPNAwayTeamID int64  `json:"espn_away_team_id"`
	APIHomeTeamID  int64  `json:"api_home_team_id"`
	APIAwayTeamID  int64  `json:"api_away_team_id"`
	HomeTeam       string `json:"home_team"`
	AwayTeam       string `json:"away_team"`

	// Estatísticas cruas da ESPN, gravadas em espn_matches.statistics. A API devolve a
	// lista escolhida (FullMatchHistory.Statistics), não isto
	Statistics *MatchStatistics `json:"-"`

	// Narração lance a lance em português, gravada em espn_matches.commentary (nil = não
	// buscada: fica a que já está gravada)
	Commentary []CommentaryEntry `json:"-"`
}

// CommentaryEntry é um lance da narração da ESPN
type CommentaryEntry struct {
	Seq    int    `json:"seq"`            // ordem na narração (estável entre leituras)
	Minute string `json:"minute"`         // ex: "40'", "90'+13'"; vazio no fim do jogo
	Text   string `json:"text"`           // ex: "Rodrigo Garro (Corinthians) recebe cartão amarelo."
	Type   string `json:"type,omitempty"` // tipo do lance na ESPN, ex: goal, yellow-card, foul, substitution
	Team   string `json:"team,omitempty"` // time do lance, quando há
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
