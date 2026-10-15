package services

import (
	"App-Futebol/models"
	"fmt"
	"testing"
	"time"
)

// tabela monta um turno e returno de n times pelo método do círculo, uma rodada por semana
func tabela(n int) (jogos []models.Match, rodadaReal map[int64]int) {
	return tabelaCom(n, 7*24*time.Hour, time.Hour)
}

// tabelaCom permite rodadas mais próximas (meio de semana) e jogos espalhados na rodada
func tabelaCom(n int, entreRodadas, entreJogos time.Duration) (jogos []models.Match, rodadaReal map[int64]int) {
	rodadaReal = map[int64]int{}
	times := make([]int64, n)
	for i := range times {
		times[i] = int64(i + 1)
	}
	id := int64(0)
	inicio := time.Date(2026, 4, 1, 16, 0, 0, 0, time.UTC)
	for turno := 0; turno < 2; turno++ {
		ts := append([]int64{}, times...)
		for r := 0; r < n-1; r++ {
			rodada := turno*(n-1) + r + 1
			for i := 0; i < n/2; i++ {
				casa, fora := ts[i], ts[n-1-i]
				if turno == 1 {
					casa, fora = fora, casa
				}
				id++
				jogos = append(jogos, models.Match{
					MatchID: id, APIHomeTeamID: casa, APIAwayTeamID: fora,
					DateEvent: inicio.Add(time.Duration(rodada-1) * entreRodadas).Add(time.Duration(i) * entreJogos).Format("2006-01-02 15:04:05"),
				})
				rodadaReal[id] = rodada
			}
			// gira todos menos o primeiro
			ts = append([]int64{ts[0], ts[n-1]}, ts[1:n-1]...)
		}
	}
	return jogos, rodadaReal
}

func moveJogo(jogos []models.Match, id int64, dias float64) {
	for i := range jogos {
		if jogos[i].MatchID == id {
			t, _ := time.Parse("2006-01-02 15:04:05", jogos[i].DateEvent)
			jogos[i].DateEvent = t.Add(time.Duration(dias * 24 * float64(time.Hour))).Format("2006-01-02 15:04:05")
		}
	}
}

func confere(t *testing.T, caso string, jogos []models.Match, real map[int64]int) {
	t.Helper()
	got := assignLeagueRounds(jogos)
	erros := 0
	for id, r := range real {
		if got[id] != r {
			erros++
			if erros <= 3 {
				t.Errorf("%s: jogo %d deveria ser da rodada %d, veio %d", caso, id, r, got[id])
			}
		}
	}
	if erros > 3 {
		t.Errorf("%s: %d jogos na rodada errada", caso, erros)
	}
}

func TestAssignLeagueRounds(t *testing.T) {
	jogos, real := tabela(20) // Série B: 20 times, 38 rodadas
	confere(t, "tabela sem mudanças", jogos, real)

	// Adiado: o 3º jogo da rodada 5 vai para depois da rodada 12
	jogos, real = tabela(20)
	adiado := int64(4*10 + 3)
	moveJogo(jogos, adiado, 7*7.5)
	confere(t, "jogo adiado", jogos, real)

	// Antecipado: um jogo da rodada 30 disputado na semana da rodada 3
	jogos, real = tabela(20)
	moveJogo(jogos, int64(29*10+5), -7*26.5)
	confere(t, "jogo antecipado", jogos, real)

	// Dois jogos da mesma rodada adiados (times diferentes) para semanas diferentes
	jogos, real = tabela(20)
	moveJogo(jogos, int64(7*10+1), 7*6.5)
	moveJogo(jogos, int64(7*10+2), 7*9.5)
	confere(t, "dois adiados na mesma rodada", jogos, real)

	// Rodadas de meio de semana: 3,5 dias entre elas, jogos espalhados por 2 dias e meio
	// (o último jogo de uma rodada fica a 1 dia do primeiro da seguinte), e um adiado
	jogos, real = tabelaCom(20, 84*time.Hour, 6*time.Hour)
	moveJogo(jogos, int64(11*10+4), 3.5*5)
	confere(t, "meio de semana com adiado", jogos, real)

	// Jogo extra sem times definidos ("TBD"): rodada 0, sem atrapalhar os outros
	jogos, real = tabela(20)
	jogos = append(jogos, models.Match{MatchID: 9999, DateEvent: "2026-12-20 20:00:00"})
	if r, ok := assignLeagueRounds(jogos)[9999]; !ok || r != 0 {
		t.Errorf("jogo sem times deveria ficar na rodada 0, veio %d (%v)", r, ok)
	}
	confere(t, "com jogo sem times", jogos, real)

	if testing.Verbose() {
		fmt.Println("rodadas conferidas: 380 jogos por caso")
	}
}
