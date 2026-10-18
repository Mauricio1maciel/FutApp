-- Fotos dos jogadores.
-- photo_checked_at: quando o robô de imagens procurou a foto pela última vez. Sem foto,
-- ele tenta de novo depois de 30 dias; com foto do Transfermarkt, atualiza a foto e o
-- valor de mercado a cada 90 dias.
-- O robô gravava 'NOT_FOUND' no lugar da foto, e o app recebia isso como endereço de
-- imagem: vira vazio (o robô tenta de novo, agora comparando os nomes melhor).
--
-- Pode rodar de novo sem efeito.

BEGIN;

ALTER TABLE espn_players ADD COLUMN IF NOT EXISTS photo_checked_at TIMESTAMPTZ;

UPDATE espn_players SET headshot_url = '' WHERE headshot_url = 'NOT_FOUND';

UPDATE espn_players SET photo_checked_at = last_updated
WHERE headshot_url LIKE '%transfermarkt%' AND photo_checked_at IS NULL;

COMMIT;
