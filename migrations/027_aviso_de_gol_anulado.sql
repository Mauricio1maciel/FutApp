-- Aviso de gol anulado (push_outbox.kind = 'goal_cancelled'), e o gol seguinte volta a
-- ser avisado.
--
-- Até aqui o aviso de gol só saía quando o placar passava do maior já avisado. Se o VAR
-- anulava um gol, o "GOL!" ficava no celular e, quando o time marcava de novo, o placar
-- só voltava ao número já avisado: o gol que valeu passava em branco.
--
-- Agora, quando o placar cai abaixo do já avisado, entra na fila um aviso de gol anulado,
-- que vai para quem recebe os avisos de gol. Ele espera alguns minutos antes de sair
-- (GetPendingPushes): se o placar voltar nesse meio-tempo, era oscilação da fonte, e o
-- aviso é retirado sem repetir o "GOL!". O fim de jogo também retira o aviso pendente,
-- porque o placar final já é o certo.
--
-- Junto com esta migration, o código deixa de aceitar que o placar ao vivo da
-- football-data (atrasado) seja gravado por cima do da ESPN.
--
-- ⚠️ Aplique DEPOIS de atualizar o worker: o worker antigo não conhece o aviso novo e o
-- mandaria sem texto, na hora. O código novo funciona sem esta migration (só não avisa
-- gol anulado).
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE push_outbox DROP CONSTRAINT IF EXISTS push_outbox_kind_check;
ALTER TABLE push_outbox ADD CONSTRAINT push_outbox_kind_check
    CHECK (kind IN ('start', 'goal', 'goal_cancelled', 'end', 'lineup', 'red_card'));

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
    anulado_casa  BOOLEAN;
    anulado_fora  BOOLEAN;
    pendente_casa BOOLEAN;
    pendente_fora BOOLEAN;
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

        -- Gol: o placar passou do já avisado. Depois do "Fim de jogo", mudança de placar é
        -- correção, não gol (o gol no último lance chega junto com o apito e é avisado
        -- antes do fim)
        gol_casa := COALESCE(NEW.home_score, 0) > st.home_notified;
        gol_fora := COALESCE(NEW.away_score, 0) > st.away_notified;

        -- O placar voltou antes de sair o aviso de gol anulado: era oscilação da fonte, não
        -- gol novo. Retira o aviso pendente e não repete o "GOL!"
        SELECT COALESCE(bool_or(goal_side IN ('home', 'both')), false),
               COALESCE(bool_or(goal_side IN ('away', 'both')), false)
          INTO pendente_casa, pendente_fora
          FROM push_outbox
         WHERE match_id = NEW.id AND kind = 'goal_cancelled' AND sent_at IS NULL;
        IF (gol_casa AND pendente_casa) OR (gol_fora AND pendente_fora) THEN
            DELETE FROM push_outbox
             WHERE match_id = NEW.id AND kind = 'goal_cancelled' AND sent_at IS NULL;
            gol_casa := gol_casa AND NOT pendente_casa;
            gol_fora := gol_fora AND NOT pendente_fora;
        END IF;

        IF (gol_casa OR gol_fora) AND NOT ja_acabou AND NOT st.ended THEN
            INSERT INTO push_outbox (match_id, kind, home_score, away_score, goal_side)
            VALUES (NEW.id, 'goal', COALESCE(NEW.home_score, 0), COALESCE(NEW.away_score, 0),
                    CASE WHEN gol_casa AND gol_fora THEN 'both' WHEN gol_casa THEN 'home' ELSE 'away' END);
        END IF;

        -- Gol anulado (VAR, ou correção da fonte): o placar caiu abaixo do já avisado
        anulado_casa := COALESCE(NEW.home_score, 0) < st.home_notified;
        anulado_fora := COALESCE(NEW.away_score, 0) < st.away_notified;
        IF (anulado_casa OR anulado_fora) AND NOT ja_acabou AND NOT st.ended THEN
            INSERT INTO push_outbox (match_id, kind, home_score, away_score, goal_side)
            VALUES (NEW.id, 'goal_cancelled', COALESCE(NEW.home_score, 0), COALESCE(NEW.away_score, 0),
                    CASE WHEN anulado_casa AND anulado_fora THEN 'both' WHEN anulado_casa THEN 'home' ELSE 'away' END);
        END IF;

        -- O avisado acompanha o placar, para cima e para baixo: depois de um gol anulado,
        -- o gol seguinte do time também é avisado
        st.home_notified := COALESCE(NEW.home_score, 0);
        st.away_notified := COALESCE(NEW.away_score, 0);

        -- Fim. O placar final já é o certo: um aviso de gol anulado pendente perde o sentido
        IF NEW.status = 'FINISHED' AND NOT st.ended THEN
            DELETE FROM push_outbox
             WHERE match_id = NEW.id AND kind = 'goal_cancelled' AND sent_at IS NULL;
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

REVOKE EXECUTE ON FUNCTION trg_matches_push_outbox() FROM PUBLIC;

COMMIT;
