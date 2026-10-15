package models

import "testing"

func TestMatchStatList(t *testing.T) {
	s := &MatchStatistics{
		Home: map[string]float64{"possessionPct": 50.2, "totalShots": 21, "accuratePasses": 483, "totalPasses": 549, "saves": 2},
		Away: map[string]float64{"possessionPct": 49.8, "totalShots": 10, "accuratePasses": 479, "totalPasses": 553},
	}
	got := MatchStatList(s)

	// "saves" só do mandante: a linha não entra
	want := []MatchStat{
		{Key: "possessionPct", Label: "Posse de bola", Home: 50.2, Away: 49.8, Unit: "%"},
		{Key: "totalShots", Label: "Finalizações", Home: 21, Away: 10},
		{Key: "totalPasses", Label: "Passes", Home: 549, Away: 553},
		{Key: "passAccuracy", Label: "Precisão dos passes", Home: 88, Away: 87, Unit: "%"},
	}
	if len(got) != len(want) {
		t.Fatalf("esperado %d linhas, veio %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("linha %d: esperado %+v, veio %+v", i, want[i], got[i])
		}
	}

	if l := MatchStatList(nil); l == nil || len(l) != 0 {
		t.Errorf("sem estatísticas deveria ser lista vazia (não null), veio %v", l)
	}
}
