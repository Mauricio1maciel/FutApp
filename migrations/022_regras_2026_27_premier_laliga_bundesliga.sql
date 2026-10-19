-- Regras de classificação próprias da temporada 2026-27 para Premier League (PL),
-- La Liga (PD) e Bundesliga (BL1). Até aqui elas usavam as de 2025-26 (a consulta pega a
-- regra mais recente até a temporada pedida): o resultado era o mesmo, mas uma mudança
-- na regra de 2025-26 mexeria sem querer na tabela de 2026-27.
--
-- Vagas (as mesmas de 2025-26; vagas extras, como a 5ª da Champions por desempenho do
-- país na Europa ou a do campeão da copa nacional, se decidem durante a temporada):
--   PL : 4 Champions, 1 Liga Europa, 3 rebaixados
--   PD : 4 Champions, 1 Liga Europa, 1 Conference, 3 rebaixados
--   BL1: 4 Champions, 1 Liga Europa, 1 Conference, 2 rebaixados (o 16º joga playoff)
--
-- Também completa o desempate da Premier League 2026-27, que estava sem o critério 1
-- (pontos). O código já punha os pontos na frente quando faltavam, então a tabela não
-- mudava; o cadastro é que ficava incompleto.
--
-- Pode rodar de novo sem efeito.

BEGIN;

INSERT INTO competition_rules (league, season, libertadores, pre_libertadores, sul_americana, rebaixamento)
SELECT v.league, '2026-2027', v.lib, v.pre, v.sul, v.reb
FROM (VALUES ('PL', 4, 1, 0, 3), ('PD', 4, 1, 1, 3), ('BL1', 4, 1, 1, 2))
     AS v(league, lib, pre, sul, reb)
WHERE NOT EXISTS (SELECT 1 FROM competition_rules r WHERE r.league = v.league AND r.season = '2026-2027');

INSERT INTO competition_tiebreakers (league, season, priority, criterion)
SELECT 'PL', '2026-2027', 1, 'points'
WHERE NOT EXISTS (SELECT 1 FROM competition_tiebreakers
                  WHERE league = 'PL' AND season = '2026-2027' AND priority = 1);

COMMIT;
