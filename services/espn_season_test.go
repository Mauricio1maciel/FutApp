package services

import (
	"reflect"
	"testing"
)

func TestParseMatchDays(t *testing.T) {
	// Formato real do calendar/ondays, fora de ordem, com dia repetido e um valor inválido
	body := []byte(`{"eventDate": {"type": "ondays", "dates": [
	  "2026-10-01T07:00Z", "2026-09-24T07:00Z", "2026-09-24T19:00Z", "x", "2026-11-17T08:00Z"]}}`)
	got, err := parseMatchDays(body)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"20260924", "20261001", "20261117"}; !reflect.DeepEqual(got, want) {
		t.Errorf("esperado %v, veio %v", want, got)
	}

	if _, err := parseMatchDays([]byte(`{"eventDate": {"dates": []}}`)); err == nil {
		t.Error("calendário vazio deveria dar erro (o worker cai na janela de dias)")
	}
}
