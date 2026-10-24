package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMatchPreviewParametros(t *testing.T) {
	for _, query := range []string{"", "match_id=abc", "id=-1", "match_id=0&id=x"} {
		rec := httptest.NewRecorder()
		MatchPreviewHandler(rec, httptest.NewRequest(http.MethodGet, "/match/preview?"+query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?%s: esperado 400, veio %d", query, rec.Code)
		}
	}
}
