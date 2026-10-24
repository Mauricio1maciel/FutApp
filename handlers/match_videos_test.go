package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchVideosParametros(t *testing.T) {
	for _, query := range []string{"", "id=abc", "id=0"} {
		rec := httptest.NewRecorder()
		MatchVideosHandler(rec, httptest.NewRequest(http.MethodGet, "/match/videos?"+query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?%s: esperado 400, veio %d", query, rec.Code)
		}
	}
}
