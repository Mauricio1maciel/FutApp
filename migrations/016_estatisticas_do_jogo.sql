-- Estatísticas do jogo (posse, finalizações, escanteios, faltas, cartões, passes...).
-- O worker já baixava o detalhe do jogo na ESPN, que traz 28 estatísticas por time, mas
-- não guardava. Ficam todas aqui, cruas ({"home": {...}, "away": {...}}); a API devolve
-- uma lista escolhida, em português e em ordem (models.MatchStatList).
ALTER TABLE espn_matches ADD COLUMN IF NOT EXISTS statistics JSONB;
