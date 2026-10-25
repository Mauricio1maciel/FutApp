-- Resultado travado (matches.result_locked): o resultado foi decidido fora de campo (W.O.,
-- decisão do tribunal) e a gravação da football-data não mexe nele. Sem a trava, ela
-- regravaria o jogo como "cancelado" a cada volta.
--
-- Primeiro caso: Independiente Medellín x Flamengo (Libertadores 2026, grupo A, 07/05),
-- interrompido por confusão na arquibancada. A CONMEBOL deu 3 x 0 ao Flamengo em 21/05/2026;
-- a football-data ainda o marca como cancelado, e a tabela do grupo ficava sem esses pontos.
--
-- Para travar outro jogo: UPDATE matches SET status = 'FINISHED', home_score = .., away_score = ..,
-- winner = '..', result_locked = true WHERE id = ..;
--
-- ⚠️ Aplique ANTES de atualizar o worker e a API (os dois leem a coluna). Com o worker
-- antigo, o jogo volta a "cancelado" até a atualização.
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE matches ADD COLUMN IF NOT EXISTS result_locked BOOLEAN NOT NULL DEFAULT false;

UPDATE matches m
SET status = 'FINISHED', home_score = 0, away_score = 3, winner = 'AWAY_TEAM', result_locked = true
FROM teams th, teams ta
WHERE th.api_id = m.api_home_team_id AND ta.api_id = m.api_away_team_id
  AND m.league = 'CLI' AND m.season = '2026'
  AND th.name ILIKE '%Medell%' AND ta.name ILIKE '%Flamengo%'
  AND m.match_date::date = '2026-05-08';

COMMIT;
