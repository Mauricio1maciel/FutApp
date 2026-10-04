package models

// ESPNTeam é um time como listado no endpoint /teams da ESPN
type ESPNTeam struct {
	ID           int64
	DisplayName  string
	ShortName    string
	Abbreviation string
}
