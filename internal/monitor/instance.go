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
	config   Config
}

func openInstance(endpoint Endpoint, conf Config) (*Instance, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", conf.MonitorUsername, conf.MonitorPassword, endpoint.Host, endpoint.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return &Instance{
		db:       db,
		endpoint: endpoint,
		config:   conf,
	}, nil
}

func (i *Instance) monitor(ctx context.Context) error {
	return nil
}
