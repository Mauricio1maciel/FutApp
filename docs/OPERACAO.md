# Operação do worker (Orange Pi)

O mesmo programa roda em dois modos: a **API** (Render, só lê o banco) e o **worker**
(`-worker`, no Orange Pi), que busca ESPN e football-data, grava no banco e manda as
notificações. Tudo o que está aqui é do worker.

## Variáveis do `.env` do worker

Além das do banco (`DB_*`), do `API_TOKEN` da football-data e do `JWT_SECRET`:

| Variável | Para que serve |
|---|---|
| `API_PUBLIC_URL` | Endereço da API no Render. O worker chama o `/health` a cada 10 min para ela não dormir (o plano grátis desliga após 15 min parado). |
| `HEALTHCHECK_URL` | Endereço do monitor externo (veja abaixo). Sem ele, o sinal de vida fica desligado. |
| `DB_SIZE_ALERT_MB` | Tamanho do banco (MB) a partir do qual o admin recebe push. Padrão 300. |
| `DB_SIZE_LIMIT_MB` | Limite do plano do Supabase em MB. Padrão 500 (grátis). |
| `DB_SSLMODE` | Padrão `require`. `disable` só para banco local sem SSL. |

## Aviso se o worker parar (healthchecks.io)

O worker manda um sinal de vida a cada 5 minutos. Se o Orange Pi travar, ficar sem
energia ou sem internet, os sinais param e o monitor avisa. Ele também manda **sinal de
falha** (com o motivo) quando o banco está fora do ar ou quando o laço do placar ao vivo
ou o da football-data passa mais de 10 minutos sem rodar.

Configuração (uma vez, grátis):
1. Crie uma conta em <https://healthchecks.io>.
2. **Add Check**: *Period* 5 minutos, *Grace* 10 minutos.
3. Em **Integrations**, deixe o e-mail (já vem ligado) e, se quiser, ligue o Telegram.
4. Copie o **Ping URL** (`https://hc-ping.com/...`) e coloque no `.env` do Orange Pi:
   `HEALTHCHECK_URL=https://hc-ping.com/...`
5. Reinicie o worker. Em até 5 minutos o check fica verde.

O alerta de tamanho do banco (push para o admin) não substitui isso: ele roda dentro do
próprio worker e para junto com ele.
