-- Contas de usuário: o que a pessoa segue (times, ligas, jogos e os tipos de aviso) fica
-- salvo na conta e vale em todos os aparelhos em que ela entrar.
--
-- - Cada inscrição pertence a um aparelho (convidado, device_id) OU a uma conta (user_id).
-- - push_devices.user_id: a conta em que o aparelho está logado. O aviso de uma inscrição
--   da conta vai para todos os aparelhos logados nela.
-- - users.display_name: o nome que a pessoa escolheu (opcional).
--
-- A chave primária (device_id, alvo) vira restrição única: a API que está no ar continua
-- gravando as inscrições de convidado do mesmo jeito (ON CONFLICT (device_id, alvo)), e as
-- da conta, com device_id vazio, não esbarram nela.
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';

ALTER TABLE push_devices ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_push_devices_user ON push_devices (user_id) WHERE user_id IS NOT NULL;

DO $$
DECLARE
    t TEXT;
    alvo TEXT;
BEGIN
    FOR t, alvo IN VALUES ('match_subscriptions', 'match_id'), ('team_subscriptions', 'team_api_id'),
                          ('league_subscriptions', 'league') LOOP
        EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users (id) ON DELETE CASCADE', t);
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I', t, t || '_pkey');
        EXECUTE format('ALTER TABLE %I ALTER COLUMN device_id DROP NOT NULL', t);
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = t || '_device_key') THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (device_id, %I)', t, t || '_device_key', alvo);
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = t || '_user_key') THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I UNIQUE (user_id, %I)', t, t || '_user_key', alvo);
        END IF;
        IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = t || '_owner_check') THEN
            EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK ((device_id IS NULL) <> (user_id IS NULL))',
                           t, t || '_owner_check');
        END IF;
    END LOOP;
END $$;

COMMIT;
