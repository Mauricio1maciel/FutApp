package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"testing"
)

func TestMinuteOf(t *testing.T) {
	for in, want := range map[string]int{"67'": 67, "45'+2'": 47, "90'+3'": 93, "": 0} {
		if got := minuteOf(in); got != want {
			t.Errorf("%q: esperado %d, veio %d", in, want, got)
		}
	}
}

func TestPlayerMatchLine(t *testing.T) {
	ev := func(tipo, minuto, jogador, detalhe string) database.MatchEvent {
		return database.MatchEvent{Type: tipo, Minute: minuto, Player: jogador, Details: detalhe}
	}
	jogo := database.PlayerAppearance{ESPNMatchID: 1, PlayerName: "Pedro", ESPNTeamID: 10,
		HomeESPNID: 10, AwayESPNID: 20, HomeScore: 2, AwayScore: 1}

	// Titular: gol de pênalti, gol de cabeça, amarelo, substituído aos 78'
	titular := jogo
	titular.Starter = true
	l, jogou := playerMatchLine(titular, []database.MatchEvent{
		ev("Penalty - Scored", "12'", "Pedro", ""),
		ev("Goal - Header", "45'+1'", "Pedro", ""),
		ev("Goal", "50'", "Outro", "Pedro"), // assistência do Pedro
		ev("Yellow Card", "60'", "Pedro", ""),
		ev("Substitution", "78'", "Bruno", "Pedro"),
	})
	if !jogou || l.Minutes != 78 || l.Goals != 2 || l.Penalties != 1 || l.Assists != 1 || l.YellowCards != 1 ||
		l.SubbedOut != "78'" || l.Result != "V" || !l.Home || l.GoalsFor != 2 {
		t.Errorf("titular: %+v", l)
	}

	// Reserva que entrou aos 60' e foi expulso aos 85', jogando fora de casa
	reserva := jogo
	reserva.ESPNTeamID = 20
	l, jogou = playerMatchLine(reserva, []database.MatchEvent{
		ev("Substitution", "60'", "Pedro", "Fulano"),
		ev("Red Card", "85'", "Pedro", ""),
	})
	if !jogou || l.Minutes != 25 || l.RedCards != 1 || l.SubbedIn != "60'" || l.Result != "D" || l.Home {
		t.Errorf("reserva expulso: %+v", l)
	}

	// Reserva que não entrou: não conta como jogo
	if _, jogou := playerMatchLine(reserva, nil); jogou {
		t.Error("reserva que ficou no banco não pode contar como jogo")
	}

	// Titular em jogo com prorrogação: 120 minutos; gol contra não conta como gol
	l, _ = playerMatchLine(titular, []database.MatchEvent{
		ev("Start Extra Time", "90'", "", ""),
		ev("Own Goal", "100'", "Pedro", ""),
	})
	if l.Minutes != 120 || l.Goals != 0 || l.OwnGoals != 1 {
		t.Errorf("prorrogação: %+v", l)
	}
}

func TestAddTeamMatch(t *testing.T) {
	c := &TeamCompetitionStats{}
	jogos := []models.Match{
		{MatchID: 1, APIHomeTeamID: 7, APIAwayTeamID: 8, HomeScore: 4, AwayScore: 0}, // vitória em casa
		{MatchID: 2, APIHomeTeamID: 9, APIAwayTeamID: 7, HomeScore: 3, AwayScore: 1}, // derrota fora
		{MatchID: 3, APIHomeTeamID: 8, APIAwayTeamID: 7, HomeScore: 0, AwayScore: 0}, // empate fora
		{MatchID: 4, APIHomeTeamID: 7, APIAwayTeamID: 9, HomeScore: 2, AwayScore: 1}, // vitória em casa
	}
	for _, m := range jogos {
		addTeamMatch(c, 7, m)
	}
	want := RecordSplit{Played: 4, Wins: 2, Draws: 1, Losses: 1, GoalsFor: 7, GoalsAgainst: 4}
	if c.RecordSplit != want {
		t.Errorf("campanha: esperado %+v, veio %+v", want, c.RecordSplit)
	}
	if c.Home != (RecordSplit{Played: 2, Wins: 2, GoalsFor: 6, GoalsAgainst: 1}) ||
		c.Away != (RecordSplit{Played: 2, Draws: 1, Losses: 1, GoalsFor: 1, GoalsAgainst: 3}) {
		t.Errorf("casa/fora: %+v / %+v", c.Home, c.Away)
	}
	if c.CleanSheets != 2 || c.FailedToScore != 1 {
		t.Errorf("sem sofrer gol / sem marcar: %d / %d", c.CleanSheets, c.FailedToScore)
	}
	if c.BiggestWin == nil || c.BiggestWin.MatchID != 1 || c.BiggestLoss == nil || c.BiggestLoss.MatchID != 2 {
		t.Errorf("maior vitória / derrota: %+v / %+v", c.BiggestWin, c.BiggestLoss)
	}
}

func TestIsAssistedGoal(t *testing.T) {
	for tipo, want := range map[string]bool{"Goal": true, "Goal - Header": true, "Goal - Volley": true,
		"Penalty - Scored": false, "Own Goal": false, "Substitution": false} {
		if got := isAssistedGoal(tipo); got != want {
			t.Errorf("%q: esperado %v, veio %v", tipo, want, got)
		}
	}
}
