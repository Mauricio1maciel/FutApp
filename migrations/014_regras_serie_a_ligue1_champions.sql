-- Regras de classificação da Serie A (SA), Ligue 1 (FL1) e Champions (CL), 2026-27.
-- Sem elas, a tabela saía sem nenhuma zona e com o desempate padrão (pontos, saldo,
-- gols pró).
--
-- Vagas (o que o app pinta):
--   SA  (20 times): 4 Champions, 1 Liga Europa, 1 Conference, 3 rebaixados
--   FL1 (18 times): 4 Champions (o 4º entra pela fase preliminar), 1 Liga Europa,
--                   1 Conference, 2 rebaixados. O 16º disputa playoff contra a Ligue 2 e
--                   não é destacado: o modelo de zonas não tem essa faixa.
--   CL  (36 times, fase de liga): 1º a 8º direto às oitavas, 9º a 24º playoffs,
--                   25º a 36º eliminados
--   Vagas extras mudam de uma temporada para outra (campeão da copa nacional, vaga a mais
--   pelo desempenho do país na Europa): quando mudar, ajuste competition_rules.
--
-- Desempate (o código entende points, wins, goal_diff, goals_for e head_to_head):
--   SA : pontos, confronto direto, saldo, gols pró
--   FL1: pontos, saldo, gols pró, confronto direto
--   CL : pontos, saldo, gols pró, vitórias (o regulamento também usa gols fora de casa,
--        que o código ainda não calcula; sem confronto direto, porque na fase de liga
--        cada time enfrenta adversários diferentes)
--
-- Pode rodar de novo sem duplicar: cada INSERT confere se a linha já existe.

BEGIN;

INSERT INTO competition_rules (league, season, libertadores, pre_libertadores, sul_americana, rebaixamento)
SELECT v.league, v.season, v.lib, v.pre, v.sul, v.reb
FROM (VALUES
    ('SA',  '2026-2027', 4,  1, 1, 3),
    ('FL1', '2026-2027', 4,  1, 1, 2),
    ('CL',  '2026-2027', 8, 16, 0, 12)
) AS v(league, season, lib, pre, sul, reb)
WHERE NOT EXISTS (SELECT 1 FROM competition_rules r WHERE r.league = v.league AND r.season = v.season);

-- Os nomes da CL seguem os da classificação por grupos das copas: o app pinta
-- "oitavas" de amarelo, "pré" de laranja e "elimin" de vermelho
INSERT INTO competition_zones (league, zone_key, zone_name, priority)
SELECT v.league, v.zone_key, v.zone_name, v.priority
FROM (VALUES
    ('SA',  'lib', 'champions_league',       1),
    ('SA',  'pre', 'europa_league',          2),
    ('SA',  'sul', 'conference_league',      3),
    ('SA',  'reb', 'rebaixamento',           4),
    ('FL1', 'lib', 'champions_league',       1),
    ('FL1', 'pre', 'europa_league',          2),
    ('FL1', 'sul', 'conference_league',      3),
    ('FL1', 'reb', 'rebaixamento',           4),
    ('CL',  'lib', 'Classificado - Oitavas', 1),
    ('CL',  'pre', 'Playoffs (pré-oitavas)', 2),
    ('CL',  'reb', 'Eliminado',              3)
) AS v(league, zone_key, zone_name, priority)
WHERE NOT EXISTS (SELECT 1 FROM competition_zones z WHERE z.league = v.league AND z.zone_key = v.zone_key);

INSERT INTO competition_tiebreakers (league, season, priority, criterion)
SELECT v.league, v.season, v.priority, v.criterion
FROM (VALUES
    ('SA',  '2026-2027', 1, 'points'),
    ('SA',  '2026-2027', 2, 'head_to_head'),
    ('SA',  '2026-2027', 3, 'goal_diff'),
    ('SA',  '2026-2027', 4, 'goals_for'),
    ('FL1', '2026-2027', 1, 'points'),
    ('FL1', '2026-2027', 2, 'goal_diff'),
    ('FL1', '2026-2027', 3, 'goals_for'),
    ('FL1', '2026-2027', 4, 'head_to_head'),
    ('CL',  '2026-2027', 1, 'points'),
    ('CL',  '2026-2027', 2, 'goal_diff'),
    ('CL',  '2026-2027', 3, 'goals_for'),
    ('CL',  '2026-2027', 4, 'wins')
) AS v(league, season, priority, criterion)
WHERE NOT EXISTS (SELECT 1 FROM competition_tiebreakers t
                  WHERE t.league = v.league AND t.season = v.season AND t.priority = v.priority);

COMMIT;
