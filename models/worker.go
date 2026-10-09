// Arquivo: models/worker.go

package models

type WorkerMatch struct {
	IDEvent       string
	League        string
	MatchTimeUnix int64
	Status        string
}
