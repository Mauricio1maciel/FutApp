-- Segurança (RLS) e índices.
--
-- 1. SEGURANÇA
--    O Supabase expõe o schema public pela API REST dele usando os papéis anon e
--    authenticated, e os dois tinham SELECT/INSERT/UPDATE/DELETE/TRUNCATE em todas as
--    tabelas. Quem tivesse a chave anon podia ler, alterar ou apagar o banco inteiro
--    sem passar pela nossa API.
--    A API Go conecta como "postgres" (dono das tabelas, BYPASSRLS), então nada muda pra ela.
--    - RLS ligado sem nenhuma policy = anon/authenticated não veem nenhuma linha
--    - REVOKE remove as permissões (TRUNCATE não passa pelo RLS, por isso os dois)
--    - DEFAULT PRIVILEGES impede que tabelas novas voltem a nascer liberadas
--
-- 2. ÍNDICES
--    - matches por time (jogos do time, trigger da ESPN) e por liga/temporada/data
--      (jogos da rodada, classificação, temporadas disponíveis)
--    - espn_matches por confronto (JOIN com matches em todas as listas de jogos)
--    - team_leagues por liga (times da liga) e player_stats por liga/temporada (artilharia)
--    - Remove índices que duplicavam a chave primária ou ficaram cobertos pelos novos

-- ========== 1. SEGURANÇA ==========

DO $$
DECLARE
    t record;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', t.tablename);
    END LOOP;
END $$;

REVOKE ALL ON ALL TABLES    IN SCHEMA public FROM anon, authenticated;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM anon, authenticated;

ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public REVOKE ALL ON TABLES    FROM anon, authenticated;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public REVOKE ALL ON SEQUENCES FROM anon, authenticated;
ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public REVOKE ALL ON FUNCTIONS FROM anon, authenticated;

-- ========== 2. ÍNDICES ==========

CREATE INDEX IF NOT EXISTS idx_matches_home_team ON public.matches (api_home_team_id);
CREATE INDEX IF NOT EXISTS idx_matches_away_team ON public.matches (api_away_team_id);
CREATE INDEX IF NOT EXISTS idx_matches_league_season_date ON public.matches (league, season, match_date);

CREATE INDEX IF NOT EXISTS idx_espn_matches_teams ON public.espn_matches (espn_home_team_id, espn_away_team_id);

CREATE INDEX IF NOT EXISTS idx_team_leagues_league ON public.team_leagues (league);
CREATE INDEX IF NOT EXISTS idx_player_stats_league_season ON public.player_stats (league, season);

-- Duplicavam a chave primária
DROP INDEX IF EXISTS public.idx_espn_matches_id;
DROP INDEX IF EXISTS public.idx_leagues_code_api;
-- Coberto por idx_matches_league_season_date (league é a primeira coluna)
DROP INDEX IF EXISTS public.idx_matches_league;
