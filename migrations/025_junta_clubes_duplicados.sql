-- Junta clubes que estavam em dois cadastros: o da football-data (histórico do Brasileirão
-- e da Libertadores, carregado depois que esses clubes saíram das listas atuais da ESPN, e
-- por isso sem ligação com ela) e o que a ESPN criou para a Série B, a Copa do Brasil e a
-- Sul-Americana de 2026. A página do time ficava dividida e o confronto direto saía
-- incompleto (ex: o Sport da Série B 2026 e o "SC Recife" do Brasileirão 2025).
--
-- Os pares NÃO saíram de comparação de nomes, que errava ("América FC" casava com o
-- América SE, "Club Nacional" com o Atlético Nacional): para cada clube, dois jogos já
-- encerrados foram achados na ESPN (mesma data, mesmo adversário, mesmo placar), e o ID
-- da ESPN do clube no jogo é o par. 53 clubes confirmados assim em 19/10/2026.
--
-- Fica o cadastro da football-data (o ID que ela reconhece se o clube voltar à Série A),
-- com o nome da ESPN (mais conhecido: "Sport", não "SC Recife") e o escudo da ESPN se
-- faltar o dele. Jogos, ligas, classificação, seguidores, jogadores e transferências
-- passam para ele; o cadastro criado pela ESPN sai. Os clubes sem par só ganham a
-- ligação com a ESPN, para não duplicarem se um dia entrarem numa liga que vem dela.
--
-- Pode rodar de novo sem efeito.

BEGIN;

CREATE TEMP TABLE juntar (fd_id BIGINT, espn_api_id BIGINT, espn_id BIGINT) ON COMMIT DROP;
INSERT INTO juntar VALUES
    (3988, 1000010357, 10357),  -- AC Goianiense = Atlético Goianiense
    (5162, 1000018995, 18995),  -- Academia Puerto Cabello = Academia Puerto Cabello
    (1838, 1000006154, 6154),  -- América FC = América Mineiro
    (5181, 1000009999, 9999),  -- CA Boston River = Boston River
    (4430, 1000006137, 6137),  -- CA Bucaramanga = Atlético Bucaramanga
    (6667, 1000000016, 16),  -- CA River Plate = River Plate
    (2071, 1000000018, 18),  -- CA San Lorenzo de Almagro = San Lorenzo
    (5167, 1000004811, 4811),  -- Caracas FC = Caracas FC
    (4259, 1000006047, 6047),  -- CD Blooming = Blooming
    (7913, 1000004133, 4133),  -- CD Cobresal = Cobresal
    (4460, 1000004422, 4422),  -- CD Palestino = Palestino
    (4438, 1000005264, 5264),  -- CDC Atlético Nacional = Atlético Nacional
    (1837, 1000009969, 9969),  -- Ceará SC = Ceará
    (4466, 1000004139, 4139),  -- CF Universidad de Chile = Universidad de Chile
    (5188, 1000005584, 5584),  -- Club Nacional = Nacional Asunción
    (4243, 1000009971, 9971),  -- Criciúma EC = Criciúma
    (4289, 1000017313, 17313),  -- Cuiabá EC = Cuiabá
    (5190, 1000001007, 1007),  -- Defensor SC = Defensor Sporting
    (4245, 1000006270, 6270),  -- EC Juventude = Juventude
    (5682, 1000007312, 7312),  -- FBC Melgar = Melgar
    (3984, 1000006272, 6272),  -- Fortaleza EC = Fortaleza
    (4250, 1000003395, 3395),  -- Goiás EC = Goiás
    (5174, 1000013481, 13481),  -- Metropolitanos FC = Metropolitanos
    (4444, 1000005484, 5484),  -- Millonarios FC = Millonarios
    (5175, 1000006041, 6041),  -- Monagas SC = Monagas SC
    (9372, 1000002675, 2675),  -- Olimpia Asuncion = Club Olimpia
    (6803, 1000000015, 15),  -- Racing Club = Racing Club
    (11268, 1000022137, 22137),  -- San Antonio Bulo Bulo = San Antonio Bulo Bulo
    (1778, 1000007635, 7635),  -- SC Recife = Sport
    (10359, 1000007466, 7466);  -- Sportivo Trinidense = Sportivo Trinidense

