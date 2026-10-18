package services

import (
	"App-Futebol/database"
	"App-Futebol/testutil"
	"context"
	"testing"
)

// Alerta de tamanho do banco ponta a ponta, contra um Postgres de verdade e o Expo falso
// (testutil.OpenDB explica como rodar)
func TestCheckDatabaseSize(t *testing.T) {
	database.DB = testutil.OpenDB(t, "alert_state")
	ctx := context.Background()
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := database.DB.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	limpa := func() {
		exec(`DELETE FROM alert_state WHERE key = $1`, dbSizeAlertKey)
		exec(`DELETE FROM push_devices WHERE device_id IN ('alerta-admin', 'alerta-comum')`)
	}
	limpa()
	t.Cleanup(limpa)
	exec(`UPDATE push_devices SET admin_alerts = false`) // banco de teste: só este teste usa
	exec(`INSERT INTO push_devices (device_id, push_token, platform, admin_alerts) VALUES
          ('alerta-admin', 'ExponentPushToken[alerta-admin]', 'android', true),
          ('alerta-comum', 'ExponentPushToken[alerta-comum]', 'android', false)`)

	_, lotes := fakeExpo(t, func(ExpoMessage) ExpoTicket { return ExpoTicket{Status: "ok", ID: "x"} })
	enviados := func() int {
		n := 0
		for _, l := range *lotes {
			n += len(l)
		}
		return n
	}
	nivel := func() int64 {
		v, _, err := database.GetAlertState(ctx, dbSizeAlertKey)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	// O banco de teste tem alguns MB: alerta em 1 MB = acima do limite
	t.Setenv("DB_SIZE_ALERT_MB", "1")
	if err := CheckDatabaseSize(ctx); err != nil {
		t.Fatal(err)
	}
	if enviados() != 1 {
		t.Fatalf("esperado 1 aviso (só o aparelho de admin), veio %d: %+v", enviados(), *lotes)
	}
	msg := (*lotes)[0][0]
	if msg.To != "ExponentPushToken[alerta-admin]" || msg.Data["type"] != "admin_alert" || msg.Tag != "admin-db-size" {
		t.Errorf("aviso errado: %+v", msg)
	}
	if nivel() != 1 {
		t.Errorf("nível avisado deveria ser 1, veio %d", nivel())
	}

	// Mesmo nível: não repete
	if err := CheckDatabaseSize(ctx); err != nil {
		t.Fatal(err)
	}
	if enviados() != 1 {
		t.Errorf("no mesmo nível o aviso não pode repetir, veio %d", enviados())
	}

	// O banco "diminuiu" (alerta acima do tamanho): o nível volta a 0, sem aviso...
	t.Setenv("DB_SIZE_ALERT_MB", "100000")
	if err := CheckDatabaseSize(ctx); err != nil {
		t.Fatal(err)
	}
	if enviados() != 1 || nivel() != 0 {
		t.Errorf("abaixo do alerta: esperado sem aviso e nível 0, veio %d avisos e nível %d", enviados(), nivel())
	}
	// ...e avisa de novo quando ele volta a passar do limite
	t.Setenv("DB_SIZE_ALERT_MB", "1")
	if err := CheckDatabaseSize(ctx); err != nil {
		t.Fatal(err)
	}
	if enviados() != 2 {
		t.Errorf("ao passar do limite de novo, deveria avisar de novo: %d avisos", enviados())
	}

	// Sem aparelho de admin: não avisa e não grava o nível (avisa quando houver aparelho)
	exec(`UPDATE push_devices SET admin_alerts = false WHERE device_id = 'alerta-admin'`)
	exec(`DELETE FROM alert_state WHERE key = $1`, dbSizeAlertKey)
	if err := CheckDatabaseSize(ctx); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := database.GetAlertState(ctx, dbSizeAlertKey); found || enviados() != 2 {
		t.Errorf("sem aparelho de admin: esperado sem aviso e sem nível gravado (found=%v, avisos=%d)", found, enviados())
	}
}
