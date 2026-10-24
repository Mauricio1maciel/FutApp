package handlers

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"net/http"
	"strconv"
)

// MatchVideosHandler: os vídeos do jogo (gols e melhores momentos da ESPN Brasil), do mais
// novo para o mais antigo. GET /match/videos?id=401912521 (o ID da ESPN). Sem vídeos: [].
func MatchVideosHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "Informe o id do jogo (o da ESPN): ?id=401912521")
		return
	}
	videos, found, err := database.GetMatchVideos(r.Context(), id)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar os vídeos do jogo")
		return
	}
	if !found {
		utils.WriteError(w, http.StatusNotFound, "Jogo não encontrado")
		return
	}
	utils.WriteJSON(w, http.StatusOK, videos)
}
