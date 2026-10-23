package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"

	"github.com/lib/pq"
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

	// A tabela inteira num comando só (eram uma linha por comando, ~20 idas e voltas por
	// recálculo de cada liga)
	n := len(standings)
	positions, played, wins, draws, losses := make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n)
	goalsFor, goalsAgainst, goalDiff, points := make([]int64, n), make([]int64, n), make([]int64, n), make([]int64, n)
	teamIDs := make([]int64, n)
	zones, groups, forms := make([]string, n), make([]string, n), make([]string, n)
	for i, st := range standings {
		positions[i], teamIDs[i] = int64(st.Position), st.TeamID
		played[i], wins[i], draws[i], losses[i] = int64(st.Played), int64(st.Wins), int64(st.Draws), int64(st.Losses)
		goalsFor[i], goalsAgainst[i], goalDiff[i], points[i] = int64(st.GoalsFor), int64(st.GoalsAgainst), int64(st.GoalDiff), int64(st.Points)
		zones[i], groups[i], forms[i] = st.Zone, st.GroupName, st.Form
	}
	if _, err := tx.ExecContext(ctx, `
        INSERT INTO standings
        (league, position, team_id, played, wins, draws, losses, goals_for, goals_against, goal_diff, points, zone, season, group_name, form)
        SELECT $1, pos, team, pl, w, d, l, gf, ga, gd, pts, z, $2, g, f
        FROM unnest($3::int[], $4::bigint[], $5::int[], $6::int[], $7::int[], $8::int[], $9::int[], $10::int[],
                    $11::int[], $12::int[], $13::varchar[], $14::varchar[], $15::varchar[])
             AS s(pos, team, pl, w, d, l, gf, ga, gd, pts, z, g, f)`,
		league, season, pq.Array(positions), pq.Array(teamIDs), pq.Array(played), pq.Array(wins),
		pq.Array(draws), pq.Array(losses), pq.Array(goalsFor), pq.Array(goalsAgainst), pq.Array(goalDiff),
		pq.Array(points), pq.Array(zones), pq.Array(groups), pq.Array(forms)); err != nil {
		utils.CustomLog("DATABASE_ERRO", "Falha ao salvar a classificação de %s: %v", league, err)
		return err
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
        COALESCE(s.group_name, ''),
        COALESCE(s.form, '')
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
			&s.Form,
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
