-- Seleções da Liga das Nações (UNL) ganham uma faixa de IDs própria.
--
-- Problema: as seleções que só existem na ESPN eram gravadas com api_id = ID da ESPN.
-- Esses números colidem com os IDs da football-data: quando o worker carregou os
-- times da Serie A, o SaveTeam (upsert em api_id) sobrescreveu três seleções:
--   470 Islândia         -> Frosinone
--   471 Polônia          -> Sassuolo   (o "Bósnia x Sassuolo" era Bósnia x Polônia)
--   586 Irlanda do Norte -> Torino
-- e os clubes ainda herdaram o espn_team_id das seleções.
--
-- Regra nova: seleção só-ESPN usa api_id = 1.000.000.000 + ID da ESPN.
-- A football-data nunca chega nessa faixa, então não há mais colisão.
-- Seleções que também estão na Copa (WC) continuam com o ID da football-data.

BEGIN;

-- 1. Seleções só-ESPN ainda intactas: api_id = espn_team_id e nenhuma liga além da UNL
CREATE TEMP TABLE remap ON COMMIT DROP AS
SELECT t.api_id AS old_id, t.api_id + 1000000000 AS new_id
FROM teams t
WHERE t.api_id = t.espn_team_id
  AND t.api_id < 1000000000
  AND EXISTS (SELECT 1 FROM team_leagues WHERE team_api_id = t.api_id AND league = 'UNL')
  AND NOT EXISTS (SELECT 1 FROM team_leagues WHERE team_api_id = t.api_id AND league <> 'UNL');

UPDATE teams SET api_id = r.new_id FROM remap r WHERE teams.api_id = r.old_id;

-- 2. Clubes que tomaram o lugar de seleções: devolvem o espn_team_id (o vínculo
--    diário da ESPN acha o ID certo do clube depois) e as seleções são recriadas
UPDATE teams SET espn_team_id = NULL WHERE api_id IN (470, 471, 586) AND espn_team_id = api_id;

INSERT INTO teams (api_id, espn_team_id, name, crest_url) VALUES
    (1000000470, 470, 'Iceland',          'https://a.espncdn.com/i/teamlogos/countries/500/isl.png'),
    (1000000471, 471, 'Poland',           'https://a.espncdn.com/i/teamlogos/countries/500/pol.png'),
    (1000000586, 586, 'Northern Ireland', 'https://a.espncdn.com/i/teamlogos/countries/500/nir.png')
ON CONFLICT DO NOTHING;

INSERT INTO remap VALUES (470, 1000000470), (471, 1000000471), (586, 1000000586);

-- 3. Referências da UNL passam para os IDs novos (as outras ligas não são tocadas:
--    470, 471 e 586 continuam sendo Frosinone, Sassuolo e Torino na Serie A)
UPDATE team_leagues tl SET team_api_id = r.new_id
FROM remap r WHERE tl.league = 'UNL' AND tl.team_api_id = r.old_id;

UPDATE matches m SET api_home_team_id = r.new_id
FROM remap r WHERE m.league = 'UNL' AND m.api_home_team_id = r.old_id;

UPDATE matches m SET api_away_team_id = r.new_id
FROM remap r WHERE m.league = 'UNL' AND m.api_away_team_id = r.old_id;

UPDATE standings s SET team_id = r.new_id
FROM remap r WHERE s.league = 'UNL' AND s.team_id = r.old_id;

-- 4. Trigger: o fallback (time ainda não cadastrado) também usa a faixa nova
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

COMMIT;
