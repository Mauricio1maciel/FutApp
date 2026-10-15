-- Brasileirão Série B (BSB), pela ESPN: a football-data não cobre no plano grátis.
-- A ESPN não informa a rodada; ela é calculada pelo calendário (services/rounds.go).
--
-- Regras de 2026: 20 times, os 4 primeiros sobem (zona "acesso"), os 4 últimos caem.
-- Desempate igual ao da Série A: pontos, vitórias, saldo, gols pró, confronto direto.
--
-- Pode rodar de novo sem efeito.

BEGIN;

INSERT INTO leagues (code_api, code_espn, name, logo_url, season_format, espn_only, search_terms)
SELECT 'BSB', 'bra.2', 'Brasileirão - Série B',
       'https://a.espncdn.com/i/leaguelogos/soccer/500/2299.png', 'calendar', true,
       'serie b brasileirao serie b segunda divisao brasileiro b acesso'
WHERE NOT EXISTS (SELECT 1 FROM leagues WHERE code_api = 'BSB');

INSERT INTO competition_rules (league, season, libertadores, pre_libertadores, sul_americana, rebaixamento)
SELECT 'BSB', '2026', 4, 0, 0, 4
WHERE NOT EXISTS (SELECT 1 FROM competition_rules WHERE league = 'BSB' AND season = '2026');

INSERT INTO competition_zones (league, zone_key, zone_name, priority)
SELECT v.league, v.zone_key, v.zone_name, v.priority
FROM (VALUES ('BSB', 'lib', 'acesso', 1), ('BSB', 'reb', 'rebaixamento', 2))
     AS v(league, zone_key, zone_name, priority)
WHERE NOT EXISTS (SELECT 1 FROM competition_zones z WHERE z.league = v.league AND z.zone_key = v.zone_key);

INSERT INTO competition_tiebreakers (league, season, priority, criterion)
SELECT v.league, v.season, v.priority, v.criterion
FROM (VALUES ('BSB', '2026', 1, 'points'), ('BSB', '2026', 2, 'wins'),
             ('BSB', '2026', 3, 'goal_diff'), ('BSB', '2026', 4, 'goals_for'),
             ('BSB', '2026', 5, 'head_to_head'))
     AS v(league, season, priority, criterion)
WHERE NOT EXISTS (SELECT 1 FROM competition_tiebreakers t
                  WHERE t.league = v.league AND t.season = v.season AND t.priority = v.priority);

COMMIT;
