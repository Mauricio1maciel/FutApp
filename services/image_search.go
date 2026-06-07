// package services

// import (
// 	"App-Futebol/database"
// 	"App-Futebol/utils"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"net/url"
// )

// // Constantes da API do Google (Você cria isso de graça no Google Cloud Console)
// const GoogleAPIKey = "AIzaSyCycQ39sBjTyTcuaQDPamsyAH2goOTXfPs"
// const GoogleSearchEngineID = "46d5b813678c34225"

// // Estrutura para ler a resposta do Google Images
// type GoogleImageSearchResponse struct {
// 	Items []struct {
// 		Link string `json:"link"` // A URL da imagem
// 	} `json:"items"`
// }

// // SearchPlayerImage faz a pesquisa da foto do rosto no Google
// func SearchPlayerImage(playerName string, teamName string) string {
// 	// 🔥 1. Tiramos as aspas duplas! Agora o Google tem liberdade para achar o jogador
// 	query := fmt.Sprintf(`%s %s headshot`, playerName, teamName)
// 	encodedQuery := url.QueryEscape(query)

// 	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?q=%s&cx=%s&key=%s&searchType=image&num=1",
// 		encodedQuery, GoogleSearchEngineID, GoogleAPIKey)

// 	resp, err := http.Get(apiURL)
// 	if err != nil {
// 		utils.CustomLog("IMAGE_SEARCH", "Erro na requisição: %v", err)
// 		return ""
// 	}
// 	defer resp.Body.Close()

// 	// 🔥 2. O LOG DETETIVE: Se o Google der erro (qualquer status diferente de 200 OK), ele dedura!
// 	if resp.StatusCode != 200 {
// 		bodyBytes, _ := io.ReadAll(resp.Body)
// 		utils.CustomLog("IMAGE_SEARCH", "❌ O Google bloqueou a pesquisa! Status: %d | Motivo: %s", resp.StatusCode, string(bodyBytes))
// 		return ""
// 	}

// 	var result GoogleImageSearchResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		utils.CustomLog("IMAGE_SEARCH", "Erro ao decodificar resposta: %v", err)
// 		return ""
// 	}

// 	if len(result.Items) > 0 {
// 		return result.Items[0].Link
// 	}

// 	// Se chegou aqui, a pesquisa deu certo mas o Google realmente não achou foto
// 	utils.CustomLog("IMAGE_SEARCH", "👀 Zero fotos encontradas para: %s", query)
// 	return ""
// }

// // RunImageBot procura no banco quem está com a url_player NULL e atualiza
// func RunImageBot() {
// 	utils.CustomLog("BOT", "Iniciando robô de busca de imagens...")

// 	// 1. Busca até 50 jogadores que estão sem foto (para não gastar toda a cota do Google de uma vez)
// 	query := `
// 		SELECT ep.id, ep.name, t.name
// 		FROM espn_players ep
// 		join teams t on ep.espn_team_id  = t.espn_team_id
// 		WHERE headshot_url IS NULL OR headshot_url = ''
// 		LIMIT 50
// 	`
// 	rows, err := database.DB.Query(query)
// 	if err != nil {
// 		utils.CustomLog("BOT", "Erro ao buscar jogadores sem foto: %v", err)
// 		return
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var id int
// 		var name, team string
// 		rows.Scan(&id, &name, &team)

// 		// 2. Chama a pesquisa na internet
// 		imageURL := SearchPlayerImage(name, team)

// 		if imageURL != "" {
// 			// 3. Atualiza o banco de dados!
// 			updateQuery := `UPDATE espn_players SET headshot_url = $1 WHERE id = $2`
// 			_, err := database.DB.Exec(updateQuery, imageURL, id)

//				if err == nil {
//					utils.CustomLog("BOT", "✅ Foto encontrada para %s: %s", name, imageURL)
//				}
//			} else {
//				// Coloca uma string padrão tipo 'NOT_FOUND' para o robô não ficar pesquisando o mesmo cara sem sucesso todo dia
//				database.DB.Exec(`UPDATE espn_players SET headshot_url = 'NOT_FOUND' WHERE id = $1`, id)
//			}
//		}
//		utils.CustomLog("BOT", "Fim do ciclo do robô de imagens.")
//	}
package services
