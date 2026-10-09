package database

import (
	"App-Futebol/models"
	"App-Futebol/utils"
	"context"
)

// GetLeaguesForTeamSync devolve as ligas (code_api -> code_espn) cobertas pela football-data.
// A UNL fica de fora: lá os times já são cadastrados com o próprio ID da ESPN.
func GetLeaguesForTeamSync(ctx context.Context) (map[string]string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `SELECT code_api, code_espn FROM leagues WHERE COALESCE(code_espn, '') <> '' AND code_api <> 'UNL'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leagues := make(map[string]string)
	for rows.Next() {
		var codeAPI, codeESPN string
		if err := rows.Scan(&codeAPI, &codeESPN); err != nil {
			return nil, err
		}
		leagues[codeAPI] = codeESPN
	}
	return leagues, rows.Err()
}

// LinkESPNTeams vincula os times da football-data de uma liga (ainda sem espn_team_id)
// aos times da ESPN da mesma liga. Só vincula quando há exatamente um candidato,
// e pula (com log) IDs da ESPN que já pertencem a outro time, em vez de abortar tudo.
func LinkESPNTeams(ctx context.Context, league string, espnTeams []models.ESPNTeam) (int, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
		SELECT DISTINCT t.api_id, COALESCE(t.name, ''), COALESCE(t.short, '')
		FROM teams t
		JOIN team_leagues tl ON tl.team_api_id = t.api_id
		WHERE tl.league = $1 AND t.espn_team_id IS NULL`, league)
	if err != nil {
		return 0, err
	}

	type DBTeam struct {
		ApiID int64
		Name  string
		Short string
	}
	var apiTeams []DBTeam
	for rows.Next() {
		var t DBTeam
		if err := rows.Scan(&t.ApiID, &t.Name, &t.Short); err != nil {
			rows.Close()
			return 0, err
		}
		apiTeams = append(apiTeams, t)
	}
	rows.Close()

	if len(apiTeams) == 0 {
		return 0, nil
	}

	// IDs da ESPN que já têm dono na tabela teams (UNIQUE espn_team_id)
	owners := make(map[int64]int64)
	ownerRows, err := DB.QueryContext(ctx, `SELECT espn_team_id, api_id FROM teams WHERE espn_team_id IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	for ownerRows.Next() {
		var espnID, apiID int64
		if err := ownerRows.Scan(&espnID, &apiID); err != nil {
			ownerRows.Close()
			return 0, err
		}
		owners[espnID] = apiID
	}
	ownerRows.Close()

	linked := 0
	for _, apiTeam := range apiTeams {
		espnTeam, ok := findESPNTeam(apiTeam.Name, apiTeam.Short, espnTeams)
		if !ok {
			utils.CustomLog("SYNC_TEAMS", "[%s] Sem correspondência única na ESPN para %s (%d)", league, apiTeam.Name, apiTeam.ApiID)
			continue
		}

		if owner, taken := owners[espnTeam.ID]; taken {
			utils.CustomLog("SYNC_TEAMS", "[%s] ⚠️ %s (%d) corresponde a ESPN %s (%d), mas esse ID já pertence ao time %d. Pulando.",
				league, apiTeam.Name, apiTeam.ApiID, espnTeam.DisplayName, espnTeam.ID, owner)
			continue
		}

		if _, err := DB.ExecContext(ctx, `UPDATE teams SET espn_team_id = $1 WHERE api_id = $2`, espnTeam.ID, apiTeam.ApiID); err != nil {
			utils.CustomLog("SYNC_TEAMS", "[%s] Falha ao vincular %s (%d): %v", league, apiTeam.Name, apiTeam.ApiID, err)
			continue
		}

		owners[espnTeam.ID] = apiTeam.ApiID
		linked++
		utils.CustomLog("SYNC_TEAMS", "[%s] 🔥 %s (%d) <--> %s (%d)", league, apiTeam.Name, apiTeam.ApiID, espnTeam.DisplayName, espnTeam.ID)
	}

	return linked, nil
}

// findESPNTeam procura primeiro nome idêntico (normalizado) e, se não achar,
// nome contido no outro. Em ambos os casos exige um único candidato.
func findESPNTeam(name, short string, espnTeams []models.ESPNTeam) (models.ESPNTeam, bool) {
	matches := func(cmp func(a, b string) bool) []models.ESPNTeam {
		var found []models.ESPNTeam
		for _, e := range espnTeams {
			for _, fdName := range []string{name, short} {
				if fdName == "" {
					continue
				}
				if cmp(fdName, e.DisplayName) || (e.ShortName != "" && cmp(fdName, e.ShortName)) {
					found = append(found, e)
					break
				}
			}
		}
		return found
	}

	if exact := matches(utils.CompareTeams); len(exact) == 1 {
		return exact[0], true
	} else if len(exact) > 1 {
		return models.ESPNTeam{}, false
	}

	if partial := matches(utils.TeamTokensSubset); len(partial) == 1 {
		return partial[0], true
	}

	return models.ESPNTeam{}, false
}

type ESPNRosterTarget struct {
	ESPNTeamID int64
	ESPNLeague string
}

// GetESPNTeamsForRosterSync lista cada time vinculado à ESPN uma vez, com o slug
// de uma das ligas em que ele joga (ex: "bra.1")
func GetESPNTeamsForRosterSync(ctx context.Context) ([]ESPNRosterTarget, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	rows, err := DB.QueryContext(ctx, `
		SELECT DISTINCT ON (t.espn_team_id) t.espn_team_id, l.code_espn
		FROM teams t
		JOIN team_leagues tl ON tl.team_api_id = t.api_id
		JOIN leagues l ON l.code_api = tl.league
		WHERE t.espn_team_id IS NOT NULL AND t.espn_team_id <> 0
		  AND COALESCE(l.code_espn, '') <> ''
		ORDER BY t.espn_team_id, tl.season DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []ESPNRosterTarget
	for rows.Next() {
		var t ESPNRosterTarget
		if err := rows.Scan(&t.ESPNTeamID, &t.ESPNLeague); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}
