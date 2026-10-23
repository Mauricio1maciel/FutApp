package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Horário de Brasília (sem horário de verão desde 2019): "hoje" é o dia daqui
var brasilia = time.FixedZone("BRT", -3*60*60)

// FollowingMatchesHandler devolve os jogos de quem a pessoa segue, numa chamada só ("Meus
// jogos" na tela inicial): GET /matches/following?teams=1783,1766&leagues=BSA&matches=4821
// [&past=0&days=0]. Por padrão, só os jogos de hoje (no horário de Brasília) e os que estão
// em andamento; past e days abrem a janela para dias atrás e à frente.
//
// O app manda o que segue (as listas do GET /push/subscriptions), em vez de o servidor
// procurar pelo aparelho: assim a resposta só depende do endereço e entra no cache, e o
// app também pode mandar times e ligas que a pessoa marcou sem ligar as notificações.
func FollowingMatchesHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	teams, ok1 := parseIDList(q.Get("teams"), 50)
	matchIDs, ok2 := parseIDList(q.Get("matches"), 100)
	leagues, ok3 := parseLeagueList(q.Get("leagues"), 20)
	if !ok1 || !ok2 || !ok3 {
		utils.WriteError(w, http.StatusBadRequest,
			"Use números separados por vírgula em teams e matches (até 50 times e 100 jogos) e até 20 ligas em leagues")
		return
	}
	if len(teams)+len(leagues)+len(matchIDs) == 0 {
		utils.WriteError(w, http.StatusBadRequest, "Informe o que a pessoa segue: teams, leagues ou matches")
		return
	}
	past, days := 0, 0 // só hoje
	if n, err := strconv.Atoi(q.Get("past")); err == nil && n >= 0 && n <= 7 {
		past = n
	}
	if n, err := strconv.Atoi(q.Get("days")); err == nil && n >= 0 && n <= 14 {
		days = n
	}

	now := time.Now().In(brasilia)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, brasilia)
	matches, err := database.GetFollowedMatches(r.Context(), teams, leagues, matchIDs,
		today.AddDate(0, 0, -past), today.AddDate(0, 0, days+1))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar os jogos de quem você segue")
		return
	}
	utils.WriteJSON(w, http.StatusOK, matches)
}

// parseIDList lê "1783, 1766" (vazio = nenhum). Falso se algum não for um número positivo
// ou se passar de max
func parseIDList(s string, max int) ([]int64, bool) {
	var ids []int64
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, len(ids) <= max
}

// parseLeagueList lê "bsa, CLI" como códigos de liga em maiúsculas (vazio = nenhuma)
func parseLeagueList(s string, max int) ([]string, bool) {
	var leagues []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.ToUpper(strings.TrimSpace(part)); part == "" {
			continue
		}
		if len(part) > 10 {
			return nil, false
		}
		leagues = append(leagues, part)
	}
	return leagues, len(leagues) <= max
}
