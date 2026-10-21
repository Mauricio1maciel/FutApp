# API App Futebol: contrato para o app

## Convenções (valem para todas as rotas)

- **Sempre JSON**, com `Content-Type: application/json; charset=utf-8`, inclusive nos erros.
- **Chaves em `snake_case`**: `team_name`, `goal_diff`, `crest_url`.
- **Um único formato de erro** (qualquer status ≥ 400):
  ```json
  { "error": "mensagem legível em português" }
  ```
- **Lista vazia é `[]`**, nunca `null`.
- **Campos de objeto sempre presentes**: um valor vazio vem como `""` ou `0`, sem
  sumir da resposta. Exceção: `home_penalty`/`away_penalty` dos jogos, que só
  aparecem quando houve pênaltis.
- **Datas:**
  - data de nascimento: `"2005-01-19"`;
  - data e hora dos jogos: string ISO vinda do banco/ESPN.

- **CORS (versão web):** a API aceita chamadas de navegador de qualquer origem. O preflight
  (`OPTIONS`) responde `204` com `Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS` e
  `Access-Control-Allow-Headers: Authorization, Content-Type`, sem precisar de token; o site
  também pode ler o header `X-Cache`. Para limitar a domínios conhecidos, defina no servidor
  `CORS_ALLOWED_ORIGINS` (separados por vírgula), ex:
  `https://futstatis.vercel.app,http://localhost:3000`. O app não manda `Origin` e não é afetado.

- **Cache curto no servidor:** as rotas de leitura podem devolver dados com até 20s
  (jogos, detalhes do jogo), 1 min (classificação, calendário) ou 10 min (times,
  elencos, temporadas, artilharia) de atraso. O header `X-Cache` diz `HIT` (veio da
  memória), `MISS` (veio do banco) ou `BYPASS` (admin, sempre fresco). Puxar para
  atualizar mais rápido que isso não traz dado novo.

### Alerta de tamanho do banco

O plano grátis do Supabase vai até 500 MB; acima disso o banco fica só leitura. O worker
mede o banco ao subir e todo dia ao meio-dia, e manda um push (`type: "admin_alert"`,
`alert: "db_size"`) para os aparelhos de admin quando passa de `DB_SIZE_ALERT_MB` (padrão
300) e a cada 50 MB a mais. Cada nível avisa uma vez só. O aparelho de admin é o que
tocou em "Enviar push de teste para este aparelho" na tela de admin; se o app for
reinstalado, é preciso tocar de novo. `DB_SIZE_LIMIT_MB` (padrão 500) muda o limite,
caso o plano mude.

### Status usados

| Status | Significado |
|---|---|
| 200 | OK |
| 202 | Ação de admin iniciada em background |
| 400 | Parâmetro faltando ou inválido |
| 401 | Sem token, token inválido/expirado, ou login errado |
| 403 | Rota exclusiva de admin |
| 404 | Não encontrado (jogo, time, jogador) |
| 405 | Método errado (ex: GET no login) |
| 409 | Ação de admin que já está rodando, ou aparelho não registrado para push |
| 429 | Muitas tentativas de login (espere 15 min) |
| 500 | Erro interno |
| 502 | A API externa (ESPN/football-data) falhou numa ação de admin |

## Versão web (navegador)

A API aceita chamadas do site em qualquer domínio (CORS), inclusive `http://localhost:8081`
do Expo web. Nada muda para o app de celular, que não manda o header `Origin`.
- O navegador faz um "preflight" (`OPTIONS`) antes de cada endereço novo, porque a chamada
  leva `Authorization`. A liberação fica guardada por até 2 h (Chrome) para o mesmo
  endereço, então a primeira chamada de cada endereço custa uma ida a mais.
- Os erros (401, 404, 500) também vêm com CORS: o site consegue ler o `{"error": ...}` e
  renovar o token no 401, como o app faz.
- O site pode ler o header `X-Cache`.
- Login de convidado: o site precisa de um `device_id` próprio, guardado no
  `localStorage`, e usa o mesmo `POST /auth/guest`.
