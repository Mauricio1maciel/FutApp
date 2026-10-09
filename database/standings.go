package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
)

// ReplaceStandings troca a classificação inteira numa única transação,
// para o app nunca ler a tabela vazia ou com linhas duplicadas.
func ReplaceStandings(ctx context.Context, league string, season string, standings []models.Standing) error {
	ctx, cancel := withTxTimeout(ctx)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Serializa recálculos simultâneos da mesma liga/temporada
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, league+"|"+season); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM standings WHERE league = $1 AND season = $2", league, season); err != nil {
		return err
	}

	for _, s := range standings {
		_, err := tx.ExecContext(ctx, `
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

func GetStandingsByLeague(ctx context.Context, league string, season string) ([]models.Standing, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
    SELECT 
        COALESCE(s.position, 0),
        COALESCE(s.team_id, 0), 
        COALESCE(t.name, ''),
        COALESCE(s.played, 0),
        COALESCE(s.wins, 0),
        COALESCE(s.draws, 0),
        COALESCE(s.losses, 0),
        COALESCE(s.goals_for, 0),
        COALESCE(s.goals_against, 0),
        COALESCE(s.goal_diff, 0),
        COALESCE(s.points, 0),
        COALESCE(s.season, ''),
        COALESCE(t.crest_url, ''),
        COALESCE(s.zone, ''),
        COALESCE(s.group_name, '')
    FROM standings s
    LEFT JOIN teams t ON s.team_id = t.api_id
    WHERE s.league = $1 AND s.season = $2
    ORDER BY s.group_name ASC, s.position ASC 
`, league, season)

	if err != nil {
		utils.CustomLog("DB_ERRO", "Erro na query GetStandingsByLeague: %v", err)
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
			utils.CustomLog("DB_ERRO", "Erro no Scan de GetStandingsByLeague: %v", err)
			return nil, err
		}
		standings = append(standings, s)
	}

	if standings == nil {
		standings = []models.Standing{}
	}
	return standings, nil
}
