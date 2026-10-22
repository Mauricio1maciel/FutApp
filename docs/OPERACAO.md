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
| `BACKUP_DIR` | Pasta dos backups. Padrão `~/backups/futapp`. |
| `BACKUP_KEEP` | Quantos backups guardar. Padrão 7 (uma semana). |
| `PG_DUMP_PATH` | Caminho do `pg_dump`, se não estiver no PATH. |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` | Push no navegador (versão web). As **mesmas três** no worker e no Render: o worker manda os avisos, a API registra os navegadores e faz o teste do admin. Gere uma vez com `go run . -gerar-vapid`; trocar as chaves invalida a inscrição de todos os navegadores. Sem elas, só o push do app funciona. Ao subir, o worker confere se a chave dele é a mesma que a API entrega (log `Chave do push no navegador confere com a API ✓`); se não for, avisa no log e por push ao admin, porque com chaves diferentes o serviço de push recusa todos os avisos do worker (403). |

## Limite de pedidos (API no Render)

A API limita os pedidos por endereço (detalhes no `docs/API.md`). O endereço do cliente
vem do `CF-Connecting-IP`, que a Cloudflare (na frente do Render) preenche e o cliente
não consegue falsificar; se ele não vier, do `X-Forwarded-For`. Se o endereço vier
interno (ex: o proxy mudou de comportamento), o limite não se aplica, para nunca juntar
todo mundo num balde só, e o log avisa uma vez: `Endereço do cliente não identificado`.
Bloqueios aparecem no log como `[RATE_LIMIT] Limite ... atingido` (um por endereço,
não um por pedido).

`RATE_LIMIT=off` no Render desliga o limite sem precisar de deploy.

## Quando a football-data fica parada

A football-data às vezes para de atualizar uma competição (a Libertadores de 2026 parou
em 22/09). Por isso:
- a gravação da football-data não volta um jogo para trás (encerrado → marcado) no
  mesmo horário: fica o resultado que a ESPN trouxe;
- com o jogo em andamento e a ESPN acompanhando, o placar ao vivo da football-data (que
  chega atrasado) não é gravado por cima do da ESPN; o resultado final dela vale;
- toda madrugada o worker reconfere na ESPN os últimos 3 dias de cada liga;
- o "atualizar jogos" do admin (`/matches?update=true`) roda em segundo plano e também
  reconfere a ESPN. Use-o se um jogo terminou e continua como marcado.

## Aviso de jogo sem resultado

A cada 30 minutos (a primeira volta, 10 minutos depois de o worker subir), o worker
procura jogos que começaram há mais de 4 horas, nos últimos 3 dias, e continuam marcados
ou em andamento. Para cada liga e dia desses jogos, busca de novo o placar na ESPN; o
que continuar sem resultado vai por push para os aparelhos de admin, uma vez por jogo
(a marca fica na tabela `alert_state`, então reiniciar o worker não repete o aviso). No
log: `[JOBS] Sem resultado: ...`, com um dos motivos:
- **na ESPN acabou X x Y, mas o resultado não chegou ao jogo**: o trigger não achou o
  jogo. Quase sempre é o vínculo de um dos times com a ESPN (`teams.espn_team_id`) ou a
  data do jogo diferente nas duas fontes;
- **a ESPN não tem o jogo**: confira o vínculo dos times e a data;
- **a ESPN também não atualizou**: as duas fontes pararam; tente o "atualizar jogos" do
  admin mais tarde.

Sem nenhum aparelho de admin, o aviso fica só no log e sai quando houver um (toque em
"Enviar push de teste para este aparelho" na tela de admin do app).

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

## Backup do banco

Toda madrugada, às 04:00, o worker grava uma cópia do banco em `BACKUP_DIR` e mantém as
últimas 7 (`futapp-AAAA-MM-DD.dump`, uns 2,5 MB cada hoje). Jogos e classificações dá
para baixar de novo das APIs, mas usuários, aparelhos e o que cada um segue não: é isso
que o backup protege.

**Instalar o `pg_dump` no Orange Pi** (uma vez). Ele precisa ser da versão 17, a mesma
do Supabase; a do repositório padrão costuma ser mais antiga:
```bash
sudo apt install -y postgresql-common
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh
sudo apt install -y postgresql-client-17
pg_dump --version   # deve mostrar 17.x
```

**Testar na hora**, sem esperar a madrugada (na pasta do backend, com o `.env`):
```bash
go run . -backup        # ou: ./futapp -backup, se usar o binário compilado
ls -lh ~/backups/futapp
```
Se o `pg_dump` faltar ou for de versão antiga, o erro aparece aqui e no log do worker.

**Cartão SD:** o backup fica no mesmo cartão do sistema. Se o cartão morrer, as cópias
vão junto. De vez em quando, copie a pasta para outro lugar (pendrive, computador ou
nuvem).

**Restaurar** (só em emergência: substitui as tabelas do banco de destino pelas da cópia):
```bash
pg_restore --clean --if-exists --no-owner --no-privileges \
  -d "postgresql://USUARIO:SENHA@HOST:5432/postgres?sslmode=require" \
  ~/backups/futapp/futapp-2026-10-18.dump
```
Num banco novo e vazio (ex: outro projeto do Supabase), tire o `--clean --if-exists`. O
aviso `schema "public" already exists` é esperado e não atrapalha. Restauração testada
em 18/10/2026: todas as tabelas voltaram com as mesmas contagens da produção.
