-- Seguir uma liga: avisos de todos os jogos dela, além de jogos e times (migration 009).
--
-- push_match_has_followers mantém a mesma assinatura: o trigger trg_matches_push_outbox
-- e o aviso de escalação passam a considerar quem segue a liga sem nenhuma mudança.

BEGIN;

CREATE TABLE IF NOT EXISTS league_subscriptions (
    device_id  TEXT        NOT NULL,
    league     VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (device_id, league),
    CONSTRAINT league_subscriptions_device_fkey FOREIGN KEY (device_id)
        REFERENCES push_devices (device_id) ON DELETE CASCADE,
    CONSTRAINT league_subscriptions_league_fkey FOREIGN KEY (league)
        REFERENCES leagues (code_api) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_league_subscriptions_league ON league_subscriptions (league);

CREATE OR REPLACE FUNCTION public.push_match_has_followers(p_match_id INTEGER, p_home BIGINT, p_away BIGINT)
RETURNS BOOLEAN
LANGUAGE sql STABLE
AS $$
    SELECT EXISTS (SELECT 1 FROM match_subscriptions WHERE match_id = p_match_id)
        OR EXISTS (SELECT 1 FROM team_subscriptions WHERE team_api_id IN (p_home, p_away))
        OR EXISTS (SELECT 1 FROM league_subscriptions ls
                   JOIN matches m ON m.league = ls.league
                   WHERE m.id = p_match_id)
$$;

-- ========== SEGURANÇA (mesmo padrão das migrations 004 e 009) ==========

ALTER TABLE league_subscriptions ENABLE ROW LEVEL SECURITY;
REVOKE EXECUTE ON FUNCTION push_match_has_followers(INTEGER, BIGINT, BIGINT) FROM PUBLIC;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON league_subscriptions FROM anon, authenticated;
    END IF;
END $$;

COMMIT;
