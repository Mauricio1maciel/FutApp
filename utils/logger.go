package utils

import (
	"fmt"
	"log"
)

var ActiveLogs = map[string]bool{
	"WORKER":                 false,
	"DATABASE":               true,
	"DATABASE_ERRO":          false,
	"API":                    false,
	"ESPN":                   false,
	"IMAGE_SEARCH":           false,
	"BOT":                    false,
	"AUTH":                   false,
	"COPA":                   false,
	"DB_INFO":                false,
	"DB_ERRO":                false,
	"SISTEMA":                false,
	"WORKER_ERRO":            false,
	"WORKER_BUSCA_ESCALACAO": true,
	"SCOREBOARD":             false,
	"UNL":                    false,
	"RATE_LIMIT":             false,
	"SYNC_TEAMS":             false,
	"JOBS":                   true,
	"HTTP":                   true,
	"STATS":                  false,
}

func CustomLog(module string, format string, args ...interface{}) {

	if ActiveLogs[module] {
		message := fmt.Sprintf(format, args...)

		log.Printf("[%s] %s\n", module, message)
	}
}
