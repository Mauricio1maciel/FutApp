package database

import (
	"App-Futebol/models"
	"context"
	"fmt"
)

func GetTeamsByLeague(ctx context.Context, league string) ([]models.Team, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	query := `
        SELECT 
            t.id, 
            t.api_id, 
            COALESCE(t.name, ''), 
            COALESCE(t.short, ''), 
            COALESCE(t.tla, ''),   
            COALESCE(tl.league, ''), 
            COALESCE(t.stadium, ''), 
            COALESCE(t.crest_url, '') 
        FROM teams t
        INNER JOIN team_leagues tl ON t.api_id = tl.team_api_id
        WHERE tl.league = $1
        ORDER BY t.name ASC
    `

	rows, err := DB.QueryContext(ctx, query, league)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team

	for rows.Next() {
		var t models.Team

		err := rows.Scan(
			&t.ID,
			&t.ApiID,
			&t.Name,
			&t.Short,
			&t.TLA,
			&t.League,
			&t.Stadium,
			&t.Crest,
		)

		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}

	if teams == nil {
		teams = []models.Team{}
	}

	return teams, nil
}

// ESPNOnlyTeamIDOffset é a faixa de api_id dos times que só existem na ESPN (seleções
// da UNL). Fica muito acima dos IDs da football-data, então os dois nunca colidem.
// O trigger trg_sync_espn_to_matches usa o mesmo valor (migration 006).
const ESPNOnlyTeamIDOffset int64 = 1_000_000_000

func SaveTeam(ctx context.Context, apiID int64, name string, short string, tla string, league string, stadium string, crest string, season string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := DB.ExecContext(ctx,
		`INSERT INTO teams (api_id, name, short, tla, stadium, crest_url) 
         VALUES ($1, $2, $3, $4, $5, $6)
         ON CONFLICT (api_id) 
         DO UPDATE SET
            name = EXCLUDED.name,
            short = EXCLUDED.short,
            tla = EXCLUDED.tla,
            stadium = EXCLUDED.stadium,
            crest_url = EXCLUDED.crest_url`,
		apiID, name, short, tla, stadium, crest,
	)

	if err != nil {
		fmt.Printf("ERRO NO BANCO AO SALVAR TIME %d: %v\n", apiID, err)
		return err
	}
	_, err = DB.ExecContext(ctx,
		`INSERT INTO team_leagues (team_api_id, league, season) 
         VALUES ($1, $2, $3)
         ON CONFLICT (team_api_id, league, season) DO NOTHING`,
		apiID, league, season,
	)

	if err != nil {
		fmt.Printf("ERRO AO VINCULAR TIME %d NA LIGA %s: %v\n", apiID, league, err)
	}

	return err
}
