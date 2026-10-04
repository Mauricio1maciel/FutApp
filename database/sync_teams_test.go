package database

import (
	"App-Futebol/models"
	"testing"
)

func TestFindESPNTeam(t *testing.T) {
	espnBundesliga := []models.ESPNTeam{
		{ID: 126, DisplayName: "SC Freiburg", ShortName: "Freiburg"},
		{ID: 3307, DisplayName: "SC Paderborn 07", ShortName: "Paderborn"},
		{ID: 133, DisplayName: "Schalke 04", ShortName: "Schalke"},
		{ID: 10388, DisplayName: "SV Elversberg", ShortName: "Elversberg"},
	}
	espnChampions := []models.ESPNTeam{
		{ID: 437, DisplayName: "FC Porto", ShortName: "FC Porto"},
		{ID: 2250, DisplayName: "Sporting CP", ShortName: "Sporting"},
		{ID: 2572, DisplayName: "Como", ShortName: "Como"},
		{ID: 493, DisplayName: "Shakhtar Donetsk", ShortName: "Shakhtar"},
		{ID: 887, DisplayName: "AEK Athens", ShortName: "AEK Athens"},
		{ID: 436, DisplayName: "Fenerbahce", ShortName: "Fenerbahce"},
	}
	espnLaLiga := []models.ESPNTeam{
		{ID: 87, DisplayName: "Racing Santander", ShortName: "Racing"},
		{ID: 99, DisplayName: "Málaga", ShortName: "Málaga"},
		{ID: 86, DisplayName: "Real Madrid", ShortName: "Real Madrid"},
		{ID: 83, DisplayName: "Barcelona", ShortName: "Barcelona"},
	}

	tests := []struct {
		name, short string
		espn        []models.ESPNTeam
		wantID      int64
		wantOK      bool
	}{
		{"FC Schalke 04", "Schalke", espnBundesliga, 133, true},
		{"SC Paderborn 07", "SC Paderborn", espnBundesliga, 3307, true},
		{"SV 07 Elversberg", "Elversberg", espnBundesliga, 10388, true},
		{"FC Porto", "Porto", espnChampions, 437, true},
		{"Como 1907", "Como 1907", espnChampions, 2572, true},
		{"FK Shakhtar Donetsk", "Shaktar", espnChampions, 493, true},
		{"PAE AEK", "PAE AEK", espnChampions, 887, true},
		{"Fenerbahçe SK", "Fenerbahçe", espnChampions, 436, true},
		{"Real Racing Club de Santander", "Santander", espnLaLiga, 87, true},
		{"Málaga CF", "Málaga", espnLaLiga, 99, true},
		{"Real Madrid CF", "Real Madrid", espnLaLiga, 86, true},
		// Sem correspondência: não pode vincular a um time qualquer
		{"ŠK Slovan Bratislava", "Sl. Bratislava", espnChampions, 0, false},
		{"Real Sociedad", "Real Sociedad", espnLaLiga, 0, false},
	}

	for _, tt := range tests {
		got, ok := findESPNTeam(tt.name, tt.short, tt.espn)
		if ok != tt.wantOK || got.ID != tt.wantID {
			t.Errorf("findESPNTeam(%q) = (%d, %v), esperado (%d, %v)", tt.name, got.ID, ok, tt.wantID, tt.wantOK)
		}
	}
}
