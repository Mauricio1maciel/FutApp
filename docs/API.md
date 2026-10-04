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
| 409 | Ação de admin que já está rodando |
| 429 | Muitas tentativas de login (espere 15 min) |
| 500 | Erro interno |
| 502 | A API externa (ESPN/football-data) falhou numa ação de admin |

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
| `GET /matches` | `league`, `[season]`, `[round]`, `[date=YYYY-MM-DD]` | lista de jogos |
| `GET /team/matches` | `id`, `[rounds=1,2,3]` | lista de jogos |
| `GET /matches/calendar` | `leagues=BSA,PL`, `month=MM`, `year=YYYY` | `[{"date", "count"}]` |
| `GET /matches/live` | `league`, `[date=YYYYMMDD]` | lista de jogos ao vivo (ESPN, cache de 30s) |
| `GET /match/history` | `id` (ID ESPN), `league` | `{"match", "lineups", "events"}` |
| `GET /standings` | `league`, `[season]` | lista de posições |
| `GET /seasons` | `league` | `["2026", "2025"]` |
| `GET /league/stats` | `league`, `[season]` | `{"top_scorers": [...], "top_assists": [...]}` |
| `GET /teams` | `league` | lista de times |
| `GET /players` | `league` | lista de jogadores |
| `GET /team/players` | `teamID`, `league` | lista de jogadores |
| `GET /details` | `api_id`, `type=team\|player` | um time ou um jogador |
| `GET /search` | `q` | `{"teams": [...], "players": [...]}` |

### Formatos

**Posição na classificação** (`/standings`):
```json
{ "position": 1, "group_name": "", "team_id": 1783, "team_name": "CR Flamengo",
  "crest_url": "https://...", "played": 28, "wins": 18, "draws": 6, "losses": 4,
  "goals_for": 55, "goals_against": 23, "goal_diff": 32, "points": 60,
  "zone": "libertadores", "season": "2026" }
```
Em copas e na Liga das Nações, `group_name` vem preenchido (ex: `"Group C1"`) e a
lista vem ordenada por grupo e posição.

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