- **Notificações no navegador:** funcionam pelo Web Push (veja "Push no navegador", na
  seção de notificações). Seguir times, ligas e jogos é igual ao app.
- Para restringir quais sites podem chamar a API, defina no Render
  `CORS_ALLOWED_ORIGINS` (ex: `https://futstatis.web.app,http://localhost:8081`). Vazio =
  qualquer origem, o que é seguro aqui porque a API não usa cookies.

## Fotos e escudos

`headshot_url` (jogador) e `crest_url` (time) vêm **vazios** quando não há imagem. Hoje
isso vale para a maioria dos jogadores: a ESPN só tem foto de ~4% deles, e o robô de
imagens completa aos poucos pelo Transfermarkt (começando por quem aparece em
escalações). O app e o site devem mostrar um avatar com as iniciais (jogador) ou um escudo
genérico com as iniciais (time) quando o campo vier vazio, e também quando a imagem não
carregar. Endereços quebrados não são mais gravados.

## Autenticação

Todas as rotas, menos `/auth/*`, exigem `Authorization: Bearer <token>`.

| Rota | Body | Resposta 200 |
|---|---|---|
| `POST /auth/guest` | `{"device_id": "..."}` | `{"token", "type": "Bearer"}`, token de convidado de 30 dias |
| `POST /auth/login` | `{"email", "password"}` | `{"token", "type": "Bearer", "role": "admin"}`, válido por 7 dias |

- Se o token de admin der **401** em qualquer rota, ele expirou: apague-o e volte ao token de convidado.
- O token de admin funciona em todas as rotas.

## Rotas

Parâmetros entre `[colchetes]` são opcionais. `season` vazio = temporada mais recente.

| Rota | Parâmetros | Resposta |
|---|---|---|
| `GET /matches` | `league`, `[season]`, `[round]`, `[stage]`, `[date=YYYY-MM-DD]` | lista de jogos |
| `GET /team/matches` | `id`, `[rounds=1,2,3]` | jogos do time na **temporada atual** de cada competição |
| `GET /h2h` | `team1`, `team2`, `[limit=10]` | confronto direto (veja abaixo) |
| `GET /player/stats` | `id` (ID da ESPN, o mesmo do `/details?type=player`), `[season]` | estatísticas do jogador (veja abaixo) |
| `GET /team/stats` | `id` (api_id), `[season]` | estatísticas do time na temporada (veja abaixo) |
| `GET /matches/calendar` | `leagues=BSA,PL`, `month=MM`, `year=YYYY` | `[{"date", "count"}]` |
| `GET /matches/live` | `league`, `[date=YYYYMMDD]` | lista de jogos ao vivo (ESPN, cache de 30s). Aqui `id_event` **e** `match_id` são o ID da ESPN |
| `GET /match/history` | `id` (ID ESPN), `league` | `{"match", "lineups", "events", "statistics"}` |
| `GET /standings` | `league`, `[season]` | lista de posições |
| `GET /standings?live=true` | `league`, `[season]` | a classificação **ao vivo** (veja abaixo) |
| `GET /seasons` | `league` | `["2026", "2025", "2024", "2023"]`: inclui as temporadas antigas carregadas |
| `GET /league/stats` | `league`, `[season]` | `{"top_scorers": [...], "top_assists": [...]}` |
| `GET /teams` | `league` | lista de times |
| `GET /players` | `league` | lista de jogadores |
| `GET /team/players` | `teamID`, `league` | lista de jogadores |
| `GET /details` | `api_id`, `type=team\|player` | um time ou um jogador |
| `GET /search` | `q` | `{"leagues": [...], "teams": [...], "players": [...]}` |

### `GET /matches`: quais jogos vêm

| Parâmetro | Devolve |
|---|---|
| `round=30` (número) | a rodada 30 |
| `round=SEMI_FINALS` (texto) ou `stage=SEMI_FINALS` | a fase inteira (valores em `stage`, ex: `GROUP_STAGE`, `QUARTER_FINALS`, `FINAL`) |
| `stage=LEAGUE_STAGE&round=3` | a rodada 3 **daquela fase**. Use quando o número da rodada se repete entre fases: na Champions, `round=1` sozinho traz a 1ª rodada da fase de liga **e** os jogos de ida do mata-mata |
| `date=2026-10-11` | os jogos do dia, no horário de Brasília (tem prioridade sobre `round`) |
| nenhum dos dois | **a fase e a rodada atuais**: as do próximo jogo; se a temporada já acabou, as do último |

