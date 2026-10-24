package handlers

import (
	"App-Futebol/database"
	"App-Futebol/services"
	"App-Futebol/utils"
	"errors"
	"net/http"
	"strconv"
)

// MatchPreviewHandler: a prévia do jogo, pelo ID do nosso banco (match_id) ou pelo da ESPN
// (id). GET /match/preview?match_id=4821 ou /match/preview?id=401912521
func MatchPreviewHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	matchID, _ := strconv.ParseInt(q.Get("match_id"), 10, 64)
	if matchID <= 0 {
		espnID, err := strconv.ParseInt(q.Get("id"), 10, 64)
		if err != nil || espnID <= 0 {
			utils.WriteError(w, http.StatusBadRequest, "Informe o jogo: ?match_id=<ID do banco> ou ?id=<ID da ESPN>")
			return
		}
		if matchID, err = database.ResolveMatchIDByESPN(r.Context(), espnID); err != nil {
			if errors.Is(err, database.ErrNotFound) {
				utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
				return
			}
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar o jogo")
			return
		}
	}

	preview, err := services.MatchPreviewFor(r.Context(), matchID)
	if errors.Is(err, database.ErrNotFound) {
		utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
		return
	}
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao montar a prévia do jogo")
		return
	}
	utils.WriteJSON(w, http.StatusOK, preview)
}
