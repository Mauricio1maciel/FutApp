package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"App-Futebol/models"
)

func GetMatchesByLeagueCode(leagueCode string) ([]models.FDMatch, error) {

	url := fmt.Sprintf(
		"https://api.football-data.org/v4/competitions/%s/matches",
		leagueCode,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	token := os.Getenv("API_TOKEN")

	req.Header.Set("X-Auth-Token", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erro API: %s - %s", resp.Status, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data models.FDResponse

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	return data.Matches, nil
}
