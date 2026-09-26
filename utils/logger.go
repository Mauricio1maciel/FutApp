package utils

import (
	"fmt"
	"log"
)

var ActiveLogs = map[string]bool{
	"WORKER":                 false,
	"DATABASE":               false,
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
	"WORKER_BUSCA_ESCALACAO": false,
	"SCOREBOARD":             false,
}

func CustomLog(module string, format string, args ...interface{}) {

	if ActiveLogs[module] {
		message := fmt.Sprintf(format, args...)

		log.Printf("[%s] %s\n", module, message)
	}
}
