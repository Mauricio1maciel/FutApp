package utils

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Palavras que não identificam o time (siglas de clube, preposições etc.)
var teamStopWords = map[string]bool{
	"fc": true, "cf": true, "afc": true, "sc": true, "ac": true, "as": true,
	"sk": true, "fk": true, "sv": true, "cd": true, "ca": true, "cp": true,
	"ec": true, "cr": true, "se": true, "rc": true, "sad": true, "pae": true,
	"club": true, "clube": true, "esporte": true, "regatas": true, "futebol": true,
	"de": true, "da": true, "do": true, "del": true, "the": true,
}

// TeamTokens devolve as palavras que identificam o time:
// minúsculas, sem acento, sem siglas genéricas e sem números (ex: "FC Schalke 04" -> [schalke])
func TeamTokens(name string) []string {
	name = strings.ToLower(name)

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	name, _, _ = transform.String(t, name)

	words := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	var all []string
	for _, w := range words {
		all = append(all, w)
		if teamStopWords[w] || isNumber(w) {
			continue
		}
		tokens = append(tokens, w)
	}

	// Nome formado só por palavras genéricas: usa tudo para não ficar vazio
	if len(tokens) == 0 {
		return all
	}
	return tokens
}

func NormalizeTeamName(name string) string {
	return strings.Join(TeamTokens(name), " ")
}

func CompareTeams(nameAPI1, nameAPI2 string) bool {
	n1, n2 := NormalizeTeamName(nameAPI1), NormalizeTeamName(nameAPI2)
	return n1 != "" && n1 == n2
}

// TeamTokensSubset indica se todas as palavras de um nome estão no outro
// (ex: "Racing Santander" ⊂ "Real Racing Club de Santander").
// Só deve ser usado quando houver um único candidato, pois é menos preciso que CompareTeams.
func TeamTokensSubset(nameA, nameB string) bool {
	a, b := TeamTokens(nameA), TeamTokens(nameB)
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	if len(a) > len(b) {
		a, b = b, a
	}

	set := make(map[string]bool, len(b))
	for _, w := range b {
		set[w] = true
	}
	for _, w := range a {
		if !set[w] {
			return false
		}
	}
	return true
}

func isNumber(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}
