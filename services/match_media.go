package services

import (
	"App-Futebol/database"
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Lance a lance e vídeos dos jogos, do resumo da ESPN em português. Ele vem num pedido à
// parte: com lang=pt, a ESPN também traduz os tipos dos lances ("Yellow Card" vira "Cartão
// amarelo"), que o resto do código lê em inglês. Em português vêm também os vídeos da ESPN
// Brasil (gols e melhores momentos), que saem durante o jogo e até umas 5 horas depois.

const (
	commentaryMaxAge = 30 * 24 * time.Hour // o lance a lance interessa no jogo e logo depois
	videosMaxAge     = 90 * 24 * time.Hour
	// Os melhores momentos saem horas depois do apito, quando o placar ao vivo já parou de
	// buscar o jogo: os jogos encerrados há menos disso são buscados de novo
	mediaRefreshWindow = 12 * time.Hour
)

// ClearOldMatchMedia apaga as narrações com mais de 30 dias e os vídeos com mais de 90
// (rotina diária do worker)
func ClearOldMatchMedia(ctx context.Context) (commentary, videos int64, err error) {
	if commentary, err = database.ClearOldCommentary(ctx, commentaryMaxAge); err != nil {
		return 0, 0, err
	}
	videos, err = database.ClearOldVideos(ctx, videosMaxAge)
	return commentary, videos, err
}

func recentForCommentary(matchDate string, now time.Time) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z", "2006-01-02 15:04:05"} {
		if start, err := time.Parse(layout, matchDate); err == nil {
			return now.Sub(start) < commentaryMaxAge
		}
	}
	return false
}

type espnCommentary struct {
	Sequence int `json:"sequence"`
	Time     struct {
		DisplayValue string `json:"displayValue"`
	} `json:"time"`
	Text string `json:"text"`
	Play struct {
		Type struct {
			Type string `json:"type"`
		} `json:"type"`
		Team struct {
			DisplayName string `json:"displayName"`
		} `json:"team"`
	} `json:"play"`
}

type espnHref struct {
	Href string `json:"href"`
}

type espnVideo struct {
	ID                  int64  `json:"id"`
	Headline            string `json:"headline"`
	Description         string `json:"description"`
	Duration            int    `json:"duration"` // segundos
	Thumbnail           string `json:"thumbnail"`
	OriginalPublishDate string `json:"originalPublishDate"`
	Links               struct {
		Source struct {
			espnHref          // MP4 na qualidade máxima (centenas de MB)
			HLS      espnHref `json:"HLS"` // se ajusta à conexão (180p a 720p)
		} `json:"source"`
		Web espnHref `json:"web"` // a página do vídeo no espn.com.br
	} `json:"links"`
}

// MatchMedia é o que vem do resumo em português
type MatchMedia struct {
	Commentary []models.CommentaryEntry
	Videos     []models.MatchVideo
}

// fetchPortugueseSummary busca a narração e os vídeos do jogo no resumo da ESPN em português
func fetchPortugueseSummary(espnLeague, matchID string) (MatchMedia, error) {
	url := fmt.Sprintf("https://site.api.espn.com/apis/site/v2/sports/soccer/%s/summary?event=%s&lang=pt&region=br", espnLeague, matchID)
	resp, err := httpClient.Get(url)
	if err != nil {
		return MatchMedia{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MatchMedia{}, fmt.Errorf("ESPN respondeu %s", resp.Status)
	}
	var data struct {
		Commentary []espnCommentary `json:"commentary"`
		Videos     []espnVideo      `json:"videos"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return MatchMedia{}, err
	}
	return MatchMedia{Commentary: commentaryEntries(data.Commentary), Videos: matchVideos(data.Videos)}, nil
}

func commentaryEntries(raw []espnCommentary) []models.CommentaryEntry {
	out := make([]models.CommentaryEntry, 0, len(raw))
	for _, c := range raw {
		if strings.TrimSpace(c.Text) == "" {
			continue
		}
		out = append(out, models.CommentaryEntry{Seq: c.Sequence, Minute: c.Time.DisplayValue,
			Text: strings.TrimSpace(c.Text), Type: c.Play.Type.Type, Team: c.Play.Team.DisplayName})
	}
	return out
}

// matchVideos converte os vídeos da ESPN: primeiro os melhores momentos e a compilação dos
// gols, depois o resto (entrevistas, reações), do mais novo para o mais antigo. Vídeo sem
// link que toque fora do site da ESPN (nem HLS nem MP4) fica de fora
func matchVideos(raw []espnVideo) []models.MatchVideo {
	out := make([]models.MatchVideo, 0, len(raw))
	for _, v := range raw {
		hls, mp4 := v.Links.Source.HLS.Href, v.Links.Source.Href
		if hls == "" && mp4 == "" {
			continue
		}
		title := strings.TrimSpace(v.Headline)
		lower := strings.ToLower(title)
		out = append(out, models.MatchVideo{
			ID: strconv.FormatInt(v.ID, 10), Title: title, Description: strings.TrimSpace(v.Description),
			DurationSeconds: v.Duration, Thumbnail: v.Thumbnail, HLSURL: hls, MP4URL: mp4,
			WebURL: v.Links.Web.Href, PublishedAt: v.OriginalPublishDate,
			Highlights: strings.Contains(lower, "melhores momentos") || strings.Contains(lower, "veja os gols") ||
				strings.Contains(lower, "highlights"),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Highlights != out[j].Highlights {
			return out[i].Highlights
		}
		return out[i].PublishedAt > out[j].PublishedAt
	})
	return out
}

// mediaFetcher busca o resumo em português. Variável para os testes
var mediaFetcher = fetchPortugueseSummary

// RefreshRecentMatchMedia busca de novo a narração e os vídeos dos jogos encerrados nas
// últimas 12 horas (os melhores momentos saem horas depois do apito). Devolve quantos
// jogos ganharam vídeo novo
func RefreshRecentMatchMedia(ctx context.Context) (int, error) {
	matches, err := database.GetRecentFinishedESPNMatches(ctx, mediaRefreshWindow)
	if err != nil {
		return 0, err
	}
	updated := 0
	for i, m := range matches {
		if ctx.Err() != nil {
			return updated, ctx.Err()
		}
		if i > 0 {
			time.Sleep(time.Second) // não martelar a ESPN
		}
		media, err := mediaFetcher(getESPNLeague(m.League), strconv.FormatInt(m.ESPNMatchID, 10))
		if err != nil {
			utils.CustomLog("WORKER_ESPN", "Vídeos do jogo %d indisponíveis: %v", m.ESPNMatchID, err)
			continue
		}
		changed, err := database.UpdateMatchMedia(ctx, m.ESPNMatchID, media.Commentary, media.Videos)
		if err != nil {
			return updated, err
		}
		if changed && len(media.Videos) > m.Videos {
			updated++
		}
	}
	return updated, nil
}
