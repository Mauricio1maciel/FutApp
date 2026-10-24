-- 1. Avisos novos, a partir dos lances que a ESPN já manda e o worker já grava:
--    - penalty (inscrição): pênalti marcado pelo VAR, perdido, defendido, na trave e o
--      começo da disputa de pênaltis. Na fila: penalty_awarded, penalty_missed,
--      penalty_saved, penalty_woodwork e shootout. As cobranças da disputa não avisam uma
--      a uma (o "Fim de jogo" traz o placar dos pênaltis);
--    - halftime: o intervalo, com o placar.
--    O já avisado de cada jogo fica em match_push_state, como os cartões vermelhos.
-- 2. Lance a lance: a narração da ESPN em português fica em espn_matches.commentary (os
--    últimos 30 dias; o worker apaga as mais antigas).
--
-- ⚠️ Aplique ANTES de atualizar o worker: o novo grava a narração e os avisos novos. O
-- worker antigo funciona com esta migration aplicada.
--
-- Pode rodar de novo sem efeito.

BEGIN;

DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['match_subscriptions', 'team_subscriptions', 'league_subscriptions'] LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', t, t || '_kinds_check');
        EXECUTE format(
            'ALTER TABLE %I ADD CONSTRAINT %I CHECK (kinds IS NULL OR (cardinality(kinds) > 0
             AND kinds <@ ARRAY[''lineup'', ''start'', ''goal'', ''penalty'', ''red_card'', ''halftime'', ''end'']))',
            t, t || '_kinds_check');
    END LOOP;
END $$;

ALTER TABLE push_outbox DROP CONSTRAINT IF EXISTS push_outbox_kind_check;
ALTER TABLE push_outbox ADD CONSTRAINT push_outbox_kind_check
    CHECK (kind IN ('start', 'goal', 'goal_cancelled', 'end', 'lineup', 'red_card', 'halftime',
                    'penalty_awarded', 'penalty_missed', 'penalty_saved', 'penalty_woodwork', 'shootout'));

ALTER TABLE match_push_state ADD COLUMN IF NOT EXISTS penalties_notified INTEGER NOT NULL DEFAULT 0;
ALTER TABLE match_push_state ADD COLUMN IF NOT EXISTS halftime_notified BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE espn_matches ADD COLUMN IF NOT EXISTS commentary JSONB;

COMMIT;
