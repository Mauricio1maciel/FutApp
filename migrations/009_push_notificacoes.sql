-- Notificações push (Expo) para quem segue um jogo ou um time.
--
-- Fluxo:
--   1. O app registra o token de push do aparelho (push_devices) e o que ele segue:
--      jogos (match_subscriptions) e times (team_subscriptions, todos os jogos do time).
--   2. Quando o status ou o placar de um jogo muda, venha da ESPN ou da football-data,
--      o trigger trg_matches_push_outbox anota o aviso em push_outbox: início, gol e fim.
--      Só anota se alguém segue o jogo ou um dos times.
--   3. A escalação confirmada é anotada pelo Go (SaveFullMatchHistory), na mesma
--      transação que grava a escalação.
--   4. O worker lê push_outbox a cada 10s e manda os pushes pelo Expo.
--
-- match_push_state guarda o que já foi avisado em cada jogo. O placar de um jogo ao
-- vivo às vezes oscila: a football-data regrava um placar atrasado por cima do da ESPN.
-- Avisando só quando o placar passa do maior já avisado, o mesmo gol não é anunciado
-- duas vezes.

BEGIN;

-- ========== TABELAS ==========

CREATE TABLE IF NOT EXISTS push_devices (
    device_id  TEXT PRIMARY KEY,  -- do token de convidado (JWT)
    push_token TEXT NOT NULL,     -- ExponentPushToken[...]
    platform   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT push_devices_token_key UNIQUE (push_token)
);

