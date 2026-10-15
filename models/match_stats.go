package models

import "math"

// MatchStatistics são as estatísticas cruas da ESPN, pelo nome dela (ex: "totalShots")
type MatchStatistics struct {
	Home map[string]float64 `json:"home"`
	Away map[string]float64 `json:"away"`
}

// MatchStat é uma linha das estatísticas do jogo, como o app mostra
type MatchStat struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Home  float64 `json:"home"`
	Away  float64 `json:"away"`
	Unit  string  `json:"unit"` // "%" ou ""
}

// Estatísticas que o app mostra, na ordem. As outras ficam guardadas para uso futuro.
var statLines = []struct{ key, label, unit string }{
	{"possessionPct", "Posse de bola", "%"},
	{"totalShots", "Finalizações", ""},
	{"shotsOnTarget", "Finalizações no gol", ""},
	{"wonCorners", "Escanteios", ""},
	{"offsides", "Impedimentos", ""},
	{"foulsCommitted", "Faltas", ""},
	{"yellowCards", "Cartões amarelos", ""},
	{"redCards", "Cartões vermelhos", ""},
	{"saves", "Defesas do goleiro", ""},
	{"totalPasses", "Passes", ""},
	{"passAccuracy", "Precisão dos passes", "%"},
	{"totalTackles", "Desarmes", ""},
	{"interceptions", "Interceptações", ""},
}

// MatchStatList monta a lista que o app mostra. Uma linha só entra se os dois times
// tiverem o número. A precisão dos passes é calculada (certos ÷ total): a ESPN manda
// arredondada a uma casa ("0.9"), o que viraria 90% num jogo de 88%.
func MatchStatList(s *MatchStatistics) []MatchStat {
	list := []MatchStat{}
	if s == nil {
		return list
	}
	for _, l := range statLines {
		home, okHome := statValue(s.Home, l.key)
		away, okAway := statValue(s.Away, l.key)
		if okHome && okAway {
			list = append(list, MatchStat{Key: l.key, Label: l.label, Home: home, Away: away, Unit: l.unit})
		}
	}
	return list
}

func statValue(team map[string]float64, key string) (float64, bool) {
	if key == "passAccuracy" {
		total, ok := team["totalPasses"]
		if !ok || total == 0 {
			return 0, false
		}
		return math.Round(team["accuratePasses"] / total * 100), true
	}
	v, ok := team[key]
	return v, ok
}
