-- Schema base do banco (tabelas, constraints, índices, funções e triggers).
--
-- Gerado com pg_dump 17 a partir do banco de produção em 2026-10-03 e já
-- inclui o efeito das migrations 001 e 002. Num banco NOVO, rode este arquivo
-- e depois seed_configuracao.sql. Num banco que já existe, NÃO rode: use só as
-- migrations novas (003 em diante). Veja migrations/README.md.

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', 'public, extensions', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

-- Extensões usadas pela API (unaccent: busca de times/jogadores sem acento)
CREATE EXTENSION IF NOT EXISTS unaccent;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA IF NOT EXISTS public;


--
-- Name: log_player_transfer(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.log_player_transfer() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Verifica se o ID do time realmente mudou
    IF OLD.team_id IS DISTINCT FROM NEW.team_id THEN
        INSERT INTO player_history (
            player_api_id,
            player_name,
            old_team_id,
            old_team_name,
            new_team_id,
            new_team_name
        ) VALUES (
            OLD.api_id,
            OLD.name,
            OLD.team_id,
            OLD.team_name,
            NEW.team_id,
            NEW.team_name
        );
    END IF;
    
    RETURN NEW;
END;
$$;


--
-- Name: trg_sync_espn_to_matches(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trg_sync_espn_to_matches() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    real_home_id BIGINT;
    real_away_id BIGINT;
    normalized_status VARCHAR(50);
BEGIN
    -- 1. Pega o ID oficial da football-data (se existir), senão usa o da ESPN como fallback
    SELECT api_id INTO real_home_id FROM teams WHERE espn_team_id = NEW.espn_home_team_id LIMIT 1;
    IF real_home_id IS NULL THEN real_home_id := NEW.espn_home_team_id; END IF;

    SELECT api_id INTO real_away_id FROM teams WHERE espn_team_id = NEW.espn_away_team_id LIMIT 1;
    IF real_away_id IS NULL THEN real_away_id := NEW.espn_away_team_id; END IF;

    -- 2. Traduz o status da ESPN para o padrão da football-data
    IF NEW.status = 'post' THEN
        normalized_status := 'FINISHED';
    ELSIF NEW.status = 'in' THEN
        normalized_status := 'IN_PLAY';
    ELSE
        normalized_status := 'TIMED';
    END IF;

    -- 3. Liga das Nações (UNL): a football-data não cobre, então o jogo é criado a partir da ESPN
    IF NEW.league = 'UNL' THEN
        INSERT INTO matches (
            id_event, league, season, match_date, status,
            home_score, away_score,
            api_home_team_id, api_away_team_id,
            group_name, stage, round
        )
        VALUES (
            NEW.espn_match_id, NEW.league, NEW.season, NEW.match_date, normalized_status,
            CAST(NULLIF(NEW.home_score, '') AS INTEGER),
            CAST(NULLIF(NEW.away_score, '') AS INTEGER),
            real_home_id, real_away_id,
            NEW.group_name, NEW.stage, 1
        )
        ON CONFLICT (id_event) DO UPDATE
        SET status = EXCLUDED.status,
            home_score = EXCLUDED.home_score,
            away_score = EXCLUDED.away_score,
            match_date = EXCLUDED.match_date,
            season = EXCLUDED.season,
            league = EXCLUDED.league,
            api_home_team_id = EXCLUDED.api_home_team_id,
            api_away_team_id = EXCLUDED.api_away_team_id,
            group_name = COALESCE(NULLIF(EXCLUDED.group_name, ''), matches.group_name),
            stage = COALESCE(NULLIF(EXCLUDED.stage, ''), matches.stage);

    -- 4. Demais ligas: o jogo vem da football-data. A ESPN só antecipa placar ao vivo/final.
    ELSIF NEW.status IN ('in', 'post')
          AND NULLIF(NEW.home_score, '') IS NOT NULL
          AND NULLIF(NEW.away_score, '') IS NOT NULL THEN
        UPDATE matches m
        SET status = normalized_status,
            home_score = CAST(NEW.home_score AS INTEGER),
            away_score = CAST(NEW.away_score AS INTEGER)
        WHERE m.api_home_team_id = real_home_id
          AND m.api_away_team_id = real_away_id
          AND m.match_date::DATE = NEW.match_date::DATE
          AND m.status NOT IN ('POSTPONED', 'SUSPENDED', 'CANCELLED', 'CANCELED');
    END IF;

    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: competition_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.competition_rules (
    id integer NOT NULL,
    league character varying(10),
    season character varying(9),
    libertadores integer,
    pre_libertadores integer,
    sul_americana integer,
    rebaixamento integer
);


--
-- Name: competition_rules_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.competition_rules_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: competition_rules_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.competition_rules_id_seq OWNED BY public.competition_rules.id;


--
-- Name: competition_tiebreakers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.competition_tiebreakers (
    id integer NOT NULL,
    league character varying(10),
    season character varying(9),
    priority integer,
    criterion character varying(50)
);


--
-- Name: competition_tiebreakers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.competition_tiebreakers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: competition_tiebreakers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.competition_tiebreakers_id_seq OWNED BY public.competition_tiebreakers.id;


--
-- Name: competition_winners; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.competition_winners (
    id integer NOT NULL,
    season character varying(9),
    competition character varying(50),
    team_name character varying(100),
    league character varying(10)
);


--
-- Name: competition_winners_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.competition_winners_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: competition_winners_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.competition_winners_id_seq OWNED BY public.competition_winners.id;


--
-- Name: competition_zones; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.competition_zones (
    id integer NOT NULL,
    league character varying(10),
    zone_key character varying(30),
    zone_name character varying(50),
    priority integer
);


--
-- Name: competition_zones_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.competition_zones_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: competition_zones_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.competition_zones_id_seq OWNED BY public.competition_zones.id;


--
-- Name: espn_match_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.espn_match_events (
    id integer NOT NULL,
    espn_match_id bigint,
    minute character varying(20),
    event_type character varying(50),
    player_name character varying(100),
    details text,
    espn_team_id bigint
);


--
-- Name: espn_match_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.espn_match_events_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: espn_match_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.espn_match_events_id_seq OWNED BY public.espn_match_events.id;


--
-- Name: espn_match_lineups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.espn_match_lineups (
    id integer NOT NULL,
    espn_match_id bigint,
    player_name character varying(100),
    jersey character varying(10),
    "position" character varying(20),
    is_starter boolean,
    formation character varying(20),
    espn_team_id bigint,
    espn_player_id bigint
);


--
-- Name: espn_match_lineups_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.espn_match_lineups_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: espn_match_lineups_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.espn_match_lineups_id_seq OWNED BY public.espn_match_lineups.id;


--
-- Name: espn_matches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.espn_matches (
    espn_match_id bigint NOT NULL,
    match_date timestamp without time zone,
    home_score character varying(10),
    away_score character varying(10),
    status character varying(50),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    league character varying(10),
    away_logo text,
    home_logo text,
    espn_away_team_id bigint,
    espn_home_team_id bigint,
    group_name character varying(50),
    stage character varying(50),
    home_penalty integer,
    away_penalty integer,
    season character varying(20)
);


--
-- Name: espn_players; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.espn_players (
    id integer NOT NULL,
    espn_id bigint NOT NULL,
    name character varying(150) NOT NULL,
    short_name character varying(100),
    "position" character varying(50),
    jersey_number integer,
    headshot_url text,
    espn_team_id bigint,
    nationality character varying(100),
    date_of_birth character varying(50),
    market_value character varying(50),
    last_updated timestamp without time zone DEFAULT now()
);


--
-- Name: espn_players_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.espn_players_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: espn_players_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.espn_players_id_seq OWNED BY public.espn_players.id;


--
-- Name: leagues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.leagues (
    code_api character varying(10) NOT NULL,
    code_espn character varying(50),
    name character varying(100),
    logo_url text,
    season_format character varying(20) DEFAULT 'european'::character varying
);


--
-- Name: matches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.matches (
    id integer NOT NULL,
    id_event bigint,
    league character varying(50),
    season character varying(20),
    round integer,
    home_score integer,
    away_score integer,
    match_date timestamp without time zone,
    status character varying(20),
    api_away_team_id bigint,
    api_home_team_id bigint,
    group_name character varying(50),
    stage character varying(50),
    home_penalty integer,
    away_penalty integer,
    winner character varying(20) DEFAULT ''::character varying
);


--
-- Name: matches_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.matches_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: matches_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.matches_id_seq OWNED BY public.matches.id;


--
-- Name: player_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.player_history (
    id integer NOT NULL,
    player_api_id integer NOT NULL,
    player_name character varying(255) NOT NULL,
    old_team_id integer,
    old_team_name character varying(255),
    new_team_id integer,
    new_team_name character varying(255),
    transfer_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: player_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.player_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: player_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.player_history_id_seq OWNED BY public.player_history.id;


--
-- Name: player_stats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.player_stats (
    espn_player_id bigint NOT NULL,
    league character varying(20) NOT NULL,
    season character varying(10) NOT NULL,
    goals integer DEFAULT 0,
    assists integer DEFAULT 0,
    matches integer DEFAULT 0,
    espn_team_id bigint
);


--
-- Name: players; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.players (
    id integer NOT NULL,
    api_id bigint NOT NULL,
    name character varying(255) NOT NULL,
    "position" character varying(100),
    date_of_birth date,
    nationality character varying(100),
    team_id bigint NOT NULL,
    team_name character varying(255) NOT NULL,
    league character varying(50) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: players_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.players_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: players_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.players_id_seq OWNED BY public.players.id;


--
-- Name: standings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.standings (
    id integer NOT NULL,
    league character varying(20),
    "position" integer,
    points integer,
    played integer,
    wins integer,
    draws integer,
    losses integer,
    goals_for integer,
    goals_against integer,
    goal_diff integer,
    zone character varying(100),
    season character varying(9),
    team_id bigint,
    group_name character varying(50)
);


--
-- Name: standings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.standings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: standings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.standings_id_seq OWNED BY public.standings.id;


--
-- Name: team_leagues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.team_leagues (
    team_api_id bigint NOT NULL,
    league character varying(20) NOT NULL,
    season character varying(20) NOT NULL
);


--
-- Name: teams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.teams (
    id integer NOT NULL,
    api_id bigint,
    name character varying(100),
    league character varying(100),
    stadium character varying(100),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    crest_url text,
    short character varying(50),
    tla character varying(50),
    espn_team_id bigint
);


--
-- Name: teams_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.teams_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: teams_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.teams_id_seq OWNED BY public.teams.id;


--
-- Name: competition_rules id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_rules ALTER COLUMN id SET DEFAULT nextval('competition_rules_id_seq'::regclass);


--
-- Name: competition_tiebreakers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_tiebreakers ALTER COLUMN id SET DEFAULT nextval('competition_tiebreakers_id_seq'::regclass);


--
-- Name: competition_winners id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_winners ALTER COLUMN id SET DEFAULT nextval('competition_winners_id_seq'::regclass);


--
-- Name: competition_zones id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_zones ALTER COLUMN id SET DEFAULT nextval('competition_zones_id_seq'::regclass);


--
-- Name: espn_match_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_events ALTER COLUMN id SET DEFAULT nextval('espn_match_events_id_seq'::regclass);


--
-- Name: espn_match_lineups id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_lineups ALTER COLUMN id SET DEFAULT nextval('espn_match_lineups_id_seq'::regclass);


--
-- Name: espn_players id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_players ALTER COLUMN id SET DEFAULT nextval('espn_players_id_seq'::regclass);


--
-- Name: matches id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.matches ALTER COLUMN id SET DEFAULT nextval('matches_id_seq'::regclass);


--
-- Name: player_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_history ALTER COLUMN id SET DEFAULT nextval('player_history_id_seq'::regclass);


--
-- Name: players id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.players ALTER COLUMN id SET DEFAULT nextval('players_id_seq'::regclass);


--
-- Name: standings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.standings ALTER COLUMN id SET DEFAULT nextval('standings_id_seq'::regclass);


--
-- Name: teams id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams ALTER COLUMN id SET DEFAULT nextval('teams_id_seq'::regclass);


--
-- Name: competition_rules competition_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_rules
    ADD CONSTRAINT competition_rules_pkey PRIMARY KEY (id);


--
-- Name: competition_tiebreakers competition_tiebreakers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_tiebreakers
    ADD CONSTRAINT competition_tiebreakers_pkey PRIMARY KEY (id);


--
-- Name: competition_winners competition_winners_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_winners
    ADD CONSTRAINT competition_winners_pkey PRIMARY KEY (id);


--
-- Name: competition_zones competition_zones_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.competition_zones
    ADD CONSTRAINT competition_zones_pkey PRIMARY KEY (id);


--
-- Name: espn_match_events espn_match_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_events
    ADD CONSTRAINT espn_match_events_pkey PRIMARY KEY (id);


--
-- Name: espn_match_lineups espn_match_lineups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_lineups
    ADD CONSTRAINT espn_match_lineups_pkey PRIMARY KEY (id);


--
-- Name: espn_matches espn_matches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_matches
    ADD CONSTRAINT espn_matches_pkey PRIMARY KEY (espn_match_id);


--
-- Name: espn_players espn_players_espn_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_players
    ADD CONSTRAINT espn_players_espn_id_key UNIQUE (espn_id);


--
-- Name: espn_players espn_players_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_players
    ADD CONSTRAINT espn_players_pkey PRIMARY KEY (id);


--
-- Name: leagues leagues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.leagues
    ADD CONSTRAINT leagues_pkey PRIMARY KEY (code_api);


--
-- Name: matches matches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.matches
    ADD CONSTRAINT matches_pkey PRIMARY KEY (id);


--
-- Name: player_history player_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_history
    ADD CONSTRAINT player_history_pkey PRIMARY KEY (id);


--
-- Name: player_stats player_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.player_stats
    ADD CONSTRAINT player_stats_pkey PRIMARY KEY (espn_player_id, league, season);


--
-- Name: players players_api_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.players
    ADD CONSTRAINT players_api_id_key UNIQUE (api_id);


--
-- Name: players players_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.players
    ADD CONSTRAINT players_pkey PRIMARY KEY (id);


--
-- Name: standings standings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.standings
    ADD CONSTRAINT standings_pkey PRIMARY KEY (id);


--
-- Name: team_leagues team_leagues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.team_leagues
    ADD CONSTRAINT team_leagues_pkey PRIMARY KEY (team_api_id, league, season);


--
-- Name: teams teams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_pkey PRIMARY KEY (id);


--
-- Name: teams unique_espn_team_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT unique_espn_team_id UNIQUE (espn_team_id);


--
-- Name: matches unique_id_event; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.matches
    ADD CONSTRAINT unique_id_event UNIQUE (id_event);


--
-- Name: matches unique_match; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.matches
    ADD CONSTRAINT unique_match UNIQUE (id_event, league);


--
-- Name: teams unique_team_api_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT unique_team_api_id UNIQUE (api_id);


--
-- Name: teams unique_team_league; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT unique_team_league UNIQUE (api_id, league);


--
-- Name: idx_espn_events_match_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_espn_events_match_id ON public.espn_match_events USING btree (espn_match_id);


--
-- Name: idx_espn_lineups_match_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_espn_lineups_match_id ON public.espn_match_lineups USING btree (espn_match_id);


--
-- Name: idx_espn_matches_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_espn_matches_id ON public.espn_matches USING btree (espn_match_id);


--
-- Name: idx_espn_matches_league; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_espn_matches_league ON public.espn_matches USING btree (league);


--
-- Name: idx_espn_players_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_espn_players_team ON public.espn_players USING btree (espn_team_id);


--
-- Name: idx_leagues_code_api; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_leagues_code_api ON public.leagues USING btree (code_api);


--
-- Name: idx_matches_league; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_matches_league ON public.matches USING btree (league);


--
-- Name: idx_player_history_api_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_player_history_api_id ON public.player_history USING btree (player_api_id);


--
-- Name: idx_players_last_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_players_last_updated ON public.espn_players USING btree (last_updated);


--
-- Name: idx_players_league; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_players_league ON public.players USING btree (league);


--
-- Name: idx_players_team_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_players_team_id ON public.players USING btree (team_id);


--
-- Name: idx_standings_league_season; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_standings_league_season ON public.standings USING btree (league, season);


--
-- Name: espn_matches after_espn_matches_upsert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER after_espn_matches_upsert AFTER INSERT OR UPDATE ON public.espn_matches FOR EACH ROW EXECUTE FUNCTION trg_sync_espn_to_matches();


--
-- Name: players trigger_player_transfer; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trigger_player_transfer AFTER UPDATE OF team_id ON public.players FOR EACH ROW EXECUTE FUNCTION log_player_transfer();


--
-- Name: espn_match_events espn_match_events_espn_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_events
    ADD CONSTRAINT espn_match_events_espn_match_id_fkey FOREIGN KEY (espn_match_id) REFERENCES espn_matches(espn_match_id) ON DELETE CASCADE;


--
-- Name: espn_match_lineups espn_match_lineups_espn_match_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.espn_match_lineups
    ADD CONSTRAINT espn_match_lineups_espn_match_id_fkey FOREIGN KEY (espn_match_id) REFERENCES espn_matches(espn_match_id) ON DELETE CASCADE;