CREATE TABLE IF NOT EXISTS match_subscriptions (
    device_id  TEXT    NOT NULL,
    match_id   INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (device_id, match_id),
    CONSTRAINT match_subscriptions_device_fkey FOREIGN KEY (device_id)
        REFERENCES push_devices (device_id) ON DELETE CASCADE,
    CONSTRAINT match_subscriptions_match_fkey FOREIGN KEY (match_id)
        REFERENCES matches (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_match_subscriptions_match ON match_subscriptions (match_id);

CREATE TABLE IF NOT EXISTS team_subscriptions (
    device_id   TEXT   NOT NULL,
    team_api_id BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (device_id, team_api_id),
    CONSTRAINT team_subscriptions_device_fkey FOREIGN KEY (device_id)
        REFERENCES push_devices (device_id) ON DELETE CASCADE,
    -- ON UPDATE CASCADE: se um time for renumerado (como as seleções na migration 006),
    -- quem o segue continua seguindo
    CONSTRAINT team_subscriptions_team_fkey FOREIGN KEY (team_api_id)
        REFERENCES teams (api_id) ON DELETE CASCADE ON UPDATE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_team_subscriptions_team ON team_subscriptions (team_api_id);

CREATE TABLE IF NOT EXISTS match_push_state (
    match_id      INTEGER PRIMARY KEY REFERENCES matches (id) ON DELETE CASCADE,
    home_notified INTEGER NOT NULL DEFAULT 0,  -- maior placar já avisado
    away_notified INTEGER NOT NULL DEFAULT 0,
    started       BOOLEAN NOT NULL DEFAULT false,
    ended         BOOLEAN NOT NULL DEFAULT false,
    lineup        BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS push_outbox (
    id         BIGSERIAL PRIMARY KEY,
    match_id   INTEGER NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('start', 'goal', 'end', 'lineup')),
    home_score INTEGER NOT NULL DEFAULT 0,
    away_score INTEGER NOT NULL DEFAULT 0,
    goal_side  TEXT    NOT NULL DEFAULT '',  -- gol: home | away | both
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at    TIMESTAMPTZ,                  -- NULL = ainda na fila
    result     TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_push_outbox_pending ON push_outbox (id) WHERE sent_at IS NULL;

-- ========== QUEM SEGUE ==========

CREATE OR REPLACE FUNCTION public.push_match_has_followers(p_match_id INTEGER, p_home BIGINT, p_away BIGINT)
RETURNS BOOLEAN
LANGUAGE sql STABLE
AS $$
    SELECT EXISTS (SELECT 1 FROM match_subscriptions WHERE match_id = p_match_id)
        OR EXISTS (SELECT 1 FROM team_subscriptions WHERE team_api_id IN (p_home, p_away))
$$;

-- ========== TRIGGER: INÍCIO, GOL E FIM ==========

CREATE OR REPLACE FUNCTION public.trg_matches_push_outbox()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    agora    TIMESTAMP := now() AT TIME ZONE 'UTC';  -- match_date é UTC, sem fuso
    rolando  CONSTANT TEXT[] := ARRAY['IN_PLAY', 'PAUSED', 'FINISHED'];
    st       match_push_state%ROWTYPE;
    gol_casa BOOLEAN;
    gol_fora BOOLEAN;
    ja_acabou BOOLEAN;
BEGIN
    -- Só jogos de agora: um jogo antigo completado depois (GetMissingMatches) não avisa
    IF NEW.match_date IS NULL
       OR NEW.match_date < agora - INTERVAL '4 hours'
       OR NEW.match_date > agora + INTERVAL '1 hour' THEN
        RETURN NULL;
    END IF;

    -- O trigger roda dentro da gravação do placar (ESPN e football-data): um erro aqui
    -- abortaria a transação e o placar deixaria de ser atualizado. O bloco com
    -- EXCEPTION desfaz só o aviso e deixa o placar seguir.
    BEGIN
        IF NOT push_match_has_followers(NEW.id, NEW.api_home_team_id, NEW.api_away_team_id) THEN
            RETURN NULL;
        END IF;

        -- Primeira mudança desde que alguém passou a seguir: parte do que já tinha
        -- acontecido, para não anunciar "começou" no meio do jogo nem gols antigos
        INSERT INTO match_push_state (match_id, home_notified, away_notified, started, ended)
        VALUES (NEW.id,
                CASE WHEN OLD.status = ANY (rolando) THEN COALESCE(OLD.home_score, 0) ELSE 0 END,
                CASE WHEN OLD.status = ANY (rolando) THEN COALESCE(OLD.away_score, 0) ELSE 0 END,
                COALESCE(OLD.status = ANY (rolando), false),
                COALESCE(OLD.status = 'FINISHED', false))
        ON CONFLICT (match_id) DO NOTHING;

        IF NOT (NEW.status = ANY (rolando)) THEN
            RETURN NULL;
        END IF;

        SELECT * INTO st FROM match_push_state WHERE match_id = NEW.id FOR UPDATE;

        -- Visto pela primeira vez já encerrado (o worker perdeu o jogo ao vivo): só o
        -- placar final, sem "começou" nem um aviso por gol
        ja_acabou := NOT st.started AND NEW.status = 'FINISHED';

        -- Início
        IF NOT st.started THEN
            IF NOT ja_acabou THEN
                INSERT INTO push_outbox (match_id, kind, home_score, away_score)
                VALUES (NEW.id, 'start', COALESCE(NEW.home_score, 0), COALESCE(NEW.away_score, 0));
            END IF;
            st.started := true;
        END IF;

        -- Gol: só quando o placar passa do maior já avisado. Depois do "Fim de jogo",
        -- mudança de placar é correção, não gol (o gol no último lance chega junto com
        -- o apito e é avisado antes do fim)
        gol_casa := COALESCE(NEW.home_score, 0) > st.home_notified;
        gol_fora := COALESCE(NEW.away_score, 0) > st.away_notified;
        IF (gol_casa OR gol_fora) AND NOT ja_acabou AND NOT st.ended THEN
            INSERT INTO push_outbox (match_id, kind, home_score, away_score, goal_side)
            VALUES (NEW.id, 'goal', COALESCE(NEW.home_score, 0), COALESCE(NEW.away_score, 0),
                    CASE WHEN gol_casa AND gol_fora THEN 'both' WHEN gol_casa THEN 'home' ELSE 'away' END);
        END IF;
        st.home_notified := GREATEST(st.home_notified, COALESCE(NEW.home_score, 0));
        st.away_notified := GREATEST(st.away_notified, COALESCE(NEW.away_score, 0));

        -- Fim
        IF NEW.status = 'FINISHED' AND NOT st.ended THEN
            INSERT INTO push_outbox (match_id, kind, home_score, away_score)
            VALUES (NEW.id, 'end', COALESCE(NEW.home_score, 0), COALESCE(NEW.away_score, 0));
            st.ended := true;
        END IF;

        UPDATE match_push_state
           SET home_notified = st.home_notified,
               away_notified = st.away_notified,
               started       = st.started,
               ended         = st.ended
         WHERE match_id = NEW.id;
    EXCEPTION WHEN OTHERS THEN
        RAISE WARNING 'push: aviso do jogo % não foi anotado: %', NEW.id, SQLERRM;
    END;

    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_matches_push_outbox ON matches;
CREATE TRIGGER trg_matches_push_outbox
    AFTER UPDATE OF status, home_score, away_score ON matches
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status
       OR OLD.home_score IS DISTINCT FROM NEW.home_score
       OR OLD.away_score IS DISTINCT FROM NEW.away_score)
    EXECUTE FUNCTION trg_matches_push_outbox();

-- ========== SEGURANÇA (mesmo padrão da migration 004) ==========

ALTER TABLE push_devices        ENABLE ROW LEVEL SECURITY;
ALTER TABLE match_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_subscriptions  ENABLE ROW LEVEL SECURITY;
ALTER TABLE match_push_state    ENABLE ROW LEVEL SECURITY;
ALTER TABLE push_outbox         ENABLE ROW LEVEL SECURITY;

REVOKE EXECUTE ON FUNCTION push_match_has_followers(INTEGER, BIGINT, BIGINT) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION trg_matches_push_outbox() FROM PUBLIC;

-- Os papéis do Supabase não existem num Postgres comum (ex: teste no Docker)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON push_devices, match_subscriptions, team_subscriptions,
                      match_push_state, push_outbox FROM anon, authenticated;
        REVOKE ALL ON SEQUENCE push_outbox_id_seq FROM anon, authenticated;
    END IF;
END $$;

COMMIT;
