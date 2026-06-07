package models

type Team struct {
	ID      int    `json:"id"`
	ApiID   int    `json:"api_id"`
	Name    string `json:"name"`
	Short   string `json:"short"`
	TLA     string `json:"tla"`
	League  string `json:"league"`
	Stadium string `json:"stadium"`
	Crest   string `json:"crestUrl"`
}
