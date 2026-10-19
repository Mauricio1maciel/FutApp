-- Notificações no navegador (Web Push), para a versão web do app. A inscrição do navegador
-- fica em push_devices com platform = 'web': push_token recebe o endereço (endpoint) do
-- serviço de push do navegador, único como o token do Expo, e as duas chaves da
-- inscrição vão em web_p256dh e web_auth (a mensagem vai criptografada com elas).
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE push_devices ADD COLUMN IF NOT EXISTS web_p256dh TEXT;
ALTER TABLE push_devices ADD COLUMN IF NOT EXISTS web_auth TEXT;

COMMIT;
