-- 1. Erro de digitação no nome da zona da Champions (PL, PD e BL1):
--    champions_leugue → champions_league. O app compara só o trecho "champions",
--    então funciona igual antes e depois.
--
-- 2. Genoa: um registro "Genoa" criado a partir da ESPN (api_id 3263, sem liga e sem
--    jogos) ocupava o ID da ESPN do Genoa (3263, que é único na tabela). O "Genoa CFC"
--    da football-data (api_id 107, Serie A) nunca conseguia se vincular à ESPN e ficava
--    sem placar ao vivo, escalação, eventos e notificações. O ID passa para o 107 e o
--    registro fantasma sai.
--
-- Pode rodar de novo sem efeito: cada passo confere o estado antes de mudar.

BEGIN;

-- 1
UPDATE competition_zones SET zone_name = 'champions_league' WHERE zone_name = 'champions_leugue';
UPDATE standings SET zone = 'champions_league' WHERE zone = 'champions_leugue';

-- 2: libera o ID da ESPN, passa para o Genoa CFC e apaga o fantasma (só se ele
--    continuar sem liga e sem jogos)
UPDATE teams SET espn_team_id = NULL
WHERE api_id = 3263 AND espn_team_id = 3263
  AND NOT EXISTS (SELECT 1 FROM team_leagues WHERE team_api_id = 3263)
  AND NOT EXISTS (SELECT 1 FROM matches WHERE 3263 IN (api_home_team_id, api_away_team_id));

UPDATE teams SET espn_team_id = 3263
WHERE api_id = 107 AND espn_team_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM teams WHERE espn_team_id = 3263);

DELETE FROM teams
WHERE api_id = 3263 AND espn_team_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM team_leagues WHERE team_api_id = 3263)
  AND NOT EXISTS (SELECT 1 FROM matches WHERE 3263 IN (api_home_team_id, api_away_team_id));

COMMIT;
