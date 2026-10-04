package services

import (
	"net/http"
	"time"
)

// httpClient é compartilhado por todas as chamadas às APIs externas (ESPN, Football-Data).
// O timeout evita que goroutines fiquem presas para sempre se a API não responder.
var httpClient = &http.Client{Timeout: 15 * time.Second}
