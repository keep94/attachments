package attachments

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/keep94/toolbox/db"
)

var (
	// Indicates that the Id does not exist in the database.
	ErrNoSuchId = errors.New("attachments: No such Id")

	// Indicates writing to a read-only resource.
	ErrReadOnly = errors.New("attachments: read-only")
)

// Entry represents a file entry
type Entry struct {

	// The unique ID for the entry
	Id int64

	// The entry name e.g "document.pdf"
	Name string

	// The size of the file in bytes
	Size int64

	// The timestamp of the file in seconds
	Ts int64

	// The owner of the file
	OwnerId int64

	// Identifies the file contents
	Checksum string
}

// Path returns the path to this file that ImmutableFs.Open() will accept.
// The returned value is of the form EntryId/EntryName e.g "12345/document.pdf"
func (e *Entry) Path() string {
	return fmt.Sprintf("%d/%s", e.Id, e.Name)
}

// Store stores and retrieves file entries from a database.
type Store interface {

	// AddEntry adds a new record to the datastore. AddEntry sets the
	// Id of the newly added record in entry.
	AddEntry(t db.Transaction, entry *Entry) error

	// EntryById retrieves the record with given id and ownerId storing it in
	// entry. EntryById returns ErrNoSuchId if no record found.
	EntryById(t db.Transaction, id, ownerId int64, entry *Entry) error
}

// NewFakeStore returns an in memory implementation of Store. Multiple
// goroutines can concurrently read the returned Store as long as no
// goroutines are writing to the returned store at the same time.
func NewFakeStore() Store {
	var store fakeStore
	return &store
}

// ReadOnlyStore returns a read-only version of Store. Calling AddEntry()
// on returned Store returns ErrReadOnly.
func ReadOnlyStore(store Store) Store {
	return &readOnlyStore{Store: store}
}

// ReadOnlyFS returns a read-only version of fileSystem. Calling Write()
// on returned file system returns ErrReadOnly.
func ReadOnlyFS(fileSystem FS) FS {
	return &readOnlyFS{FS: fileSystem}
}

// Option represents an optional settings for ImmutableFS.
type Option interface {
	apply(fs *ImmutableFS)
}

// Key enables encryption with given encryption key.
func Key(key []byte) Option {
	return &keyOption{Key: copyKey(key)}
}

// ImmutableFS represents an immutable file system featuring AES-256
// encryption.
type ImmutableFS struct {

	// Store is the database store for the file entries
	store Store

	// fileSystem stores the contents of the files by checksum
	fileSystem aesFS
}

// NewImmutableFS creates a new ImmutableFS instance.
func NewImmutableFS(
	fileSystem FS, store Store, ownerId int64, options ...Option) *ImmutableFS {
	result := &ImmutableFS{
		store: store,
		fileSystem: aesFS{
			FileSystem: fileSystem,
			OwnerId:    ownerId,
		},
	}
	for _, o := range options {
		o.apply(result)
	}
	return result
}

// Open opens the named file. name is of the form EntryId/EntryName e.g
// "12345/document.pdf"
func (f *ImmutableFS) Open(name string) (fs.File, error) {
	pathErr := &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	id, baseName, ok := parsePath(name)
	if !ok {
		return nil, pathErr
	}
	var entry Entry
	err := f.store.EntryById(nil, id, f.fileSystem.OwnerId, &entry)
	if err != nil {
		return nil, pathErr
	}
	if baseName != entry.Name {
		return nil, pathErr
	}
	readCloser, err := f.fileSystem.Open(entry.Checksum)
	if err != nil {
		return nil, pathErr
	}
	return &immutableFile{ReadCloser: readCloser, entry: &entry}, nil
}

// Write writes a new file. name is the file name e.g "document.pdf" Write
// returns the Id of the new file, e.g 12345.
func (f *ImmutableFS) Write(name string, contents []byte) (int64, error) {
	checksum, err := f.fileSystem.Write(contents)
	if err != nil {
		return 0, err
	}
	entry := Entry{
		Name:     name,
		Size:     int64(len(contents)),
		Ts:       time.Now().Unix(),
		OwnerId:  f.fileSystem.OwnerId,
		Checksum: checksum,
	}
	if err := f.store.AddEntry(nil, &entry); err != nil {
		return 0, err
	}
	return entry.Id, nil
}

// List returns the files with given ids ordered by id.
// If an id has no file associated with it, the slice returned will not have
// an Entry for that id.
func (f *ImmutableFS) List(
	t db.Transaction, ids map[int64]bool) ([]Entry, error) {
	var result []Entry
	for id, ok := range ids {
		if !ok {
			continue
		}
		var entry Entry
		err := f.store.EntryById(t, id, f.fileSystem.OwnerId, &entry)
		if err == ErrNoSuchId {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	sort.Slice(
		result, func(i, j int) bool { return result[i].Id < result[j].Id })
	return result, nil
}

func parsePath(name string) (id int64, baseName string, ok bool) {
	if !fs.ValidPath(name) {
		return
	}
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		return
	}
	fileId, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return
	}
	return fileId, parts[1], true
}

type immutableFile struct {
	io.ReadCloser
	entry *Entry
}

func (f *immutableFile) Stat() (fs.FileInfo, error) {
	return fileInfo{entry: f.entry}, nil
}

type fileInfo struct {
	entry *Entry
}

func (f fileInfo) Name() string {
	return f.entry.Name
}

func (f fileInfo) Size() int64 {
	return f.entry.Size
}

func (f fileInfo) Mode() fs.FileMode {
	return 0400
}

func (f fileInfo) ModTime() time.Time {
	return time.Unix(f.entry.Ts, 0)
}

func (f fileInfo) IsDir() bool {
	return false
}

func (f fileInfo) Sys() interface{} {
	return nil
}

type fakeStore []Entry

func (f *fakeStore) AddEntry(t db.Transaction, entry *Entry) error {
	newId := int64(len(*f)) + 1
	entry.Id = newId
	*f = append(*f, *entry)
	return nil
}

func (f fakeStore) EntryById(
	t db.Transaction, id, ownerId int64, entry *Entry) error {
	index := int(id - 1)
	if index < 0 || index >= len(f) {
		return ErrNoSuchId
	}
	if ownerId != f[index].OwnerId {
		return ErrNoSuchId
	}
	*entry = f[index]
	return nil
}

type readOnlyStore struct {
	Store
}

func (r *readOnlyStore) AddEntry(t db.Transaction, entry *Entry) error {
	return ErrReadOnly
}

type readOnlyFS struct {
	FS
}

func (r *readOnlyFS) Write(name string) (io.WriteCloser, error) {
	return nil, ErrReadOnly
}

func copyKey(key []byte) []byte {
	result := make([]byte, len(key))
	copy(result, key)
	return result
}

type keyOption struct {
	Key []byte
}

func (k *keyOption) apply(fs *ImmutableFS) {
	fs.fileSystem.Key = k.Key
}