A fase atual funciona igual para ligas e copas. Exemplos com os dados de hoje:
`BSA` → rodada 30; `CL` → rodada 2 da fase de liga (18 jogos); `CLI` → as 4
semifinais; `WC` (encerrada) → a final. Cada jogo traz `stage` e `round`, então o app
sabe qual fase e rodada recebeu e pode marcar no seletor.

### Ligas que vêm da ESPN: Série B (`BSB`), Copa do Brasil (`CDB`), Sul-Americana (`CSU`) e Liga das Nações (`UNL`)

A football-data não cobre essas quatro no plano grátis, então os jogos vêm da ESPN. Para o
app, as rotas e os formatos são os mesmos das outras ligas, com estas diferenças:
- **Times:** os que já existem por outra liga mantêm o ID (os brasileiros da
  Sul-Americana são os mesmos do Brasileirão). Os demais têm `api_id` acima de
  1.000.000.000.
- **Grupos:** `group_name` no formato da ESPN, `"Group A"` (na Libertadores é `"GROUP_A"`).
- **Fases:** os mesmos nomes da Libertadores: `ROUND_1` a `ROUND_5` (fases
  preliminares), `GROUP_STAGE`, `PLAYOFFS`, `LAST_16`, `QUARTER_FINALS`, `SEMI_FINALS`,
  `FINAL`. A fase de liga da UNL continua `league-phase`.
- **Rodada:** nas copas, `round` é sempre 1: navegue pela fase (`round=GROUP_STAGE`).
- **Série B:** pontos corridos como o Brasileirão (`round=1` a `38`, fase
  `REGULAR_SEASON`). A ESPN não informa a rodada: ela é calculada pelo calendário,
  considerando jogos adiados. Regra de 2026 (CBF): **1º e 2º sobem direto** (zona
  `acesso`), **3º a 6º vão aos playoffs de acesso** (zona `playoff`) e 17º a 20º caem
  (`rebaixamento`); o resto é `neutro`.
- **Playoffs da Série B:** 3º x 6º e 4º x 5º, em ida e volta (21 e 28/11), valendo as
  outras 2 vagas. A ida é na casa do pior colocado; a volta, na do melhor. Empate no
  agregado: avança o de **melhor campanha, sem pênaltis**. Os 4 jogos vêm com fase
  `PLAYOFFS` e `round` 0 (`/matches?league=BSB&round=PLAYOFFS`), com os times "a
  definir" (ID 0) até o fim da 38ª rodada. Não somam pontos na tabela. Depois da última
  rodada, o `/matches?league=BSB` sem `round` passa a trazer os playoffs.
- **Pênaltis e vencedor:** como na football-data. Jogo decidido nos pênaltis traz
  `home_penalty`/`away_penalty`, e `winner` é `HOME_TEAM`, `AWAY_TEAM` ou `DRAW` (vazio
  enquanto o jogo não acabou).
- **Copa do Brasil:** só mata-mata, então **não tem classificação** (`/standings`
  devolve `[]`).
- **Classificação da Sul-Americana:** por grupo. O 1º vai às oitavas
  (`"Classificado - Oitavas"`), o 2º joga os playoffs contra um 3º da Libertadores
  (`"Playoffs (pré-oitavas)"`), 3º e 4º são `"Eliminado"`.
- Toda madrugada o worker confere a **temporada inteira** pelo calendário da ESPN (jogos
  novos, remarcados e os que ficaram sem resultado com o worker desligado); o placar ao
  vivo, como nas outras ligas.

### Formatos

