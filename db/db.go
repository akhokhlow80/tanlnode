package db

import (
	"akhokhlow80/tanlnode/sqlgen/sqlpeers"
	"database/sql"
	"sync"
)

type DB struct {
	sync.RWMutex
	*sqlpeers.Queries
	*sql.DB
}
