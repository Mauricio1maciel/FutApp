package models

type FDResponse struct {
	Matches []FDMatch `json:"matches"`
}

type FDMatch struct {
	ID       int    `json:"id"`
	UTCDate  string `json:"utcDate"`
	Status   string `json:"status"`
	Matchday int    `json:"matchday"`
	Stage    string `json:"stage"`
	Group    string `json:"group"`

	HomeTeam struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"homeTeam"`

	AwayTeam struct {
		ID   int    `json:"id"`
		Name string `json:"shortName"`
	} `json:"awayTeam"`

	Score struct {
		Winner   string `json:"winner"`
		Duration string `json:"duration"` // ADICIONADO: Diz se foi para pênaltis (PENALTY_SHOOTOUT)

		FullTime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"fullTime"`

		RegularTime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"regularTime"`

		ExtraTime struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"extraTime"`

		Penalties struct {
			Home *int `json:"home"`
			Away *int `json:"away"`
		} `json:"penalties"`
	} `json:"score"`
}