**Posição na classificação** (`/standings`):
```json
{ "position": 1, "group_name": "", "team_id": 1783, "team_name": "CR Flamengo",
  "crest_url": "https://...", "played": 28, "wins": 18, "draws": 6, "losses": 4,
  "goals_for": 55, "goals_against": 23, "goal_diff": 32, "points": 60,
  "zone": "libertadores", "season": "2026", "form": "VVEDV" }
```
**Classificação ao vivo** (`/standings?league=PL&live=true`): a tabela como estaria se
os jogos em andamento acabassem agora, montada a cada pedido (cache de 20 s). Mesmo
formato da `/standings`, com dois campos a mais, que só aparecem quando valem:
`"live": true` (o time está jogando agora) e `"position_change": 2` (posições ganhas, ou
perdidas se negativo, em relação à tabela antes dos jogos em andamento). Sem jogo
rolando, é igual à tabela normal. A `/standings` normal também conta os jogos em
andamento, mas só é recalculada a cada ~10 minutos e não traz as marcações.

**Vagas por título (Brasileirão):** os campeões da Libertadores, da Sul-Americana e da
Copa do Brasil têm vaga garantida na Libertadores, e o vice da Copa do Brasil ganha uma
vaga a mais na pré-Libertadores. O backend lê o vencedor de cada final sozinho e a
`zone` da tabela muda em até ~10 minutos depois do apito final. Quando o campeão já está
na zona da Libertadores, a vaga passa para o próximo da tabela.

`form`: os últimos 5 resultados na competição, do mais antigo para o mais recente
(`V` vitória, `E` empate, `D` derrota). Tem menos letras no começo da temporada.
Em copas e na Liga das Nações, `group_name` vem preenchido (ex: `"Group C1"`) e a
lista vem ordenada por grupo e posição.

**Liga** (`leagues` do `/search`, até 5, só ligas com jogos no banco):
```json
{ "code": "BSA", "name": "Brasileirão - Série A",
  "logo_url": "https://...", "season": "2026" }
```
`code` é o valor do parâmetro `league` das outras rotas, e `season` é a temporada mais
recente. A busca acha pelo nome, pelo código (`pl`, `bsa`) e por apelidos sem acento
obrigatório ("brasileirão", "champions", "campeonato inglês", "copa do mundo").

**Jogo** (`/matches`, `/team/matches`), campos principais:
```json
{ "match_id": 4821, "id_event": "401861080", "league": "BSA", "season": "2026",
  "round": 30, "stage": "REGULAR_SEASON", "group_name": "",
  "api_home_team_id": 1783, "home_team": "CR Flamengo", "home_logo": "https://...",
  "api_away_team_id": 1769, "away_team": "SE Palmeiras", "away_logo": "https://...",
  "home_score": 0, "away_score": 0, "date_event": "2026-10-11 21:30:00", "status": "TIMED" }
```
- `match_id`: ID do jogo no nosso banco. **Estável**: existe desde que o jogo é marcado.
  É o que o app usa para seguir o jogo (notificações).
- `id_event`: ID da ESPN, usado para abrir os detalhes (`/match/history`). Vem `"0"`
  enquanto o jogo ainda não tem vínculo com a ESPN (comum em jogos futuros).

**Estatísticas do jogo** (`statistics` do `/match/history`), já em ordem para exibir:
```json
[ { "key": "possessionPct", "label": "Posse de bola", "home": 50.2, "away": 49.8, "unit": "%" },
  { "key": "totalShots", "label": "Finalizações", "home": 21, "away": 10, "unit": "" } ]
```
Linhas possíveis, nesta ordem: posse de bola, finalizações, finalizações no gol,
escanteios, impedimentos, faltas, cartões amarelos, cartões vermelhos, defesas do goleiro,
passes, precisão dos passes, desarmes, interceptações. Só vêm as que a ESPN informou para
os dois times; antes do jogo, e em jogos menores sem dados, a lista vem vazia (`[]`).
Durante o jogo, elas são atualizadas a cada minuto, junto com o placar. O `match` também
traz `home_penalty`/`away_penalty` quando houve disputa de pênaltis.

