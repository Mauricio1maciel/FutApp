package services

import (
	"App-Futebol/database"
	"App-Futebol/utils"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Alerta de tamanho do banco: o plano grátis do Supabase vai até 500 MB, e acima disso
// o banco fica só leitura (o worker para de gravar placares). O admin recebe um push
// quando o banco passa de DB_SIZE_ALERT_MB e, depois, a cada 50 MB a mais.
const (
	dbSizeAlertKey = "db_size_level"
	dbSizeStepMB   = 50
	bytesPerMB     = 1024 * 1024
)

type TableSizeMB struct {
	Name   string  `json:"name"`
	SizeMB float64 `json:"size_mb"`
}

type DBSizeReport struct {
	SizeMB  float64       `json:"size_mb"`
	LimitMB int64         `json:"limit_mb"` // DB_SIZE_LIMIT_MB, padrão 500 (plano grátis)
	Percent float64       `json:"percent"`
	AlertMB int64         `json:"alert_mb"` // DB_SIZE_ALERT_MB, padrão 300
	Tables  []TableSizeMB `json:"tables"`   // as maiores, da maior para a menor
}

// GetDBSizeReport mede o banco e as 10 maiores tabelas
func GetDBSizeReport(ctx context.Context) (DBSizeReport, error) {
	size, err := database.GetDatabaseSize(ctx)
	if err != nil {
		return DBSizeReport{}, err
	}
	tables, err := database.GetLargestTables(ctx, 10)
	if err != nil {
		return DBSizeReport{}, err
	}

	r := DBSizeReport{
		SizeMB:  toMB(size),
		LimitMB: envMB("DB_SIZE_LIMIT_MB", 500),
		AlertMB: envMB("DB_SIZE_ALERT_MB", 300),
		Tables:  make([]TableSizeMB, 0, len(tables)),
	}
	r.Percent = math.Round(r.SizeMB/float64(r.LimitMB)*1000) / 10
	for _, t := range tables {
		r.Tables = append(r.Tables, TableSizeMB{Name: t.Name, SizeMB: toMB(t.Bytes)})
	}
	return r, nil
}

// CheckDatabaseSize mede o banco e avisa o admin quando ele sobe de nível. Verificar de
// novo no mesmo nível não repete o aviso; se o banco diminuir (limpeza), o nível volta
// e o aviso sai de novo quando ele crescer.
func CheckDatabaseSize(ctx context.Context) error {
	r, err := GetDBSizeReport(ctx)
	if err != nil {
		return err
	}
	utils.CustomLog("JOBS", "Banco de dados: %.0f MB de %d MB (%.1f%%)", r.SizeMB, r.LimitMB, r.Percent)

	level := dbSizeAlertLevel(r.SizeMB, r.AlertMB)
	last, _, err := database.GetAlertState(ctx, dbSizeAlertKey)
	if err != nil {
		return err
	}
	if level < last {
		return database.SetAlertState(ctx, dbSizeAlertKey, level)
	}
	if level == last {
		return nil
	}

	tokens, err := database.GetAdminPushTokens(ctx)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		// Não grava o nível: o aviso sai quando houver um aparelho
		utils.CustomLog("JOBS", "Banco acima de %d MB, mas nenhum aparelho recebe os alertas: toque em \"Enviar push de teste para este aparelho\" na tela de admin do app", r.AlertMB)
		return nil
	}

	title, body := dbSizeAlertText(r)
	msgs := make([]ExpoMessage, len(tokens))
	for i, t := range tokens {
		msgs[i] = ExpoMessage{
			To: t, Title: title, Body: body,
			Data:  map[string]string{"type": "admin_alert", "alert": "db_size"},
			Sound: "default", Priority: "high", TTL: 24 * 3600,
			Tag: "admin-db-size", CollapseID: "admin-db-size",
		}
	}
	tickets, err := SendExpo(ctx, msgs)
	if err != nil {
		return err
	}
	sent := false
	var refused []string
	for i, tk := range tickets {
		if tk.Status == "ok" {
			sent = true
			continue
		}
		refused = append(refused, tk.Details.Error+" "+tk.Message)
		if tk.Details.Error == "DeviceNotRegistered" && i < len(tokens) {
			database.DeletePushToken(ctx, tokens[i])
		}
	}
	if !sent {
		return fmt.Errorf("o Expo não aceitou o alerta do banco: %s", strings.Join(refused, "; "))
	}
	utils.CustomLog("JOBS", "Alerta de tamanho do banco enviado: %s", title)
	return database.SetAlertState(ctx, dbSizeAlertKey, level)
}

// dbSizeAlertLevel: 0 abaixo do alerta; 1 ao passar dele; +1 a cada 50 MB a mais
func dbSizeAlertLevel(sizeMB float64, alertMB int64) int64 {
	if sizeMB < float64(alertMB) {
		return 0
	}
	return 1 + int64((sizeMB-float64(alertMB))/dbSizeStepMB)
}

func dbSizeAlertText(r DBSizeReport) (title, body string) {
	title = fmt.Sprintf("⚠️ Banco de dados em %.0f%%", r.Percent)
	if r.Percent >= 90 {
		title = fmt.Sprintf("🚨 Banco de dados quase cheio: %.0f%%", r.Percent)
	}
	body = fmt.Sprintf("%.0f MB de %d MB do plano grátis do Supabase.", r.SizeMB, r.LimitMB)
	if len(r.Tables) > 0 {
		body += fmt.Sprintf(" Maior tabela: %s (%.0f MB).", r.Tables[0].Name, r.Tables[0].SizeMB)
	}
	return title, body
}

func toMB(bytes int64) float64 {
	return math.Round(float64(bytes)/bytesPerMB*10) / 10
}

func envMB(name string, def int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(name), 10, 64); err == nil && v > 0 {
		return v
	}
	return def
}
