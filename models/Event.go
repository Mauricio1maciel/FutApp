package models

type Event struct {
	IDEvent   string `json:"idEvent"`
	HomeTeam  string `json:"strHomeTeam"`
	AwayTeam  string `json:"strAwayTeam"`
	DateEvent string `json:"dateEvent"`
	Time      string `json:"strTime"`
	Round     string `json:"intRound"`

	HomeScore string `json:"intHomeScore"`
	AwayScore string `json:"intAwayScore"`
	Starus    string `json:"Status"`
}

func FiltrarRodada(events []Event, rodada string) []Event {

	var jogos []Event

	for _, e := range events {

		if e.Round == rodada {
			jogos = append(jogos, e)
		}
	}

	return jogos
}
