package services

import "testing"

func TestDBSizeAlertLevel(t *testing.T) {
	for _, c := range []struct {
		sizeMB float64
		alert  int64
		want   int64
	}{
		{33, 300, 0},
		{299.9, 300, 0},
		{300, 300, 1},
		{349.9, 300, 1},
		{350, 300, 2},
		{455, 300, 4},
		{510, 300, 5},
	} {
		if got := dbSizeAlertLevel(c.sizeMB, c.alert); got != c.want {
			t.Errorf("%.1f MB com alerta em %d: esperado nível %d, veio %d", c.sizeMB, c.alert, c.want, got)
		}
	}
}

func TestDBSizeAlertText(t *testing.T) {
	r := DBSizeReport{SizeMB: 312.4, LimitMB: 500, Percent: 62.5,
		Tables: []TableSizeMB{{Name: "espn_match_events", SizeMB: 98.2}}}
	title, body := dbSizeAlertText(r)
	if title != "⚠️ Banco de dados em 62%" {
		t.Errorf("título: %q", title)
	}
	if body != "312 MB de 500 MB do plano grátis do Supabase. Maior tabela: espn_match_events (98 MB)." {
		t.Errorf("corpo: %q", body)
	}

	r.Percent, r.Tables = 91.2, nil
	title, body = dbSizeAlertText(r)
	if title != "🚨 Banco de dados quase cheio: 91%" || body != "312 MB de 500 MB do plano grátis do Supabase." {
		t.Errorf("perto do limite: %q | %q", title, body)
	}
}
