package services

import (
	"encoding/json"
	"io"
	"net/http"

	"App-Futebol/models"
)

type MatchesResponse struct {
	Events []models.Event `json:"events"`
}

func GetSerieBMatches() ([]models.Event, error) {

	url := "https://www.thesportsdb.com/api/v1/json/3/eventsseason.php?id=4404"

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data MatchesResponse

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return data.Events, nil
}
