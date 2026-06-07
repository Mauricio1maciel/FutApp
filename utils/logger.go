package utils

import (
	"fmt"
	"log"
)

var ActiveLogs = map[string]bool{
	"WORKER":       false,
	"DATABASE":     true,
	"API":          false,
	"ESPN":         false,
	"IMAGE_SEARCH": false,
	"BOT":          false,
	"AUTH":         false,
}

func CustomLog(module string, format string, args ...interface{}) {

	if ActiveLogs[module] {
		message := fmt.Sprintf(format, args...)

		log.Printf("[%s] %s\n", module, message)
	}
}
