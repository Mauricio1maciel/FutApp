# Migrations do banco

Todo o schema do banco (tabelas, constraints, índices, funções e triggers) fica
versionado aqui. **Nenhuma mudança de estrutura deve ser feita só pelo painel do
Supabase**: se mudar lá, crie o arquivo aqui também.

## Arquivos

| Arquivo | O que faz |
|---|---|
| `000_schema_base.sql` | Schema completo, exportado da produção em 2026-10-03 (já inclui 001 e 002) |
| `seed_configuracao.sql` | Dados de configuração: ligas, regras de zona, critérios de desempate, zonas |
| `001_...sql`, `002_...sql`, ... | Mudanças aplicadas depois, em ordem |

## Banco novo (do zero)

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/000_schema_base.sql
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/seed_configuracao.sql
# depois todas as migrations numeradas, em ordem (001, 002, 003...)
```

Testado num PostgreSQL 17 vazio: roda sem erro e a API funciona.

## Banco que já existe (produção)

Rode **só as migrations novas**, uma vez cada, em ordem (no SQL Editor do
Supabase ou com `psql`). Não rode o `000_schema_base.sql`.

| Migration | Aplicada na produção |
|---|---|
| 001_fix_trg_sync_espn_to_matches | ✅ |
| 002_remove_clubes_espn_duplicados | ✅ |
| 003_vincula_selecoes_unl_team_leagues | ✅ |
| 004_rls_e_indices | ✅ |
| 005_tabela_users | ✅ |
| 006_ids_proprios_selecoes_unl | ✅ |
| 007_corrige_temporadas_e_jogos_intrusos | ✅ |
| 008_busca_de_ligas | ✅ |
| 009_push_notificacoes | ✅ |
| 010_push_seguir_liga | ✅ |

## Testes de integração

Os testes de integração (notificações push, por exemplo) rodam contra um Postgres
de verdade com as migrations aplicadas. Sem `TEST_DATABASE_URL`, o `go test` os pula.

```bash
docker run -d --name pgteste -p 55436:5432 -e POSTGRES_PASSWORD=x postgres:17
# aplique o 000, o seed e as migrations numeradas (a 004 e a 005 dependem de
# papéis do Supabase e podem ser puladas num Postgres comum)
TEST_DATABASE_URL='postgres://postgres:x@localhost:55436/postgres?sslmode=disable' go test ./...
```

Os testes só aceitam banco local (`localhost`/`127.0.0.1`), porque apagam os dados
que criam.

## Criando uma mudança nova

1. Crie `migrations/NNN_descricao_curta.sql` com o próximo número.
2. Comente no topo **o porquê** da mudança.
3. Teste antes num banco descartável ou dentro de `BEGIN; ... ROLLBACK;`.
4. Aplique na produção e marque ✅ na tabela acima.
5. Faça o commit do arquivo junto com o código que depende dele.

## Atualizando o schema base

Se o `000_schema_base.sql` ficar muito defasado, gere de novo. O `pg_dump`
precisa ser da **mesma versão do servidor ou mais nova** (o Supabase usa a 17):

```bash
/usr/lib/postgresql/17/bin/pg_dump "$DATABASE_URL" --schema-only --schema=public \
  --no-owner --no-privileges --no-comments -f migrations/000_schema_base.sql
```
