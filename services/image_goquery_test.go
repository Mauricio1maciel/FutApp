package services

import "testing"

func TestSameTeam(t *testing.T) {
	casos := []struct {
		tm, ours string
		want     bool
	}{
		{"Flamengo", "CR Flamengo", true}, // antes falhava: o nosso nome tem o "CR"
		{"Palmeiras", "SE Palmeiras", true},
		{"Grêmio", "Grêmio FBPA", true},
		{"Atlético-MG", "CA Mineiro", false}, // nomes diferentes de verdade: não arrisca
		{"Sporting CP", "Sporting Clube de Portugal", true},
		{"Atlético de Madrid", "Club Atlético de Madrid", true},
		{"São Paulo", "São Paulo FC", true},
		{"Santos", "Santos FC", true},
		{"Flamengo", "SE Palmeiras", false},
		{"", "CR Flamengo", false},
	}
	for _, c := range casos {
		if got := sameTeam(c.tm, c.ours); got != c.want {
			t.Errorf("%q x %q: esperado %v, veio %v", c.tm, c.ours, c.want, got)
		}
	}
}

func TestSamePlayer(t *testing.T) {
	casos := []struct {
		tm, ours string
		want     bool
	}{
		{"Bruno Henrique", "Bruno Henrique", true},
		{"Arrascaeta", "Giorgian de Arrascaeta", true},
		{"Gonzalo Plata", "Gonzalo Plata", true},
		{"Matías Viña", "Matias Vina", true}, // acento
		{"Pedro", "Pedro", true},
		{"Pedro Raul", "Pedro", true},
		{"Bruno Henrique", "Bruno Fuchs", false},
		{"", "Pedro", false},
	}
	for _, c := range casos {
		if got := samePlayer(c.tm, c.ours); got != c.want {
			t.Errorf("%q x %q: esperado %v, veio %v", c.tm, c.ours, c.want, got)
		}
	}
}
