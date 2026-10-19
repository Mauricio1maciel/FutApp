-- Série B 2026: regra nova de acesso (aprovada pelo Conselho Técnico da CBF em 05/02/2026).
--   1º e 2º: sobem direto (zona "acesso")
--   3º a 6º: playoffs de acesso, 3º x 6º e 4º x 5º em ida e volta (21 e 28/11), valendo
--            as outras 2 vagas (zona "playoff"); empate no agregado: avança o de melhor
--            campanha, sem pênaltis
--   17º a 20º: rebaixados (não mudou)
-- O nome "playoff" não pode conter "acesso": o app pinta de verde tudo que tem "acesso".
--
-- Os jogos dos playoffs vêm da ESPN com a mesma fase dos pontos corridos (o nome da
-- temporada, REGULAR_SEASON aqui). O recálculo da classificação marca os jogos além do
-- turno e returno como PLAYOFFS; o gatilho abaixo deixa de apagar essa fase a cada
-- atualização da ESPN.
--
-- Pode rodar de novo sem efeito.

BEGIN;

UPDATE competition_rules SET libertadores = 2, pre_libertadores = 4
WHERE league = 'BSB' AND season = '2026' AND (libertadores, pre_libertadores) IS DISTINCT FROM (2, 4);

INSERT INTO competition_zones (league, zone_key, zone_name, priority)
SELECT 'BSB', 'pre', 'playoff', 2
WHERE NOT EXISTS (SELECT 1 FROM competition_zones WHERE league = 'BSB' AND zone_key = 'pre');
UPDATE competition_zones SET priority = 3 WHERE league = 'BSB' AND zone_key = 'reb' AND priority <> 3;

CREATE OR REPLACE FUNCTION public.trg_sync_espn_to_matches()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    real_home_id BIGINT;
    real_away_id BIGINT;
    normalized_status VARCHAR(50);
    gols_casa INTEGER;
    gols_fora INTEGER;
    vencedor VARCHAR(20);
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

    -- 3. Ligas só da ESPN (leagues.espn_only, ex: UNL, CSU e CDB): a football-data não
    --    cobre, então o jogo é criado a partir da ESPN
    IF EXISTS (SELECT 1 FROM leagues WHERE code_api = NEW.league AND espn_only) THEN
        gols_casa := CAST(NULLIF(NEW.home_score, '') AS INTEGER);
        gols_fora := CAST(NULLIF(NEW.away_score, '') AS INTEGER);

        -- Vencedor, como na football-data: com disputa de pênaltis, é quem venceu a
        -- disputa (quem avançou), mesmo que tenha perdido a partida (ex: jogo de volta
        -- 2 x 1 com agregado empatado); sem disputa, quem fez mais gols
        vencedor := CASE
            WHEN normalized_status <> 'FINISHED' OR gols_casa IS NULL OR gols_fora IS NULL THEN ''
            WHEN NEW.home_penalty > NEW.away_penalty THEN 'HOME_TEAM'
            WHEN NEW.home_penalty < NEW.away_penalty THEN 'AWAY_TEAM'
            WHEN gols_casa > gols_fora THEN 'HOME_TEAM'
            WHEN gols_casa < gols_fora THEN 'AWAY_TEAM'
            ELSE 'DRAW'
        END;

        INSERT INTO matches (
            id_event, league, season, match_date, status,
            home_score, away_score, home_penalty, away_penalty, winner,
            api_home_team_id, api_away_team_id,
            group_name, stage, round
        )
        VALUES (
            NEW.espn_match_id, NEW.league, NEW.season, NEW.match_date, normalized_status,
            gols_casa, gols_fora, NEW.home_penalty, NEW.away_penalty, vencedor,
            real_home_id, real_away_id,
            NEW.group_name, NEW.stage, 1
        )
        ON CONFLICT (id_event) DO UPDATE
        SET status = EXCLUDED.status,
            home_score = EXCLUDED.home_score,
            away_score = EXCLUDED.away_score,
            home_penalty = COALESCE(EXCLUDED.home_penalty, matches.home_penalty),
            away_penalty = COALESCE(EXCLUDED.away_penalty, matches.away_penalty),
            winner = EXCLUDED.winner,
            match_date = EXCLUDED.match_date,
            season = EXCLUDED.season,
            league = EXCLUDED.league,
            api_home_team_id = EXCLUDED.api_home_team_id,
            api_away_team_id = EXCLUDED.api_away_team_id,
            group_name = COALESCE(NULLIF(EXCLUDED.group_name, ''), matches.group_name),
            -- REGULAR_SEASON vindo da ESPN é só o nome da temporada (liga sem fases): ele
            -- preenche a fase vazia, mas não apaga uma fase calculada pelo recálculo (os
            -- playoffs da Série B). Fase de verdade da ESPN (ex: SEMI_FINALS) vale sempre
            stage = CASE
                WHEN COALESCE(EXCLUDED.stage, '') IN ('', 'REGULAR_SEASON')
                    THEN COALESCE(NULLIF(matches.stage, ''), NULLIF(EXCLUDED.stage, ''))
                ELSE EXCLUDED.stage
            END;

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
