package database

import (
	"App-Futebol/utils"
	"log"
)

func SyncCrossAPITeams(espnTeams map[string]int64) error {
	queryAPI := `SELECT api_id, name, short FROM teams WHERE espn_team_id IS NULL`

	rowsAPI, err := DB.Query(queryAPI)
	if err != nil {
		return err
	}
	defer rowsAPI.Close()
	type DBTeam struct {
		ApiID int64
		Name  string
		Short string
	}
	var apiTeams []DBTeam

	for rowsAPI.Next() {
		var t DBTeam
		rowsAPI.Scan(&t.ApiID, &t.Name, &t.Short)
		apiTeams = append(apiTeams, t)
	}

	log.Printf("Iniciando cruzamento: %d times ESPN contra %d times da API sem vínculo...", len(espnTeams), len(apiTeams))

	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	matchesFound := 0

	for espnName, espnID := range espnTeams {
		for _, apiTeam := range apiTeams {
			if utils.CompareTeams(espnName, apiTeam.Name) || utils.CompareTeams(espnName, apiTeam.Short) {
				log.Printf("🔥 MATCH ENCONTRADO: %s (%d) <--> %s (%d)", espnName, espnID, apiTeam.Name, apiTeam.ApiID)
				_, err := tx.Exec(`UPDATE teams SET espn_team_id = $1 WHERE api_id = $2`, espnID, apiTeam.ApiID)
				if err != nil {
					tx.Rollback()
					return err
				}

				matchesFound++
				break
			}
		}
	}

	err = tx.Commit()
	if err == nil {
		log.Printf("✅ Sincronização concluída! %d times atualizados.", matchesFound)
	}
	return err
}
