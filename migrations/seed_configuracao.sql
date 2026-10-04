-- Dados de configuração sem os quais a API não funciona:
-- ligas (códigos football-data <-> ESPN), regras de zona, critérios de desempate,
-- nomes das zonas e campeões (vagas por título).
--
-- Exportado do banco de produção em 2026-10-03. Rode depois do 000_schema_base.sql.

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Data for Name: competition_rules; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.competition_rules (id, league, season, libertadores, pre_libertadores, sul_americana, rebaixamento) VALUES (1, 'PD', '2025-2026', 4, 1, 1, 3);
INSERT INTO public.competition_rules (id, league, season, libertadores, pre_libertadores, sul_americana, rebaixamento) VALUES (2, 'BSA', '2026', 4, 1, 6, 4);
INSERT INTO public.competition_rules (id, league, season, libertadores, pre_libertadores, sul_americana, rebaixamento) VALUES (3, 'PL', '2025-2026', 4, 1, 0, 3);
INSERT INTO public.competition_rules (id, league, season, libertadores, pre_libertadores, sul_americana, rebaixamento) VALUES (4, 'BL1', '2025-2026', 4, 1, 1, 2);


--
-- Data for Name: competition_tiebreakers; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (1, 'BSA', '2026', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (2, 'BSA', '2026', 2, 'wins');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (3, 'BSA', '2026', 3, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (4, 'BSA', '2026', 4, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (5, 'BSA', '2026', 5, 'head_to_head');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (6, 'BSA', '2026', 6, 'red_cards');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (7, 'BSA', '2026', 7, 'yellow_cards');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (8, 'PD', '2025-2026', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (9, 'PD', '2025-2026', 2, 'head_to_head');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (10, 'PD', '2025-2026', 3, 'head_to_head_goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (11, 'PD', '2025-2026', 4, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (12, 'PD', '2025-2026', 5, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (13, 'PL', '2025-2026', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (14, 'PL', '2025-2026', 2, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (15, 'PL', '2025-2026', 3, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (16, 'PL', '2025-2026', 4, 'head_to_head_points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (17, 'PL', '2025-2026', 5, 'head_to_head_away_goals');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (24, 'BL1', '2025-2026', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (25, 'BL1', '2025-2026', 2, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (26, 'BL1', '2025-2026', 3, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (27, 'BL1', '2025-2026', 4, 'head_to_head_points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (28, 'BL1', '2025-2026', 5, 'head_to_head_away_goals');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (29, 'BL1', '2025-2026', 6, 'away_goals_scored');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (30, 'BL1', '2025-2026', 7, 'playoff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (31, 'WC', '2026', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (32, 'WC', '2026', 2, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (33, 'WC', '2026', 3, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (34, 'WC', '2026', 4, 'wins');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (35, 'PD', '2026-2027', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (36, 'PD', '2026-2027', 2, 'head_to_head');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (37, 'PD', '2026-2027', 3, 'head_to_head_goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (38, 'PD', '2026-2027', 4, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (39, 'PD', '2026-2027', 5, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (40, 'PL', '2026-2027', 2, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (41, 'PL', '2026-2027', 3, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (42, 'PL', '2026-2027', 4, 'head_to_head_points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (43, 'PL', '2026-2027', 5, 'head_to_head_away_goals');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (44, 'BL1', '2026-2027', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (45, 'BL1', '2026-2027', 2, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (46, 'BL1', '2026-2027', 3, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (47, 'BL1', '2026-2027', 4, 'head_to_head_points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (48, 'BL1', '2026-2027', 5, 'head_to_head_away_goals');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (49, 'BL1', '2026-2027', 6, 'away_goals_scored');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (50, 'BL1', '2026-2027', 7, 'playoff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (51, 'UNL', '2026-2027', 1, 'points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (52, 'UNL', '2026-2027', 2, 'head_to_head_points');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (53, 'UNL', '2026-2027', 3, 'head_to_head_goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (54, 'UNL', '2026-2027', 4, 'head_to_head_goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (55, 'UNL', '2026-2027', 5, 'goal_diff');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (56, 'UNL', '2026-2027', 6, 'goals_for');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (57, 'UNL', '2026-2027', 7, 'away_goals_scored');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (58, 'UNL', '2026-2027', 8, 'wins');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (59, 'UNL', '2026-2027', 9, 'away_wins');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (60, 'UNL', '2026-2027', 10, 'fair_play');
INSERT INTO public.competition_tiebreakers (id, league, season, priority, criterion) VALUES (61, 'UNL', '2026-2027', 11, 'ranking');


--
-- Data for Name: competition_winners; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: competition_zones; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (1, 'BSA', 'lib', 'libertadores', 1);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (2, 'BSA', 'pre', 'pre_libertadores', 2);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (3, 'BSA', 'sul', 'sul_americana', 3);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (4, 'BSA', 'reb', 'rebaixamento', 4);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (5, 'PD', 'lib', 'champions_leugue', 1);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (6, 'PD', 'pre', 'europa_league', 2);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (7, 'PD', 'sul', 'conference_league', 3);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (8, 'PD', 'reb', 'rebaixamento', 4);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (9, 'PL', 'lib', 'champions_leugue', 1);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (10, 'PL', 'pre', 'europa_league', 2);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (11, 'PL', 'sul', 'conference_league', 0);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (12, 'PL', 'reb', 'rebaixamento', 3);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (17, 'BL1', 'lib', 'champions_leugue', 1);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (18, 'BL1', 'pre', 'europa_league', 2);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (19, 'BL1', 'sul', 'conference_league', 3);
INSERT INTO public.competition_zones (id, league, zone_key, zone_name, priority) VALUES (20, 'BL1', 'reb', 'rebaixamento', 4);


--
-- Data for Name: leagues; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('PL', 'eng.1', 'Premier League', 'https://a.espncdn.com/i/leaguelogos/soccer/500/23.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('PD', 'esp.1', 'La Liga', 'https://a.espncdn.com/i/leaguelogos/soccer/500/15.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('CL', 'uefa.champions', 'Liga dos Campeões da UEFA', 'https://a.espncdn.com/i/leaguelogos/soccer/500/2.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('BL1', 'ger.1', 'Bundesliga', 'https://a.espncdn.com/i/leaguelogos/soccer/500/10.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('SA', 'ita.1', 'Compeonato Italiano - Série A', 'https://a.espncdn.com/i/leaguelogos/soccer/500/12.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('FL1', 'fra.1', 'Ligue 1', 'https://a.espncdn.com/i/leaguelogos/soccer/500/9.png', 'european');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('BSA', 'bra.1', 'Brasileirão - Série A', 'https://a.espncdn.com/i/leaguelogos/soccer/500/85.png', 'calendar');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('CLI', 'conmebol.libertadores', 'Libertadores', 'https://a.espncdn.com/i/leaguelogos/soccer/500/58.png', 'calendar');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('CSU', 'conmebol.sudamericana', 'Sul-Americana', 'https://a.espncdn.com/i/leaguelogos/soccer/500/1208.png', 'calendar');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('WC', 'fifa.world', 'Copa do Mundo', 'https://a.espncdn.com/i/leaguelogos/soccer/500/4.png', 'calendar');
INSERT INTO public.leagues (code_api, code_espn, name, logo_url, season_format) VALUES ('UNL', 'uefa.nations', 'Liga das Nações UEFA', 'https://a.espncdn.com/i/leaguelogos/soccer/500/2395.png', 'european');


--
-- Name: competition_rules_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.competition_rules_id_seq', 4, true);


--
-- Name: competition_tiebreakers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.competition_tiebreakers_id_seq', 61, true);


--
-- Name: competition_winners_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.competition_winners_id_seq', 4, true);


--
-- Name: competition_zones_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.competition_zones_id_seq', 20, true);
