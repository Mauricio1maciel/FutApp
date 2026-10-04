-- Vincula as seleções da UNL à tabela team_leagues.
--
-- A busca (/search) e os detalhes (/details) só encontram times que estão em
-- team_leagues. As seleções cadastradas a partir da ESPN nunca eram vinculadas,
-- então não apareciam no app. O SaveFullMatchHistory passa a vincular os novos
-- times. Este arquivo preenche os que já existem, a partir dos jogos da UNL.

INSERT INTO team_leagues (team_api_id, league, season)
SELECT DISTINCT t.api_id, 'UNL', m.season
FROM matches m
JOIN teams t ON t.api_id IN (m.api_home_team_id, m.api_away_team_id)
WHERE m.league = 'UNL'
  AND COALESCE(m.season, '') <> ''
ON CONFLICT DO NOTHING;
