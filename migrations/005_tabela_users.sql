-- Usuários com login (e-mail + senha).
--
-- Hoje o app usa só tokens de convidado (por device_id). Esta tabela guarda as
-- contas que fazem login; por enquanto só o admin, que pode forçar atualizações
-- (update=true / force_update=true). Não existe cadastro pela API: o admin é criado
-- pelo terminal com `go run . -criar-admin seu@email.com`.
--
-- A senha é guardada só como hash bcrypt, nunca em texto.

CREATE TABLE IF NOT EXISTS public.users (
    id            bigserial PRIMARY KEY,
    email         varchar(255) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          varchar(20) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

-- Mesmo padrão da 004: invisível para a API REST do Supabase (anon/authenticated)
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.users FROM anon, authenticated;
REVOKE ALL ON SEQUENCE public.users_id_seq FROM anon, authenticated;
