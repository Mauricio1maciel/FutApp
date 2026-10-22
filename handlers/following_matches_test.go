package handlers

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestFollowingMatchesParametrosInvalidos(t *testing.T) {
	for _, query := range []string{
		"",                         // não segue nada
		"teams=&leagues=&matches=", // idem
		"teams=1783,abc",           // time que não é número
		"matches=-5",               // jogo com número negativo
		"leagues=" + strings.Repeat("X", 11),
		"teams=" + strings.Repeat("1,", 51) + "1", // mais de 50 times
	} {
		rec := httptest.NewRecorder()
		FollowingMatchesHandler(rec, httptest.NewRequest(http.MethodGet, "/matches/following?"+query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("?%s: esperado 400, veio %d", query, rec.Code)
		}
	}
}

func TestParseListas(t *testing.T) {
	if ids, ok := parseIDList(" 1783, 1766 ,,", 50); !ok || !reflect.DeepEqual(ids, []int64{1783, 1766}) {
		t.Errorf("parseIDList: veio %v %v", ids, ok)
	}
	if leagues, ok := parseLeagueList("bsa, CLI", 20); !ok || !reflect.DeepEqual(leagues, []string{"BSA", "CLI"}) {
		t.Errorf("parseLeagueList: veio %v %v", leagues, ok)
	}
}
