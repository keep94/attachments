package for_sqlite_test

import (
	"testing"

	"github.com/keep94/attachments/attachmentsdb/fixture"
	"github.com/keep94/attachments/attachmentsdb/for_sqlite"
	"github.com/keep94/attachments/attachmentsdb/sqlite_setup"
	"github.com/keep94/gosqlite/sqlite"
	"github.com/keep94/toolbox/db/sqlite_db"
)

func TestEntryById(t *testing.T) {
	db := openDb(t)
	defer closeDb(t, db)
	fixture.EntryById(t, for_sqlite.New(db))
}

func closeDb(t *testing.T, db *sqlite_db.Db) {
	if err := db.Close(); err != nil {
		t.Errorf("Error closing database: %v", err)
	}
}

func openDb(t *testing.T) *sqlite_db.Db {
	conn, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	db := sqlite_db.New(conn)
	err = db.Do(func(conn *sqlite.Conn) error {
		return sqlite_setup.SetUpTables(conn)
	})
	if err != nil {
		t.Fatalf("Error creating tables: %v", err)
	}
	return db
}
