package database

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// O worker espera o banco voltar (5 s, 10 s, 20 s... até 1 min entre as tentativas)
func TestWaitForPingTentaDeNovo(t *testing.T) {
	falhas := 4
	var esperas []time.Duration
	waitForPing(func() error {
		if falhas > 0 {
			falhas--
			return errors.New("sem internet")
		}
		return nil
	}, func(d time.Duration) { esperas = append(esperas, d) })

	if want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second}; !reflect.DeepEqual(esperas, want) {
		t.Errorf("esperas: veio %v, esperado %v", esperas, want)
	}

	esperas = nil
	falhas = 6
	waitForPing(func() error {
		if falhas > 0 {
			falhas--
			return errors.New("sem internet")
		}
		return nil
	}, func(d time.Duration) { esperas = append(esperas, d) })
	if esperas[len(esperas)-1] != time.Minute {
		t.Errorf("a espera não passa de 1 minuto: %v", esperas)
	}
}
