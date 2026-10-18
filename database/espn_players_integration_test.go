package database

import (
	"App-Futebol/models"
	"context"
	"testing"
)

// Foto do jogador na sincronização do elenco: link montado antigo (404) sai; foto achada
// pelo robô fica; foto real da ESPN entra
func TestUpsertESPNPlayerFoto(t *testing.T) {
	openTestDB(t)
	ctx := context.Background()
	const id = 990001
	mustExec(t, `DELETE FROM espn_players WHERE espn_id = $1`, id)
	t.Cleanup(func() { mustExec(t, `DELETE FROM espn_players WHERE espn_id = $1`, id) })

	foto := func() string {
		var f string
		DB.QueryRow(`SELECT COALESCE(headshot_url, '') FROM espn_players WHERE espn_id = $1`, id).Scan(&f)
		return f
	}
	salva := func(href string) {
		t.Helper()
		if err := UpsertESPNPlayer(ctx, models.Player{ID: id, Name: "Jogador Teste", TeamID: int(tstESPNHome), HeadshotURL: href}); err != nil {
			t.Fatal(err)
		}
	}

	mustExec(t, `INSERT INTO espn_players (espn_id, name, headshot_url) VALUES ($1, 'Jogador Teste',
                 'https://a.espncdn.com/combiner/i?img=/i/headshots/soccer/players/full/990001.png')`, id)
	salva("")
	if f := foto(); f != "" {
		t.Errorf("link montado antigo deveria sair, veio %q", f)
	}

	mustExec(t, `UPDATE espn_players SET headshot_url = 'https://img.a.transfermarkt.technology/portrait/x.jpg' WHERE espn_id = $1`, id)
	salva("")
	if f := foto(); f != "https://img.a.transfermarkt.technology/portrait/x.jpg" {
		t.Errorf("a foto do Transfermarkt não pode ser apagada pela ESPN sem foto, veio %q", f)
	}

	salva("https://a.espncdn.com/i/headshots/soccer/players/full/990001.png")
	if f := foto(); f != "https://a.espncdn.com/i/headshots/soccer/players/full/990001.png" {
		t.Errorf("foto real da ESPN deveria entrar, veio %q", f)
	}
}
