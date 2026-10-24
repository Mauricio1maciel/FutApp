package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchCommentaryParametros(t *testing.T) {
	for _, query := range []string{"", "id=abc", "id=-3"} {
		rec := httptest.NewRecorder()
		MatchCommentaryHandler(rec, httptest.NewRequest(http.MethodGet, "/match/commentary?"+query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?%s: esperado 400, veio %d", query, rec.Code)
		}
	}
}