CREATE TEMP TABLE ligar (fd_id BIGINT, espn_id BIGINT) ON COMMIT DROP;
INSERT INTO ligar VALUES
    (7582, 11989),  -- CA Central Córdoba
    (2064, 10),  -- CA Huracán
    (2068, 10374),  -- CA Patronato
    (2073, 19),  -- CA Talleres
    (2078, 21),  -- CA Vélez Sarsfield
    (2079, 6756),  -- CD Godoy Cruz Antonio Tomba
    (4458, 10142),  -- CD Iquique
    (8777, 10000),  -- CD Maldonado
    (9362, 7427),  -- CD Nublense
    (4465, 17365),  -- CDP Curicó Unido
    (5680, 2680),  -- Club Alianza Lima
    (4260, 6046),  -- Club Aurora
    (4267, 2687),  -- Club The Strongest
    (4524, 882),  -- CSCyD El Nacional
    (4410, 2688),  -- CSD Colo-Colo
    (5350, 10318),  -- CSD Sport Huancayo
    (8602, 5486),  -- Deportivo Pereira
    (10137, 17363),  -- Magallanes
    (5176, 6762),  -- Portuguesa FC
    (4445, 9762),  -- Rionegro Águilas
    (4529, 6017),  -- SD Aucas
    (9377, 9283),  -- Universidad Catolica
    (5178, 6763);  -- Zamora FC

UPDATE matches m SET api_home_team_id = j.fd_id FROM juntar j WHERE m.api_home_team_id = j.espn_api_id;
UPDATE matches m SET api_away_team_id = j.fd_id FROM juntar j WHERE m.api_away_team_id = j.espn_api_id;

INSERT INTO team_leagues (team_api_id, league, season)
SELECT j.fd_id, tl.league, tl.season FROM team_leagues tl JOIN juntar j ON j.espn_api_id = tl.team_api_id
ON CONFLICT DO NOTHING;
DELETE FROM team_leagues tl USING juntar j WHERE tl.team_api_id = j.espn_api_id;

UPDATE standings s SET team_id = j.fd_id FROM juntar j WHERE s.team_id = j.espn_api_id;

-- Seguidores: a ligação apaga em cascata, então passam antes do cadastro sair
INSERT INTO team_subscriptions (device_id, team_api_id, kinds)
SELECT s.device_id, j.fd_id, s.kinds FROM team_subscriptions s JOIN juntar j ON j.espn_api_id = s.team_api_id
ON CONFLICT DO NOTHING;
DELETE FROM team_subscriptions s USING juntar j WHERE s.team_api_id = j.espn_api_id;

UPDATE players p SET team_id = j.fd_id FROM juntar j WHERE p.team_id = j.espn_api_id;
UPDATE player_history h SET old_team_id = j.fd_id FROM juntar j WHERE h.old_team_id = j.espn_api_id;
UPDATE player_history h SET new_team_id = j.fd_id FROM juntar j WHERE h.new_team_id = j.espn_api_id;

UPDATE teams f SET name = e.name, crest_url = COALESCE(NULLIF(f.crest_url, ''), e.crest_url)
FROM juntar j JOIN teams e ON e.api_id = j.espn_api_id
WHERE f.api_id = j.fd_id;
UPDATE teams e SET espn_team_id = NULL FROM juntar j WHERE e.api_id = j.espn_api_id;
UPDATE teams f SET espn_team_id = j.espn_id FROM juntar j
WHERE f.api_id = j.fd_id AND f.espn_team_id IS DISTINCT FROM j.espn_id;
DELETE FROM teams e USING juntar j WHERE e.api_id = j.espn_api_id;

UPDATE teams f SET espn_team_id = l.espn_id FROM ligar l
WHERE f.api_id = l.fd_id AND f.espn_team_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM teams o WHERE o.espn_team_id = l.espn_id);

COMMIT;
