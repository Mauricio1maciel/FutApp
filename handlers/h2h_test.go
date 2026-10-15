package handlers

import (
	"App-Futebol/models"
	"testing"
)

func TestSummarizeHeadToHead(t *testing.T) {
	pen := func(n int) *int { return &n }
	matches := []models.Match{
		{APIHomeTeamID: 1, APIAwayTeamID: 2, HomeScore: 2, AwayScore: 0},
		{APIHomeTeamID: 2, APIAwayTeamID: 1, HomeScore: 3, AwayScore: 1}, // time 1 fora, perde
		{APIHomeTeamID: 2, APIAwayTeamID: 1, HomeScore: 1, AwayScore: 1, HomePenalty: pen(5), AwayPenalty: pen(4)},
	}
	got := summarizeHeadToHead(1, matches)
	want := h2hSummary{Played: 3, Team1Wins: 1, Draws: 1, Team2Wins: 1, Team1Goals: 4, Team2Goals: 4}
	if got != want {
		t.Errorf("esperado %+v, veio %+v", want, got)
	}
}
