-- Vídeos dos jogos (gols e melhores momentos) da ESPN Brasil, em espn_matches.videos: título,
-- duração, miniatura e os links (HLS, que se ajusta à conexão, MP4 e a página no
-- espn.com.br). Os vídeos saem durante o jogo e até umas 5 horas depois do apito: o worker
-- volta aos jogos encerrados nas últimas 12 horas a cada 30 minutos. Ficam 90 dias.
--
-- ⚠️ Aplique ANTES de atualizar o worker e a API: os dois usam a coluna. O código antigo
-- funciona com ela.
--
-- Pode rodar de novo sem efeito.

ALTER TABLE espn_matches ADD COLUMN IF NOT EXISTS videos JSONB;
