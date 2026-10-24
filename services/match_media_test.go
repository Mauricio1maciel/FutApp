package services

import (
	"encoding/json"
	"testing"
)

func TestMatchVideos(t *testing.T) {
	const raw = `[
	  {"id": 1, "headline": "VEJA a reação de Seedorf ao gol", "duration": 50, "thumbnail": "t1.jpg",
	   "originalPublishDate": "2026-10-20T00:52:59Z",
	   "links": {"source": {"href": "a.mp4", "HLS": {"href": "a.m3u8"}}, "web": {"href": "https://www.espn.com.br/video/clip/_/id/1"}}},
	  {"id": 2, "headline": "Corinthians e Vitória empatam; VEJA os Melhores Momentos", "duration": 242,
	   "originalPublishDate": "2026-10-20T01:38:00Z",
	   "links": {"source": {"href": "b.mp4", "HLS": {"href": "b.m3u8"}}, "web": {"href": "w2"}}},
	  {"id": 4, "headline": "Técnico fala depois do empate", "duration": 30, "originalPublishDate": "2026-10-20T04:05:00Z",
	   "links": {"source": {"href": "d.mp4", "HLS": {"href": "d.m3u8"}}}},
	  {"id": 3, "headline": "Só no site da ESPN", "originalPublishDate": "2026-10-20T03:00:00Z",
	   "links": {"web": {"href": "w3"}}}
	]`
	var v []espnVideo
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	got := matchVideos(v)
	if len(got) != 3 {
		t.Fatalf("o vídeo sem HLS nem MP4 fica de fora: esperado 3, veio %+v", got)
	}
	if got[0].ID != "2" || !got[0].Highlights || got[0].HLSURL != "b.m3u8" || got[0].MP4URL != "b.mp4" || got[0].DurationSeconds != 242 {
		t.Errorf("os melhores momentos primeiro, mesmo com vídeo mais novo: %+v", got[0])
	}
	if got[1].ID != "4" || got[2].ID != "1" {
		t.Errorf("depois, do mais novo para o mais antigo: %+v", got)
	}
	if got[2].Highlights || got[2].WebURL != "https://www.espn.com.br/video/clip/_/id/1" || got[2].Thumbnail != "t1.jpg" {
		t.Errorf("vídeo da reação ao gol: %+v", got[2])
	}
}
