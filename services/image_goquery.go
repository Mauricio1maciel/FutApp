package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

func SearchPlayerDetails(playerName string, teamName string) (string, string) {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
	)
	c.Limit(&colly.LimitRule{DomainGlob: "*transfermarkt*", Delay: 2 * time.Second})

	var profileURL string
	var marketValue string

	safeQuery := url.QueryEscape(playerName)
	searchURL := fmt.Sprintf("https://www.transfermarkt.com.br/schnellsuche/ergebnis/schnellsuche?query=%s", safeQuery)

	c.OnHTML("table.items tbody tr", func(e *colly.HTMLElement) {
		if profileURL != "" {
			return
		}

		name := e.ChildText("td.hauptlink")
		team := e.ChildText("td table.inline-table a")

		val := e.ChildText("td.rechts.hauptlink")

		if strings.Contains(strings.ToLower(name), strings.ToLower(playerName)) &&
			strings.Contains(strings.ToLower(team), strings.ToLower(teamName)) {

			profileURL = "https://www.transfermarkt.com.br" + e.ChildAttr("td.hauptlink a", "href")
			marketValue = strings.TrimSpace(val)
		}
	})
	c.Visit(searchURL)

	if profileURL == "" {
		return "", ""
	}

	var imageURL string
	c2 := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
	)
	c2.OnHTML("img.data-header__profile-image", func(e *colly.HTMLElement) { imageURL = e.Attr("src") })
	c2.Visit(profileURL)

	return imageURL, marketValue
}

func RunImageBot() {
	utils.CustomLog("BOT", "🤖 Robô iniciado: Monitorando fotos e valores de mercado.")

	for {
		query := `
			SELECT ep.id, ep.name, t.name
			FROM espn_players ep
			JOIN teams t ON ep.espn_team_id = t.espn_team_id
			WHERE (headshot_url IS NULL OR headshot_url = '' OR last_updated < NOW() - INTERVAL '90 days'
			)
			LIMIT 50
		`

		rows, err := database.DB.Query(query)
		if err != nil {
			utils.CustomLog("BOT", "Erro DB: %v", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		type Player struct {
			ID   int
			Name string
			Team string
		}
		var batch []Player
		for rows.Next() {
			var p Player
			rows.Scan(&p.ID, &p.Name, &p.Team)
			batch = append(batch, p)
		}
		rows.Close()

		if len(batch) == 0 {
			utils.CustomLog("BOT", "✅ Todos atualizados. Pausa (6h).")
			time.Sleep(6 * time.Hour)
			continue
		}

		for _, p := range batch {
			img, val := SearchPlayerDetails(p.Name, p.Team)

			if img != "" {
				_, err := database.DB.Exec(`UPDATE espn_players SET headshot_url = $1, market_value = $2, last_updated = NOW() WHERE id = $3`, img, val, p.ID)
				if err == nil {
					utils.CustomLog("BOT", "✅ [%s] OK | Valor: %s", p.Name, val)
				}
			} else {
				database.DB.Exec("UPDATE espn_players SET headshot_url = 'NOT_FOUND' WHERE id = $1", p.ID)
			}
			time.Sleep(5 * time.Second)
		}
		time.Sleep(1 * time.Minute)
	}
}
