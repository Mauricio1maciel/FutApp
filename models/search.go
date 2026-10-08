package models

type SearchResult struct {
	Leagues []League `json:"leagues"`
	Teams   []Team   `json:"teams"`
	Players []Player `json:"players"`
}

// League é uma liga encontrada na busca
type League struct {
	Code    string `json:"code"` // código usado nas outras rotas (?league=BSA)
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
	Season  string `json:"season"` // temporada mais recente com jogos
}
