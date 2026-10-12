-- Ligas que vêm só da ESPN (a football-data não cobre no plano grátis): Liga das Nações
-- (UNL) e Copa Sul-Americana (CSU).
--
-- Até aqui a UNL era um caso especial escrito à mão ('UNL' no trigger e no Go). Agora a
-- marca fica na tabela leagues (espn_only), e o trigger e o código consultam a marca:
-- para trazer outra liga da ESPN, basta marcá-la.
--
-- Para essas ligas, o trigger cria o jogo em matches a partir da espn_matches. Para as
-- demais, continua só atualizando placar e status dos jogos que vieram da football-data.

BEGIN;

ALTER TABLE leagues ADD COLUMN IF NOT EXISTS espn_only BOOLEAN NOT NULL DEFAULT false;
UPDATE leagues SET espn_only = true WHERE code_api IN ('UNL', 'CSU');

CREATE OR REPLACE FUNCTION public.trg_sync_espn_to_matches()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    real_home_id BIGINT;
    real_away_id BIGINT;
    normalized_status VARCHAR(50);
BEGIN
    -- 1. Pega o api_id do time dono do ID da ESPN; senão usa a faixa das seleções só-ESPN
    SELECT api_id INTO real_home_id FROM teams WHERE espn_team_id = NEW.espn_home_team_id LIMIT 1;
    IF real_home_id IS NULL THEN real_home_id := CASE WHEN COALESCE(NEW.espn_home_team_id, 0) = 0 THEN NEW.espn_home_team_id ELSE 1000000000 + NEW.espn_home_team_id END; END IF;

    SELECT api_id INTO real_away_id FROM teams WHERE espn_team_id = NEW.espn_away_team_id LIMIT 1;
    IF real_away_id IS NULL THEN real_away_id := CASE WHEN COALESCE(NEW.espn_away_team_id, 0) = 0 THEN NEW.espn_away_team_id ELSE 1000000000 + NEW.espn_away_team_id END; END IF;

    -- 2. Traduz o status da ESPN para o padrão da football-data
    IF NEW.status = 'post' THEN
        normalized_status := 'FINISHED';
    ELSIF NEW.status = 'in' THEN
        normalized_status := 'IN_PLAY';
    ELSE
        normalized_status := 'TIMED';
    END IF;

    -- 3. Ligas só da ESPN (leagues.espn_only, ex: UNL e CSU): a football-data não cobre,
    --    então o jogo é criado a partir da ESPN
    IF EXISTS (SELECT 1 FROM leagues WHERE code_api = NEW.league AND espn_only) THEN
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

COMMIT;
