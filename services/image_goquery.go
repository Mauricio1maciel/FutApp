package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/gocolly/colly/v2"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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

		if samePlayer(name, playerName) && sameTeam(team, teamName) {

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

var imageBotRunning atomic.Bool

// StartImageBot sobe o robô em background apenas se ele ainda não estiver rodando.
func StartImageBot() bool {
	if !imageBotRunning.CompareAndSwap(false, true) {
		return false
	}
	// Contexto próprio: o robô roda em background e não pode morrer junto
	// com a requisição do admin que o disparou
	go RunImageBot(context.Background())
	return true
}

// RunImageBot procura no Transfermarkt a foto (e o valor de mercado) dos jogadores sem
// foto, começando pelos que aparecem em escalações (os que o app mostra). Não achou:
// tenta de novo depois de 30 dias. Achou: atualiza a cada 90 dias.
func RunImageBot(ctx context.Context) {
	utils.CustomLog("BOT", "🤖 Robô iniciado: Monitorando fotos e valores de mercado.")

	// A versão antiga gravava 'NOT_FOUND' no lugar da foto (o app recebia isso como
	// endereço de imagem): volta a vazio, e esses jogadores entram na fila de novo
	database.DB.Exec(`UPDATE espn_players SET headshot_url = '' WHERE headshot_url = 'NOT_FOUND'`)

	for {
		query := `
			SELECT ep.id, ep.name, t.name
			FROM espn_players ep
			JOIN teams t ON ep.espn_team_id = t.espn_team_id
			LEFT JOIN (SELECT DISTINCT espn_player_id FROM espn_match_lineups) l
			       ON l.espn_player_id = ep.espn_id
			WHERE (COALESCE(ep.headshot_url, '') = ''
			       AND (ep.photo_checked_at IS NULL OR ep.photo_checked_at < NOW() - INTERVAL '30 days'))
			   OR (ep.headshot_url LIKE '%transfermarkt%' AND ep.photo_checked_at < NOW() - INTERVAL '90 days')
			ORDER BY (l.espn_player_id IS NOT NULL) DESC, ep.photo_checked_at NULLS FIRST
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
				_, err := database.DB.Exec(`UPDATE espn_players SET headshot_url = $1, market_value = $2,
                    photo_checked_at = NOW(), last_updated = NOW() WHERE id = $3`, img, val, p.ID)
				if err == nil {
					utils.CustomLog("BOT", "✅ [%s] OK | Valor: %s", p.Name, val)
				}
			} else {
				// Não achou: a foto continua vazia (o app mostra as iniciais) e ele tenta de novo
				// daqui a 30 dias. Antes gravava 'NOT_FOUND' no lugar da foto
				database.DB.Exec(`UPDATE espn_players SET photo_checked_at = NOW() WHERE id = $1`, p.ID)
			}
			time.Sleep(5 * time.Second)
		}
		time.Sleep(1 * time.Minute)
	}
}

// Palavras que não identificam o clube ("CR Flamengo" e "Flamengo" são o mesmo time)
var clubWords = map[string]bool{
	"fc": true, "cf": true, "sc": true, "ec": true, "cr": true, "se": true, "ca": true, "ac": true,
	"afc": true, "club": true, "clube": true, "esporte": true, "esportes": true, "futebol": true,
	"sport": true, "de": true, "do": true, "da": true, "das": true, "dos": true, "the": true,
	"cd": true, "rb": true, "us": true, "ss": true, "as": true, "sv": true, "vfl": true,
	"tsg": true, "fk": true, "sk": true, "ud": true, "rc": true, "rcd": true, "fbpa": true,
}

var stripAccents = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// normalizeName: minúsculas, sem acento e sem pontuação ("Atlético-MG" → "atletico mg")
func normalizeName(s string) string {
	s, _, _ = transform.String(stripAccents, strings.ToLower(s))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// sameTeam compara o time do Transfermarkt com o nosso: um nome contém o outro, ou os dois
// têm uma palavra própria em comum ("Sporting CP" e "Sporting Clube de Portugal")
func sameTeam(tm, ours string) bool {
	a, b := normalizeName(tm), normalizeName(ours)
	if a == "" || b == "" {
		return false
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	words := map[string]bool{}
	for _, w := range strings.Fields(a) {
		if !clubWords[w] && len(w) >= 4 {
			words[w] = true
		}
	}
	for _, w := range strings.Fields(b) {
		if words[w] {
			return true
		}
	}
	return false
}

// samePlayer: todas as palavras de um nome aparecem no outro, sem acento ("Bruno Henrique"
// e "Bruno Henrique Pinto")
func samePlayer(tm, ours string) bool {
	a, b := strings.Fields(normalizeName(tm)), strings.Fields(normalizeName(ours))
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	contains := func(all, part []string) bool {
		set := map[string]bool{}
		for _, w := range all {
			set[w] = true
		}
		for _, w := range part {
			if !set[w] {
				return false
			}
		}
		return true
	}
	return contains(a, b) || contains(b, a)
}
