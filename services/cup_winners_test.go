package services

import (
	"App-Futebol/models"
	"testing"
)

func TestFinalWinner(t *testing.T) {
	pen := func(n int) *int { return &n }
	jogo := func(date string, home, away int64, hs, as int) models.Match {
		return models.Match{DateEvent: date, Status: "FINISHED", APIHomeTeamID: home, APIAwayTeamID: away, HomeScore: hs, AwayScore: as}
	}
	casos := []struct {
		nome          string
		finais        []models.Match
		campeao, vice int64
		decidido      bool
	}{
		{"jogo único", []models.Match{jogo("2026-11-28 20:00:00", 1, 2, 1, 0)}, 1, 2, true},
		{"jogo único nos pênaltis", func() []models.Match {
			m := jogo("2026-11-28 20:00:00", 1, 2, 1, 1)
			m.HomePenalty, m.AwayPenalty = pen(3), pen(4)
			return []models.Match{m}
		}(), 2, 1, true},
		{"ida e volta pela soma", []models.Match{
			jogo("2026-12-09 21:30:00", 2, 1, 2, 1), // volta (fora de ordem de propósito)
			jogo("2026-12-02 21:30:00", 1, 2, 2, 0), // ida
		}, 1, 2, true},
		{"ida e volta empatada, pênaltis na volta", func() []models.Match {
			volta := jogo("2026-12-09 21:30:00", 2, 1, 1, 0)
			volta.HomePenalty, volta.AwayPenalty = pen(5), pen(6)
			return []models.Match{jogo("2026-12-02 21:30:00", 1, 2, 1, 0), volta}
		}(), 1, 2, true},
		{"final ainda não jogada", []models.Match{{Status: "TIMED", APIHomeTeamID: 1, APIAwayTeamID: 2}}, 0, 0, false},
		{"finalistas indefinidos", []models.Match{{Status: "TIMED"}}, 0, 0, false},
		{"empate sem pênaltis anotados", []models.Match{jogo("2026-11-28 20:00:00", 1, 2, 0, 0)}, 0, 0, false},
		{"sem final", nil, 0, 0, false},
	}
	for _, c := range casos {
		campeao, vice, ok := finalWinner(c.finais)
		if ok != c.decidido || campeao != c.campeao || vice != c.vice {
			t.Errorf("%s: esperado %d/%d decidido=%v, veio %d/%d decidido=%v", c.nome, c.campeao, c.vice, c.decidido, campeao, vice, ok)
		}
	}
}
