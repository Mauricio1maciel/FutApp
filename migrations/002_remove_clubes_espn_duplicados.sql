-- Remove clubes que o SaveFullMatchHistory cadastrou por engano com o ID da ESPN como api_id
-- (antes ele fazia isso para jogos de qualquer liga, agora só para a UNL).
--
-- Esses registros "sequestram" o espn_team_id (UNIQUE) e impedem o SyncESPNTeamLinks de
-- vincular o time verdadeiro da football-data:
--   99   Málaga        -> football-data 84   Málaga CF
--   373  Ipswich Town  -> football-data 349  Ipswich Town FC
--   388  Coventry City -> football-data 1076 Coventry City FC
--
-- Travas de segurança: só apaga se o registro for auto-criado (api_id = espn_team_id),
-- não estiver em nenhuma liga da football-data e não for usado por jogos da UNL.
-- O Genoa (3263) fica de fora até os times da Serie A (SA) serem carregados da football-data.

DELETE FROM teams t
WHERE t.api_id IN (99, 373, 388)
  AND t.api_id = t.espn_team_id
  AND NOT EXISTS (SELECT 1 FROM team_leagues tl WHERE tl.team_api_id = t.api_id)
  AND NOT EXISTS (
      SELECT 1 FROM matches m
      WHERE m.league = 'UNL' AND (m.api_home_team_id = t.api_id OR m.api_away_team_id = t.api_id)
  );

-- Linha sem nome criada a partir de jogos com time ainda indefinido (ID 0)
DELETE FROM teams WHERE api_id = 0 AND COALESCE(name, '') = '';
