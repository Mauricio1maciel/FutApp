-- Corrige o trigger que copia os jogos de espn_matches para matches.
--
-- Problemas da versão anterior:
--   1. Jogo ainda não iniciado na ESPN ('pre') sobrescrevia o status 'TIMED' da
--      football-data por 'NOT_STARTED', quebrando o GetCurrentRound (que busca 'TIMED').
--   2. Jogos adiados/cancelados pela football-data podiam virar 'FINISHED' com placar
--      vazio (contado como 0x0 na classificação) quando a ESPN mandava 'post'.
--
-- Regras novas:
--   - UNL: continua criando/atualizando o jogo; 'pre' vira 'TIMED' (mesmo padrão da football-data).
--   - Demais ligas: só atualiza jogos ao vivo ou encerrados, só com placar preenchido,
--     e nunca mexe em jogos que a football-data marcou como adiados/cancelados.

CREATE OR REPLACE FUNCTION public.trg_sync_espn_to_matches()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    real_home_id BIGINT;
    real_away_id BIGINT;
    normalized_status VARCHAR(50);
BEGIN
    -- 1. Pega o ID oficial da football-data (se existir), senão usa o da ESPN como fallback
    SELECT api_id INTO real_home_id FROM teams WHERE espn_team_id = NEW.espn_home_team_id LIMIT 1;
    IF real_home_id IS NULL THEN real_home_id := NEW.espn_home_team_id; END IF;

    SELECT api_id INTO real_away_id FROM teams WHERE espn_team_id = NEW.espn_away_team_id LIMIT 1;
    IF real_away_id IS NULL THEN real_away_id := NEW.espn_away_team_id; END IF;

    -- 2. Traduz o status da ESPN para o padrão da football-data
    IF NEW.status = 'post' THEN
        normalized_status := 'FINISHED';
    ELSIF NEW.status = 'in' THEN
        normalized_status := 'IN_PLAY';
    ELSE
        normalized_status := 'TIMED';
    END IF;

    -- 3. Liga das Nações (UNL): a football-data não cobre, então o jogo é criado a partir da ESPN
    IF NEW.league = 'UNL' THEN
        INSERT INTO matches (
            id_event, league, season, match_date, status,
            home_score, away_score,
            api_home_team_id, api_away_team_id,
            group_name, stage, round
        )
        VALUES (
            NEW.espn_match_id, NEW.league, NEW.season, NEW.match_date, normalized_status,
            CAST(NULLIF(NEW.home_score, '') AS INTEGER),
            CAST(NULLIF(NEW.away_score, '') AS INTEGER),
            real_home_id, real_away_id,
            NEW.group_name, NEW.stage, 1
        )
        ON CONFLICT (id_event) DO UPDATE
        SET status = EXCLUDED.status,
            home_score = EXCLUDED.home_score,
            away_score = EXCLUDED.away_score,
            match_date = EXCLUDED.match_date,
            season = EXCLUDED.season,
            league = EXCLUDED.league,
            api_home_team_id = EXCLUDED.api_home_team_id,
            api_away_team_id = EXCLUDED.api_away_team_id,
            group_name = COALESCE(NULLIF(EXCLUDED.group_name, ''), matches.group_name),
            stage = COALESCE(NULLIF(EXCLUDED.stage, ''), matches.stage);

    -- 4. Demais ligas: o jogo vem da football-data. A ESPN só antecipa placar ao vivo/final.
    ELSIF NEW.status IN ('in', 'post')
          AND NULLIF(NEW.home_score, '') IS NOT NULL
          AND NULLIF(NEW.away_score, '') IS NOT NULL THEN
        UPDATE matches m
        SET status = normalized_status,
            home_score = CAST(NEW.home_score AS INTEGER),
            away_score = CAST(NEW.away_score AS INTEGER)
        WHERE m.api_home_team_id = real_home_id
          AND m.api_away_team_id = real_away_id
          AND m.match_date::DATE = NEW.match_date::DATE
          AND m.status NOT IN ('POSTPONED', 'SUSPENDED', 'CANCELLED', 'CANCELED');
    END IF;

    RETURN NEW;
END;
$function$;

-- Ajusta o único jogo da UNL que ficou com o status antigo
UPDATE matches SET status = 'TIMED' WHERE league = 'UNL' AND status = 'NOT_STARTED';
