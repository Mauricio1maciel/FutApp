package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

func SaveFullMatchHistoryold(match models.ESPNMatchDB, lineups []models.ESPNLineupDB, events []models.ESPNEventDB) error {
	utils.CustomLog("DATABASE", "Iniciando persistência da partida ESPN ID: %s", match.MatchID)

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO espn_matches (espn_match_id, league, match_date, home_logo, espn_home_team_id, away_logo, espn_away_team_id, home_score, away_score, status) 
         VALUES ($1::BIGINT, $2, NULLIF($3, '')::TIMESTAMP, $4, $5::BIGINT, $6, $7::BIGINT, $8, $9, $10)
         ON CONFLICT (espn_match_id) DO UPDATE 
         SET home_score = EXCLUDED.home_score,
             away_score = EXCLUDED.away_score,
             status = EXCLUDED.status,
             league = EXCLUDED.league`,
		match.MatchID, match.League, match.MatchDate,
		match.HomeLogo, match.ESPNHomeTeamID,
		match.AwayLogo, match.ESPNAwayTeamID,
		match.HomeScore, match.AwayScore, match.Status,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Exec(`DELETE FROM espn_match_lineups WHERE espn_match_id = $1::BIGINT`, match.MatchID)
	tx.Exec(`DELETE FROM espn_match_events WHERE espn_match_id = $1::BIGINT`, match.MatchID)

	for _, l := range lineups {
		_, err = tx.Exec(
			`INSERT INTO espn_match_lineups (espn_match_id, espn_team_id, espn_player_id, player_name, jersey, position, is_starter, formation)
             VALUES ($1::BIGINT, $2::BIGINT, $3::BIGINT, $4, $5, $6, $7, $8)`,
			l.MatchID, l.ESPNTeamID, l.ESPNPlayerID, l.PlayerName, l.Jersey, l.Position, l.IsStarter, l.Formation,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, e := range events {
		_, err = tx.Exec(
			`INSERT INTO espn_match_events (espn_match_id, minute, event_type, espn_team_id, player_name, details)
             VALUES ($1::BIGINT, $2, $3, $4::BIGINT, $5, $6)`,
			e.MatchID, e.Minute, e.EventType, e.ESPNTeamID, e.PlayerName, e.Details,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
