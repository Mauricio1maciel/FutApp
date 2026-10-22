package database

import (
	"context"
	"testing"
	"time"
)

// "Meus jogos": os jogos de quem a pessoa segue, na janela de dias e os em andamento
func TestGetFollowedMatches(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	hoje := newTestMatch(t, 7772001, 2*time.Hour, "TIMED", 0, 0)
	daquiATresDias := newTestMatch(t, 7772002, 72*time.Hour, "TIMED", 0, 0)
	anteontem := newTestMatch(t, 7772003, -48*time.Hour, "FINISHED", 2, 1)
	aoVivo := newTestMatch(t, 7772004, -time.Hour, "IN_PLAY", 1, 0)
	vivoEsquecido := newTestMatch(t, 7772005, -30*time.Hour, "IN_PLAY", 0, 0) // status antigo, nunca encerrado

	ids := func(teams []int64, leagues []string, matches []int64, from, to time.Time) []int64 {
		t.Helper()
		ms, err := GetFollowedMatches(ctx, teams, leagues, matches, from, to)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, m := range ms {
			out = append(out, m.MatchID)
			if m.LeagueName != "Liga de Teste" {
				t.Errorf("jogo %d sem o nome da liga: %q", m.MatchID, m.LeagueName)
			}
		}
		return out
	}
	igual := func(nome string, got []int64, want ...int64) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: veio %v, esperado %v", nome, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: veio %v, esperado %v", nome, got, want)
				return
			}
		}
	}
	agora := time.Now()

	// Do dia anterior até daqui a 2 dias: o de hoje e o ao vivo (o de anteontem e o de
	// daqui a 3 dias ficam de fora; o "ao vivo" de 30 h atrás é status esquecido)
	janela := func() (time.Time, time.Time) { return agora.Add(-26 * time.Hour), agora.Add(48 * time.Hour) }
	from, to := janela()
	igual("seguindo o time", ids([]int64{tstHome}, nil, nil, from, to), aoVivo, hoje)
	// Pelo time e pela liga ao mesmo tempo: cada jogo uma vez só
	igual("time e liga", ids([]int64{tstAway}, []string{tstLeague}, nil, from, to), aoVivo, hoje)
	// Só um jogo seguido
	igual("um jogo", ids(nil, nil, []int64{hoje}, from, to), hoje)
	// Time que não joga nada: nada
	igual("outro time", ids([]int64{123456789}, nil, nil, from, to))

	// Janela inteira no futuro: o jogo em andamento continua aparecendo
	igual("ao vivo fora da janela", ids([]int64{tstHome}, nil, nil, agora.Add(24*time.Hour), agora.Add(96*time.Hour)),
		aoVivo, daquiATresDias)

	// Janela larga: todos, pela data, inclusive o de status esquecido
	igual("janela larga", ids(nil, []string{tstLeague}, nil, agora.Add(-72*time.Hour), agora.Add(96*time.Hour)),
		anteontem, vivoEsquecido, aoVivo, hoje, daquiATresDias)
}
