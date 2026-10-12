-- Em confronto ainda indefinido, a ESPN põe times de mentira chamados "TBD Home" /
-- "TBD Away". Na carga da Sul-Americana, o cadastro automático registrou os dois como
-- times (na final, que ainda não tem finalistas). O código agora trata esses nomes como
-- confronto indefinido (ID 0, a convenção da football-data); aqui saem os já gravados.
--
-- Só toca em times cadastrados pela ESPN (api_id >= 1.000.000.000) com nome "TBD...".
-- Pode rodar de novo sem efeito.

BEGIN;

CREATE TEMP TABLE tbd ON COMMIT DROP AS
SELECT api_id, espn_team_id FROM teams
WHERE api_id >= 1000000000 AND upper(name) LIKE 'TBD%';

-- O jogo fica "a definir" (0) até a ESPN mandar os times de verdade
UPDATE espn_matches SET espn_home_team_id = 0 WHERE espn_home_team_id IN (SELECT espn_team_id FROM tbd);
UPDATE espn_matches SET espn_away_team_id = 0 WHERE espn_away_team_id IN (SELECT espn_team_id FROM tbd);
UPDATE matches SET api_home_team_id = 0 WHERE api_home_team_id IN (SELECT api_id FROM tbd);
UPDATE matches SET api_away_team_id = 0 WHERE api_away_team_id IN (SELECT api_id FROM tbd);

DELETE FROM standings    WHERE team_id     IN (SELECT api_id FROM tbd);
DELETE FROM team_leagues WHERE team_api_id IN (SELECT api_id FROM tbd);
DELETE FROM teams        WHERE api_id      IN (SELECT api_id FROM tbd);

COMMIT;
