package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
)

// ReplaceStandings troca a classificação inteira numa única transação,
// para o app nunca ler a tabela vazia ou com linhas duplicadas.
func ReplaceStandings(league string, season string, standings []models.Standing) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Serializa recálculos simultâneos da mesma liga/temporada
	if _, err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext($1))`, league+"|"+season); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM standings WHERE league = $1 AND season = $2", league, season); err != nil {
		return err
	}

	for _, s := range standings {
		_, err := tx.Exec(`
            INSERT INTO standings
            (league, position, team_id, played, wins, draws, losses, goals_for, goals_against, goal_diff, points, zone, season, group_name)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
        `,
			league,
			s.Position,
			s.TeamID,
			s.Played,
			s.Wins,
			s.Draws,
			s.Losses,
			s.GoalsFor,
			s.GoalsAgainst,
			s.GoalDiff,
			s.Points,
			s.Zone,
			season,
			s.GroupName,
		)

		if err != nil {
			utils.CustomLog("DATABASE_ERRO", "Falha ao salvar time ID %d na tabela standings: %v", s.TeamID, err)
			return err
		}
	}

	return tx.Commit()
}

func GetStandingsByLeague(league string, season string) ([]models.Standing, error) {

	rows, err := DB.Query(`
    SELECT 
        s.position,
        s.team_id, 
        COALESCE(t.name, ''),
        s.played,
        s.wins,
        s.draws,
        s.losses,
        s.goals_for,
        s.goals_against,
        s.goal_diff,
        s.points,
        s.season,
        COALESCE(t.crest_url, ''),
        COALESCE(s.zone, ''),
        COALESCE(s.group_name, '')
    FROM standings s
    LEFT JOIN teams t ON s.team_id = t.api_id
    WHERE s.league = $1 AND s.season = $2
    ORDER BY s.group_name ASC, s.position ASC 
`, league, season)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var standings []models.Standing

	for rows.Next() {
		var s models.Standing

		err := rows.Scan(
			&s.Position,
			&s.TeamID,
			&s.TeamName,
			&s.Played,
			&s.Wins,
			&s.Draws,
			&s.Losses,
			&s.GoalsFor,
			&s.GoalsAgainst,
			&s.GoalDiff,
			&s.Points,
			&s.Season,
			&s.CrestURL,
			&s.Zone,
			&s.GroupName,
		)

		if err != nil {
			return nil, err
		}
		standings = append(standings, s)
	}

	if standings == nil {
		standings = []models.Standing{}
	}
	return standings, nil
}
