-- Busca de ligas no /search.
--
-- search_terms: apelidos que o nome oficial não cobre ("brasileirão", "champions",
-- "campeonato inglês", "copa do mundo"...). Guardados sem acento e em minúsculas;
-- a busca compara com unaccent, então "brasileirao" e "brasileirão" acham o mesmo.

BEGIN;

ALTER TABLE leagues ADD COLUMN IF NOT EXISTS search_terms TEXT NOT NULL DEFAULT '';

UPDATE leagues SET search_terms = v.terms
FROM (VALUES
    ('PL',  'premier league epl inglaterra ingles campeonato ingles england'),
    ('PD',  'la liga laliga espanha espanhol campeonato espanhol spain'),
    ('CL',  'champions league ucl liga dos campeoes uefa'),
    ('BL1', 'bundesliga alemanha alemao campeonato alemao germany'),
    ('SA',  'serie a calcio italia italiano campeonato italiano italy'),
    ('FL1', 'ligue 1 franca frances campeonato frances france'),
    ('BSA', 'brasileirao serie a brasil campeonato brasileiro brazil'),
    ('CLI', 'libertadores copa libertadores conmebol'),
    ('CSU', 'sul-americana sulamericana sudamericana copa sul-americana conmebol'),
    ('WC',  'copa do mundo mundial world cup fifa'),
    ('UNL', 'liga das nacoes nations league uefa')
) AS v(code, terms)
WHERE leagues.code_api = v.code;

-- Erro de digitação no nome exibido no app
UPDATE leagues SET name = 'Campeonato Italiano - Série A' WHERE code_api = 'SA' AND name = 'Compeonato Italiano - Série A';

COMMIT;
