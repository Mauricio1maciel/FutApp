package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
	"strconv"
)

// MatchCommentaryHandler: a narração lance a lance do jogo, em português, do lance mais
// recente para o mais antigo. GET /match/commentary?id=401912521 (o ID da ESPN, o mesmo do
// /match/history). Jogo sem narração (antes de começar, ou com mais de 30 dias) dá [].
func MatchCommentaryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Informe o id do jogo (o da ESPN): ?id=401912521")
		return
	}
	entries, found, err := database.GetMatchCommentary(r.Context(), id)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar a narração do jogo")
		return
	}
	if !found {
		utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
		return
	}
	utils.WriteJSON(w, http.StatusOK, entries)
}
