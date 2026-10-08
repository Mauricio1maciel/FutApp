-- Corrige temporadas da Libertadores e apaga jogos que não existem.
--
-- 1. Ligas de ano-calendário (CLI, BSA, WC...): a temporada é o ano do jogo.
--    A Libertadores 2026 estava espalhada em "2025-2026", "2026" e "2026-2027"
--    (gravada quando o formato ainda era o europeu, e o SaveMatch não atualizava a
--    temporada de um jogo existente). Por isso a classificação da CLI saía vazia.
-- 2. Rascunhos antigos da CLI que a football-data já trocou por outros IDs
--    (times 0, não aparecem mais no /competitions/CLI/matches).
-- 3. Jogos intrusos gravados com IDs da ESPN (trigger antigo, antes da migration 001):
--    cópias de jogos que já existem com o ID da football-data.
-- 4. Classificação da CLI: era uma tabela única com 4 times da temporada errada.
--    O worker recalcula por grupos (fase de grupos) na próxima volta.

BEGIN;

-- 1
UPDATE matches m
SET season = EXTRACT(YEAR FROM m.match_date)::INT::TEXT
FROM leagues l
WHERE l.code_api = m.league
  AND l.season_format = 'calendar'
  AND m.match_date IS NOT NULL
  AND m.season IS DISTINCT FROM EXTRACT(YEAR FROM m.match_date)::INT::TEXT;

-- 2
DELETE FROM matches
WHERE league = 'CLI' AND id_event IN (557180, 557181, 557184, 557185)
  AND api_home_team_id = 0 AND api_away_team_id = 0;

-- 3 (PL: Burnley x Leeds 18/10/2025 e Forest x Tottenham 14/12/2025;
--    WC: semifinal França x Espanha e final Espanha x Argentina)
DELETE FROM matches m
WHERE (m.league, m.id_event) IN (('PL', 740667), ('PL', 740755), ('WC', 760514), ('WC', 760517))
  AND NOT EXISTS (SELECT 1 FROM teams t WHERE t.api_id = m.api_home_team_id);

-- 4
DELETE FROM standings WHERE league = 'CLI';

COMMIT;
