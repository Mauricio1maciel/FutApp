package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ESPNMatchDays devolve os dias com jogo da temporada atual de uma liga ("20260924"),
// pelo calendário da própria ESPN. O placar da ESPN não aceita período (só um dia por
// vez), e percorrer o ano inteiro seriam centenas de dias vazios.
func ESPNMatchDays(league string) ([]string, error) {
	url := fmt.Sprintf("https://sports.core.api.espn.com/v2/sports/soccer/leagues/%s/calendar/ondays", getESPNLeague(league))
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendário da ESPN respondeu %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseMatchDays(body)
}

// parseMatchDays lê o calendário da ESPN ({"eventDate": {"dates": ["2026-09-24T07:00Z"]}})
func parseMatchDays(body []byte) ([]string, error) {
	var cal struct {
		EventDate struct {
			Dates []string `json:"dates"`
		} `json:"eventDate"`
	}
	if err := json.Unmarshal(body, &cal); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	days := []string{}
	for _, d := range cal.EventDate.Dates {
		if len(d) < 10 {
			continue
		}
		day := strings.ReplaceAll(d[:10], "-", "")
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	if len(days) == 0 {
		return nil, errors.New("calendário da ESPN sem dias com jogo")
	}
	sort.Strings(days)
	return days, nil
}

// SyncESPNSeason carrega a temporada atual inteira de uma liga que vem só da ESPN: o
// placar de cada dia com jogo (jogos encerrados e completos não buscam o detalhe de
// novo) e a classificação. Cobre a liga incluída com a temporada em andamento (a Liga
// das Nações entrou sem as rodadas anteriores) e os jogos que o worker perdeu enquanto
// estava desligado. Devolve quantos dias foram conferidos; erro só quando o calendário
// da ESPN não responde.
func SyncESPNSeason(ctx context.Context, league string) (int, error) {
	days, err := ESPNMatchDays(league)
	if err != nil {
		return 0, err
	}
	for _, d := range days {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		SyncESPNScoreboardForDate(ctx, league, d)
		time.Sleep(300 * time.Millisecond) // não martelar a ESPN
	}
	// Direto do banco: o cache do ResolveSeason ainda pode ter a temporada anterior
	if season := database.GetLatestSeason(ctx, league); season != "" {
		if err := RecalculateStandings(ctx, league, season); err != nil {
			utils.CustomLog("JOBS", "[%s] Erro ao recalcular a classificação: %v", league, err)
		}
	}
	return len(days), nil
}
