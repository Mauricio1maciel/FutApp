package utils

import (
	"fmt"
	"log"
)

var ActiveLogs = map[string]bool{
	"WORKER":                 false,
	"DATABASE":               true,
	"DATABASE_ERRO":          true,
	"API":                    true,
	"ESPN":                   true,
	"IMAGE_SEARCH":           true,
	"BOT":                    true,
	"AUTH":                   true,
	"COPA":                   true,
	"DB_INFO":                true,
	"DB_ERRO":                true,
	"SISTEMA":                true,
	"WORKER_ERRO":            true,
	"WORKER_BUSCA_ESCALACAO": true,
	"SCOREBOARD":             true,
	"UNL":                    true,
}

func CustomLog(module string, format string, args ...interface{}) {

	if ActiveLogs[module] {
		message := fmt.Sprintf(format, args...)

		log.Printf("[%s] %s\n", module, message)
	}
}
