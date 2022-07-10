package db

import (
	"testing"

	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DBTest struct {
	db *DBStore
}

func NewDBTest(t *testing.T) *DBTest {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err)

	driver, err := sqlite3.WithInstance(db.DB, &sqlite3.Config{})
	require.NoError(t, err)

	dbStore, err := New(db, driver, "")
	require.NoError(t, err)

	return &DBTest{db: dbStore}
}

func (d *DBTest) Cleanup(t *testing.T) {
	err := d.db.db.Close()
	assert.NoError(t, err)
}
