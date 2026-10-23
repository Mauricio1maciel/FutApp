package handlers

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"net/http"
	"sync"
)

func GlobalSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	if query == "" {
		utils.WriteError(w, http.StatusBadRequest, "Informe o termo de busca (ex: ?q=nome)")
		return
	}

	// As três buscas são independentes e vão ao banco ao mesmo tempo: em fila, eram três
	// idas e voltas do Render (EUA) ao banco (São Paulo)
	var (
		leagues                     []models.League
		teams                       []models.Team
		players                     []models.Player
		errLeagues, errTeams, errPl error
		wg                          sync.WaitGroup
	)
	wg.Add(3)
	go func() { defer wg.Done(); leagues, errLeagues = database.SearchLeagues(r.Context(), query) }()
	go func() { defer wg.Done(); teams, errTeams = database.SearchTeamsGlobal(r.Context(), query) }()
	go func() { defer wg.Done(); players, errPl = database.SearchPlayersGlobal(r.Context(), query) }()
	wg.Wait()
	for _, e := range []struct {
		err  error
		what string
	}{{errLeagues, "ligas"}, {errTeams, "times"}, {errPl, "jogadores"}} {
		if e.err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Erro ao buscar "+e.what)
			return
		}
	}

	result := models.SearchResult{
		Leagues: leagues,
		Teams:   teams,
		Players: players,
	}
	utils.WriteJSON(w, http.StatusOK, result)
}
