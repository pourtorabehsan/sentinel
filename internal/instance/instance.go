package instance

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Instance struct {
	db       *sql.DB
	endpoint Endpoint
	config   Config
}

func Open(endpoint Endpoint, conf Config) (*Instance, error) {
	dsn := fmt.Sprintf("%s@tcp(%s)/", conf.MonitorCredentials.AuthString(), endpoint)
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

func (i *Instance) Close() error {
	return i.db.Close()
}

func (i *Instance) Ping() error {
	return i.db.Ping()
}
