package models

type CompetitionRule struct {
	league          string
	Season          string
	Libertadores    int
	PreLibertadores int
	SulAmericana    int
	Rebaixamento    int
}

type Winner struct {
	league      string
	season      string
	Competition string
	TeamName    string
}

type CompetitionTiebreaker struct {
	league    string
	season    string
	priority  int
	criterion int
}
