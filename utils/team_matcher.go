package utils

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func NormalizeTeamName(name string) string {
	name = strings.ToLower(name)

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	name, _, _ = transform.String(t, name)

	stopWords := []string{
		" fc", " cf", " sad", " cd", " ca", " cp", " ec",
		"esporte clube ", "clube de regatas ", "athletic ", "club ",
	}

	for _, word := range stopWords {
		name = strings.ReplaceAll(name, word, "")
	}

	return strings.TrimSpace(name)
}

func CompareTeams(nameAPI1, nameAPI2 string) bool {
	return NormalizeTeamName(nameAPI1) == NormalizeTeamName(nameAPI2)
}
