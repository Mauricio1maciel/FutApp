package models

type SearchResult struct {
	Teams   []Team   `json:"teams"`
	Players []Player `json:"players"`
}
