// Package for_sqlite provides a sqlite implementation of the attachments
// database.
package for_sqlite

import (
	"github.com/keep94/attachments"
	"github.com/keep94/gosqlite/sqlite"
	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/db/sqlite_db"
	"github.com/keep94/toolbox/db/sqlite_rw"
)

const (
	kSQLEntryById = "select id, name, size, ts, owner, checksum from attachments where id = ? and owner = ?"
	kSQLAddEntry  = "insert into attachments (name, size, ts, owner, checksum) values (?, ?, ?, ?, ?)"
)

// Store is a sqlite implementation of attachments.Store
type Store struct {
	db sqlite_db.Doer
}

// New creates a new Store instance.
func New(db *sqlite_db.Db) Store {
	return Store{db}
}

// ConnNew creates a new Store instance from a sqlite connection.
func ConnNew(conn *sqlite.Conn) Store {
	return Store{sqlite_db.NewSqliteDoer(conn)}
}

func (s Store) AddEntry(
	t db.Transaction, entry *attachments.Entry) error {
	return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
		return sqlite_rw.AddRow(
			conn, (&rawEntry{}).init(entry), &entry.Id, kSQLAddEntry)
	})
}

func (s Store) EntryById(
	t db.Transaction, id, ownerId int64, entry *attachments.Entry) error {
	return sqlite_db.ToDoer(s.db, t).Do(func(conn *sqlite.Conn) error {
		return sqlite_rw.ReadSingle(
			conn,
			(&rawEntry{}).init(entry),
			attachments.ErrNoSuchId,
			kSQLEntryById,
			id,
			ownerId)
	})
}

type rawEntry struct {
	*attachments.Entry
	sqlite_rw.SimpleRow
}

func (r *rawEntry) init(bo *attachments.Entry) *rawEntry {
	r.Entry = bo
	return r
}

func (r *rawEntry) Ptrs() []interface{} {
	return []interface{}{&r.Id, &r.Name, &r.Size, &r.Ts, &r.OwnerId, &r.Checksum}
}

func (r *rawEntry) Values() []interface{} {
	return []interface{}{r.Name, r.Size, r.Ts, r.OwnerId, r.Checksum, r.Id}
}

func (r *rawEntry) ValuePtr() interface{} {
	return r.Entry
}
