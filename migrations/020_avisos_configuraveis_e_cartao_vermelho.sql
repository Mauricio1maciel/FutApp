-- 1. Tipos de aviso por inscrição: quem segue um time, uma liga ou um jogo escolhe o que
--    recebe (lineup, start, goal, red_card, end). NULL = todos, como era até aqui.
-- 2. Cartão vermelho como aviso novo: o worker lê os eventos da ESPN e anota um aviso por
--    cartão (push_outbox.kind = 'red_card', com o jogador e o minuto). O total já avisado
--    de cada jogo fica em match_push_state.reds_notified, para não repetir.
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE match_subscriptions  ADD COLUMN IF NOT EXISTS kinds TEXT[];
ALTER TABLE team_subscriptions   ADD COLUMN IF NOT EXISTS kinds TEXT[];
ALTER TABLE league_subscriptions ADD COLUMN IF NOT EXISTS kinds TEXT[];

-- Só tipos conhecidos, e pelo menos um (para não receber nada, deixa-se de seguir)
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY['match_subscriptions', 'team_subscriptions', 'league_subscriptions'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = t || '_kinds_check') THEN
            EXECUTE format(
                'ALTER TABLE %I ADD CONSTRAINT %I CHECK (kinds IS NULL OR (cardinality(kinds) > 0
                 AND kinds <@ ARRAY[''lineup'', ''start'', ''goal'', ''red_card'', ''end'']))',
                t, t || '_kinds_check');
        END IF;
    END LOOP;
END $$;

ALTER TABLE push_outbox DROP CONSTRAINT IF EXISTS push_outbox_kind_check;
ALTER TABLE push_outbox ADD CONSTRAINT push_outbox_kind_check
    CHECK (kind IN ('start', 'goal', 'end', 'lineup', 'red_card'));
ALTER TABLE push_outbox ADD COLUMN IF NOT EXISTS player TEXT NOT NULL DEFAULT '';
ALTER TABLE push_outbox ADD COLUMN IF NOT EXISTS minute TEXT NOT NULL DEFAULT '';

ALTER TABLE match_push_state ADD COLUMN IF NOT EXISTS reds_notified INTEGER NOT NULL DEFAULT 0;

COMMIT;
