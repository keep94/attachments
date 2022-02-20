// Package sqlite_setup sets up a sqlite database for attachments
package sqlite_setup

import (
	"github.com/keep94/gosqlite/sqlite"
)

// SetUpTables creates all needed tables for attachments.
func SetUpTables(conn *sqlite.Conn) error {
	return conn.Exec("create table if not exists attachments (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, size INTEGER, ts INTEGER, owner INTEGER, checksum TEXT)")
}
