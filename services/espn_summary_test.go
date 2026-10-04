package services

import (
	"encoding/json"
	"testing"
)

func TestGroupFromCompetitors(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	tests := []struct {
		nome   string
		groups []json.RawMessage
		want   string
	}{
		{"mesmo grupo", []json.RawMessage{raw(`{"name":"Group C1"}`), raw(`{"name":"Group C1"}`)}, "Group C1"},
		{"grupos diferentes", []json.RawMessage{raw(`{"name":"Group C1"}`), raw(`{"name":"Group B1"}`)}, ""},
		{"sem grupo (liga de clubes)", []json.RawMessage{nil, nil}, ""},
		{"nome que não é grupo", []json.RawMessage{raw(`{"name":"Premier League"}`), raw(`{"name":"Premier League"}`)}, ""},
		{"formato inesperado (lista)", []json.RawMessage{raw(`[{"name":"Group C1"}]`), raw(`[{"name":"Group C1"}]`)}, ""},
	}

	for _, tt := range tests {
		if got := groupFromCompetitors(tt.groups); got != tt.want {
			t.Errorf("%s: esperado %q, veio %q", tt.nome, tt.want, got)
		}
	}
}

func TestSeasonFromESPN(t *testing.T) {
	tests := []struct {
		year int
		name string
		want string
	}{
		{2026, "2026-27 UEFA Nations League, League Phase", "2026-2027"},
		{2026, "2026-27 UEFA Nations League", "2026-2027"},
		{2026, "2026 Brasileirão Série A", "2026"},
		{2026, "", "2026"},
	}

	for _, tt := range tests {
		if got := seasonFromESPN(tt.year, tt.name); got != tt.want {
			t.Errorf("seasonFromESPN(%d, %q) = %q, esperado %q", tt.year, tt.name, got, tt.want)
		}
	}
}