**Confronto direto** (`/h2h?team1=1783&team2=1769`): todos os jogos já encerrados
entre os dois, em qualquer competição e temporada que o banco tem.
```json
{ "team1": 1783, "team2": 1769,
  "summary": { "played": 7, "team1_wins": 3, "draws": 2, "team2_wins": 2,
               "team1_goals": 9, "team2_goals": 8 },
  "matches": [ ...jogos no formato de /matches, do mais recente para o mais antigo... ] }
```
O `summary` conta todos os jogos; `matches` traz até `limit` (padrão 10, máximo 50).
Decisão por pênaltis conta como empate no resumo.

**Estatísticas do jogador** (`/player/stats?id=235017`), calculadas das escalações e
dos lances dos jogos que a ESPN detalha:
```json
{ "player_id": 235017,
  "current": { "appearances": 31, "starts": 28, "minutes": 2410, "goals": 16, "penalties": 3,
               "own_goals": 0, "yellow_cards": 4, "red_cards": 0 },
  "competitions": [ { "league": "BSA", "league_name": "Brasileirão - Série A", "season": "2026",
                      "team": { "id": 1783, "name": "CR Flamengo", "crest_url": "..." },
                      "appearances": 31, "starts": 28, "minutes": 2410, "goals": 16, ... } ],
  "last_matches": [ { "espn_match_id": 401841258, "league": "BSA", "date": "2026-10-18T21:30:00Z",
                      "team": {...}, "opponent": {...}, "home": true, "goals_for": 3, "goals_against": 0,
                      "result": "V", "started": true, "minutes": 76, "goals": 1, "penalties": 0,
                      "own_goals": 0, "yellow_cards": 0, "red_cards": 0, "subbed_out": "76'" } ] }
```
- `current` soma a temporada atual de cada competição; `competitions` traz uma linha por
  competição, temporada e time (o jogador que trocou de time aparece duas vezes).
- `goals` inclui os de pênalti (`penalties`); gol contra fica em `own_goals`.
- `minutes` é aproximado: 90 por jogo (120 com prorrogação), menos o tempo fora de campo.
- Reserva que não entrou não conta como jogo. Assistências não vêm (a ESPN não manda nos
  lances).
- `last_matches`: até 10, do mais recente; `espn_match_id` abre a tela do jogo.
- Só jogos com escalação da ESPN (temporada atual das ligas acompanhadas).

**Estatísticas do time** (`/team/stats?id=1783`), uma linha por competição:
```json
{ "team_id": 1783,
  "competitions": [ { "league": "BSA", "league_name": "Brasileirão - Série A", "season": "2026",
      "played": 34, "wins": 21, "draws": 7, "losses": 6, "goals_for": 58, "goals_against": 27,
      "home": { "played": 17, "wins": 13, ... }, "away": { ... },
      "clean_sheets": 14, "failed_to_score": 4,
      "biggest_win": { "match_id": 4821, "espn_match_id": "401841258", "date": "...",
                       "home_team": "CR Flamengo", "away_team": "EC Bahia", "home_score": 4, "away_score": 0 },
      "biggest_loss": { ... },
      "averages": { "matches": 34, "possession": 57.3, "shots": 15.2, "shots_on_target": 5.4,
                    "corners": 6.1, "fouls": 12.8, "yellow_cards": 2.1, "red_cards": 0.1,
                    "pass_accuracy": 86.2 },
      "top_scorers": [ { "player_id": 235017, "name": "Pedro", "headshot_url": "...", "goals": 16 } ] } ] }
```
- A campanha (`played`, `wins`... `biggest_loss`) conta todos os jogos encerrados; decisão
  por pênaltis conta como empate.
- `averages` e `top_scorers` vêm dos jogos detalhados pela ESPN (`averages.matches` diz
  quantos); `averages` é `null` quando nenhum tem estatística. `top_scorers`: até 5.
- Sem `season`: a temporada atual de cada competição em que o time joga.

**Time:**
```json
{ "id": 1, "api_id": 1783, "name": "CR Flamengo", "short": "Flamengo", "tla": "FLA",
  "league": "BSA", "stadium": "Maracanã", "crest_url": "https://..." }
```

