-- Alerta de tamanho do banco. O plano grátis do Supabase vai até 500 MB (acima disso o
-- banco fica só leitura). O worker mede o banco e avisa o admin por push a cada 50 MB
-- acima de DB_SIZE_ALERT_MB (padrão 300).
--
-- push_devices.admin_alerts: aparelhos que recebem os alertas do servidor. Liga sozinho
-- quando o admin toca em "Enviar push de teste para este aparelho" no app.
-- alert_state: o último nível avisado, para o aviso não se repetir a cada verificação
-- nem quando o worker reinicia.
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE push_devices ADD COLUMN IF NOT EXISTS admin_alerts BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS alert_state (
    key        VARCHAR(64) PRIMARY KEY,
    value      BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Mesmo padrão de segurança das outras tabelas (a API conecta como postgres)
ALTER TABLE alert_state ENABLE ROW LEVEL SECURITY;

-- Os papéis do Supabase não existem num Postgres comum (ex: teste no Docker)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'anon') THEN
        REVOKE ALL ON alert_state FROM anon, authenticated;
    END IF;
END $$;

COMMIT;
