-- Índice para a busca de jogadores (/search). A busca é por pedaço do nome, sem acento
-- (nome sem acento ILIKE '%pedro%'), e varria os 13 mil jogadores da ESPN a cada pedido:
-- 30 ms com "pedro", 95 ms em média nos pedidos mais pesados. Com um índice de trigramas
-- (pg_trgm) sobre o nome sem acento, os candidatos saem do índice.
--
-- O unaccent não pode entrar num índice (não é IMMUTABLE: o dicionário pode mudar).
-- f_unaccent fixa o dicionário e pode. A busca precisa usar a mesma função
-- (database/search.go).
--
-- ⚠️ Aplique ANTES de publicar a API nova: a busca nova chama f_unaccent.
--
-- Pode rodar de novo sem efeito.

BEGIN;

CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE SCHEMA IF NOT EXISTS extensions;
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA extensions;

-- unaccent com o dicionário fixo, no schema onde a extensão estiver instalada
DO $$
DECLARE
    s TEXT;
BEGIN
    SELECT n.nspname INTO s
      FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace
     WHERE e.extname = 'unaccent';
    EXECUTE format(
        'CREATE OR REPLACE FUNCTION public.f_unaccent(text) RETURNS text
         LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS %L',
        format('SELECT %I.unaccent(%L::regdictionary, $1)', s, s || '.unaccent'));
END $$;

CREATE INDEX IF NOT EXISTS idx_espn_players_nome_busca
    ON espn_players USING gin (public.f_unaccent(name) extensions.gin_trgm_ops);

COMMIT;
