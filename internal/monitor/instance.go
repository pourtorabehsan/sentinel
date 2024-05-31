package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

type Instance struct {
	mu         sync.RWMutex
	db         *sql.DB
	endpoint   Endpoint
	alias      string
	serverID   int
	serverUUID string

	source   *Instance
	replicas []*Instance
	conf     instanceConfig
}

func openInstance(endpoint Endpoint, conf instanceConfig) (*Instance, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", conf.monitorUsername, conf.monitorPassword, endpoint.Host, endpoint.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return &Instance{
		db:       db,
		endpoint: endpoint,
		conf:     conf,
	}, nil
}

func (i *Instance) monitor(ctx context.Context) error {
	return nil
}
