package db

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

//go:embed migrations
var migrations embed.FS

type ExecError struct {
	sql  string
	err  error
	msg  string
	args []interface{}
}

func newExecError(msg, sql string, err error, args ...interface{}) *ExecError {
	return &ExecError{sql: sql, err: err, msg: msg, args: args}
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("%s: executing SQL:\n%s\nargs:%#v\nerror:%v", e.msg, e.sql, e.args, e.err)
}

type DBStore struct {
	db *sqlx.DB
}

func New(db *sqlx.DB, migrationDriver database.Driver, dbName string) (*DBStore, error) {
	d, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("new iofs: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, dbName, migrationDriver)
	if err != nil {
		return nil, fmt.Errorf("new migration instance: %w", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &DBStore{
		db: db,
	}, nil
}
