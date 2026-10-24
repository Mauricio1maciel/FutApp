package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTeamCardsParametros(t *testing.T) {
	for query, want := range map[string]int{
		"":                   400,
		"id=abc&league=BSA":  400,
		"id=1783":            400,
		"id=1783&league=CLI": 404, // regra de cartões ainda não cadastrada
	} {
		rec := httptest.NewRecorder()
		TeamCardsHandler(rec, httptest.NewRequest(http.MethodGet, "/team/cards?"+query, nil))
		if rec.Code != want {
			t.Errorf("?%s: esperado %d, veio %d", query, want, rec.Code)
		}
	}
}
