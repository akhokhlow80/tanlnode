package db

import (
	"akhokhlow80/tanlnode/sqlgen"
	"database/sql"
	"sync"
)

type DB struct {
	sync.RWMutex
	*sqlgen.Queries
	*sql.DB
}