**Jogador:**
```json
{ "id": 123, "api_id": 0, "name": "Pedro", "short_name": "Pedro", "position": "Forward",
  "jersey_number": 9, "date_of_birth": "1997-06-20", "nationality": "Brazil",
  "team_id": 1783, "team_name": "CR Flamengo", "headshot_url": "https://...",
  "source": "ESPN", "league": "BSA" }
```

## Notificações push

O app avisa o usuário de **início do jogo, gols, fim do jogo e escalação confirmada**
de tudo o que ele segue:
- **time ou seleção** (estrela na tela do time): todos os jogos dele, em qualquer liga;
- **liga** (estrela na tela da liga): todos os jogos da liga;
- **jogo avulso** (estrela no jogo): só aquele jogo.

Seguir mais de um caminho que leva ao mesmo jogo (ex: o time e a liga) não duplica o
aviso. A entrega é pelo Expo Push.

**Use sempre o token de convidado** nestas rotas, mesmo com o admin logado: é o
`device_id` dele que identifica o aparelho. Com o token de admin elas respondem 400.

| Rota | Body / parâmetros | Resposta |
|---|---|---|
| `POST /push/register` | app: `{"token": "ExponentPushToken[...]", "platform": "android"}` · site: `{"platform": "web", "subscription": {...}}` | 200 `{"status": "ok"}` · 503 (site) push no navegador desligado |
| `GET /push/web-key` | (sem token) | 200 `{"public_key": "..."}` · 503 push no navegador desligado |
| `DELETE /push/register` | | 200: desliga as notificações e apaga tudo o que o aparelho seguia |
| `GET /push/subscriptions` | | 200 `{"matches": [4821], "teams": [1783], "leagues": ["BSA"], "kinds": {...}, "available_kinds": [...]}` (veja "Tipos de aviso") |
| `POST /push/subscriptions` | um entre `{"team_id"}`, `{"league"}`, `{"match_id"}` ou `{"espn_match_id"}`, e opcionalmente `"kinds"` | 200 `{"status": "ok", "team_id": 1783}`, `{"status": "ok", "league": "BSA"}` ou `{"status": "ok", "match_id": 4821}` (com `"kinds"`, se veio) · 400 tipo inválido ou lista vazia |
| `DELETE /push/subscriptions` | `?team_id=`, `?league=`, `?match_id=` ou `?espn_match_id=` | 200, mesmo formato do POST |

- Chame o `POST /push/register` ao abrir o app (o token pode mudar). Repetir não dá erro.
- `espn_match_id` serve para a tela de jogos ao vivo, que só conhece o ID da ESPN; a
  resposta devolve o `match_id` correspondente. IDs podem ir como número ou texto.
- `league` é o código da liga (`BSA`, `BSB`, `PL`, `CLI`, `CSU`, `CDB`, `UNL`...), o mesmo das outras rotas.
- Seguir o mesmo jogo, time ou liga de novo não dá erro.
- Erros: 400 (token inválido, nenhum ou mais de um alvo), 404 (jogo, time ou liga não existe),
  409 (aparelho ainda não registrado: chame o `POST /push/register` antes).

**O que chega no celular:**

| Aviso | Título | Texto |
|---|---|---|
| Escalação (~30 min antes) | `Escalações confirmadas 📋` | `Flamengo x Palmeiras · começa às 21:30` |
| Início | `Começou! ⚽` | `Flamengo x Palmeiras` |
| Gol | `⚽ GOL! Flamengo` | `Flamengo 1 x 0 Palmeiras · Pedro 23'` (autor quando a ESPN já informou) |
| Cartão vermelho | `🟥 Cartão vermelho · Palmeiras` | `Gustavo Gómez 38' · Flamengo 1 x 0 Palmeiras` |
| Fim | `Fim de jogo` | `Flamengo 2 x 1 Palmeiras` (pênaltis: `1 (4) x (3) 1`) |

**Tipos de aviso:** `lineup` (escalação confirmada), `start` (começou), `goal` (gol),
`red_card` (cartão vermelho) e `end` (fim de jogo). Cada inscrição (time, liga ou jogo)
recebe todos, a não ser que escolha:
```json
POST /push/subscriptions  { "team_id": 1783, "kinds": ["goal", "red_card", "end"] }
```
- Sem `"kinds"`, a inscrição nova recebe todos e a que já existe **mantém** a escolha (o
  app pode seguir de novo ao abrir sem apagar a preferência).
