package config

import (
	"chirpy/internal/database"
	"sync/atomic"
)

type ApiConfig struct {
	FileserverHits atomic.Int32
	Db             database.Queries
	TokenSecret    string
	PolkaKey       string
}