- A lista com todos os tipos volta ao padrão (inclusive tipos que surgirem depois). Lista
  vazia dá 400: para não receber nada, deixe de seguir (`DELETE`).
- Quem segue o mesmo jogo por dois caminhos (ex: o time e a liga) recebe o aviso se
  qualquer um dos dois pedir aquele tipo.
- O `GET` traz, em `"kinds"`, só as inscrições com escolha (as outras recebem todos), e a
  lista de tipos em `"available_kinds"`:
```json
{ "matches": [4821], "teams": [1783], "leagues": ["BSA"],
  "kinds": { "matches": {}, "teams": { "1783": ["goal", "end"] }, "leagues": {} },
  "available_kinds": ["lineup", "start", "goal", "red_card", "end"] }
```

**Cartão vermelho:** *"🟥 Cartão vermelho · Palmeiras"* | *"Gustavo Gómez 38' · Flamengo
1 x 0 Palmeiras"*. Um aviso por cartão, só com o jogo rolando (ou recém-acabado). Quando a
ESPN não diz o time do expulso, o título vem sem o nome do time.

**Push no navegador (versão web).** Mesmos avisos, textos, tipos e inscrições do app;
só o registro muda:
1. `GET /push/web-key` (sem token) → `{"public_key": "..."}`. Com 503, o push no
   navegador está desligado no servidor: esconda a opção.
2. No site, registre um service worker e inscreva o navegador com essa chave:
   `registration.pushManager.subscribe({userVisibleOnly: true, applicationServerKey: <public_key>})`.
3. `POST /push/register` com `{"platform": "web", "subscription": <inscrição.toJSON()>}`
   (endpoint e keys `p256dh`/`auth`). Só são aceitos endereços dos serviços de push dos
   navegadores (Chrome, Firefox, Safari e Edge); outro dá 400.
4. Seguir e deixar de seguir: as mesmas rotas `/push/subscriptions` do app.

O service worker recebe, no evento `push`, o JSON
`{"title", "body", "tag", "data"}`, com o mesmo `data` dos avisos do app, e mostra com
`showNotification(title, {body, tag, data})`. O `tag` (`jogo-<match_id>`) faz o aviso novo
de um jogo substituir o anterior, como no app. No clique, abra a página do jogo a partir
do `data`, do mesmo jeito que o app. No iPhone, o Safari só recebe push com o site
adicionado à tela de início (PWA, iOS 16.4 ou mais novo). Se o navegador trocar a
inscrição (evento `pushsubscriptionchange`), registre de novo. Inscrição que o
navegador cancelou é apagada sozinha no primeiro envio que falhar.

**Uma notificação por jogo:** escalação, início, gols, cartões vermelhos e fim do mesmo jogo usam o mesmo
identificador (`tag`/`collapseId` = `jogo-<match_id>`), então cada aviso novo
**substitui** o anterior na tela em vez de empilhar. Como o texto sempre traz o placar
atual, a notificação vira um resumo do jogo. Jogos diferentes continuam separados.

Cada notificação leva um `data` para o app abrir a tela do jogo ao ser tocada:
```json
{ "type": "goal", "match_id": "4821", "league": "BSA",
  "espn_match_id": "401861080", "espn_home": "819", "espn_away": "2029",
  "date": "2026-10-12T00:30:00Z" }
```
`type` é `lineup`, `start`, `goal`, `red_card`, `end`, `test` ou `admin_alert`. `espn_match_id`, `espn_home`,
`espn_away` e `date` só vêm quando o backend os conhece. Todos os valores são texto.

Os avisos saem do worker em até ~1 minuto depois do lance (o tempo de a ESPN atualizar
e o worker ler). Aviso que ficar mais de 15 minutos sem sair é descartado.

## Admin

### Parâmetros que só funcionam com token de admin
Com token de convidado eles são **ignorados**, e a rota responde normalmente com os dados do banco.

| Rota | Parâmetro | Efeito |
|---|---|---|
| `/matches` | `update=true` | busca os jogos na football-data antes de responder |
| `/standings` | `update=true` | recalcula a classificação |
| `/league/stats` | `update=true` | atualiza artilharia e assistências na ESPN |
| `/match/history` | `force_update=true` | busca escalação e eventos na ESPN |
| `/teams` | `update=true` | atualiza os times na football-data (a resposta é a mesma lista) |
| `/players` | `force_update=true` | atualiza os elencos na football-data (a resposta é a mesma lista) |

### Rotas exclusivas (403 para convidado)

| Rota | Resposta |
|---|---|
| `GET /admin/sync-daily` | 202 `{"message"}` (roda em background) · 409 se já estiver rodando |
| `GET /admin/sync-teams` | 200 `{"message", "linked"}` |
| `GET /admin/force-sync?league=BSA` | 200 `{"message", "total"}` |
| `GET /admin/run-image-bot` | 202 `{"message"}` · 409 se já estiver rodando |
| `GET /team/players_espn?teamID=X&league=BSA` | 200 `{"message", "team_api_id", "espn_team_id", "espn_league"}` |
| `POST /admin/sync-season?league=BSA&season=2024` | 202: carrega uma temporada antiga (times, jogos e classificação) em background · 409 se outra estiver carregando |
| `POST /admin/push/test` `{"device_id"}` | 200 `{"expo_status", "expo_error", "expo_message", "admin_alerts"}` · 404 aparelho não registrado · 502 Expo fora. Com o envio aceito, o aparelho passa a receber os **alertas do servidor** (`admin_alerts: "ativado"`) |
| `POST /admin/sync-espn-season?league=UNL` | 202: carrega na hora a temporada atual inteira de uma liga que vem da ESPN (`BSB`, `CDB`, `CSU`, `UNL`), pelo calendário dela; o worker faz o mesmo toda madrugada · 400 liga que não vem da ESPN · 409 outra carga em andamento |
| `GET /admin/db-size` | `{"size_mb", "limit_mb", "percent", "alert_mb", "tables": [{"name", "size_mb"}]}`: tamanho do banco e as 10 maiores tabelas |

---

## O que mudou nesta versão (checklist para o app)

1. **Classificação (`/standings`): todas as chaves mudaram** de PascalCase para snake_case:

   | Antes | Agora |
   |---|---|
   | `Position` | `position` |
   | `GroupName` | `group_name` |
   | `TeamID` | `team_id` |
   | `TeamName` | `team_name` |
   | `Played`, `Wins`, `Draws`, `Losses`, `Points` | `played`, `wins`, `draws`, `losses`, `points` |
   | `GoalsFor`, `GoalsAgainst`, `GoalDiff` | `goals_for`, `goals_against`, `goal_diff` |
   | `CrestURL` | `crest_url` |
   | `Zone`, `Season` | `zone`, `season` |

2. **Times (`/teams`, `/details`, `/search`):** `crestUrl` → `crest_url`.
3. **Jogadores (`/players`, `/team/players`, `/details`, `/search`):**
   - `dateOfBirth` → `date_of_birth`;
   - em `/players` e `/team/players`, a data passou de `"2005-01-19T00:00:00Z"` para `"2005-01-19"`;
   - campos vazios agora vêm como `""`/`0`, em vez de sumir.
4. **Erros:** sempre `{"error": "..."}`.
   - A tela de login lia `erro`: troque para `error`.
   - Erros que vinham como texto puro (ex: `Informe a liga`) agora também são JSON.
5. **Listas vazias** em `/seasons`, `/matches/calendar` e `/matches/live` agora vêm como `[]` (antes vinham `null`).
6. **`/details`** passa a aceitar `type=player` (além de `players`) e responde **404** quando não acha (antes era 500).
7. **Admin:**
   - `/teams?update=true` e `/players?force_update=true` agora devolvem a mesma lista do GET normal;
   - `/admin/sync-daily` e `/admin/run-image-bot` respondem JSON (`202`/`409`), em vez de texto;
   - falha da ESPN em `/team/players_espn` agora responde 502 (antes era 200 com `error`).
