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

// FormatTime returns the file timestamp in the form of "02-Jan-06 15:04"
// in the local timezone.
func (e *Entry) FormatTime() string {
	return time.Unix(e.Ts, 0).Format("02-Jan-06 15:04")
}

// FormatSize returns the file size as readable string such as "1.23 MB"
func (e *Entry) FormatSize() string {
	return formatSize(e.Size)
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

// ImmutableFS represents an immutable file system featuring AES-256
// encryption. Note that ImmutableFS implements io/fs.FS
type ImmutableFS interface {

	// Open opens the named file. name is of the form EntryId/EntryName e.g
	// "12345/document.pdf"
	Open(name string) (fs.File, error)

	// Write writes a new file. name is the file name e.g "document.pdf."
	// Write returns the Id of the new file, e.g 12345. If this instance
	// is read-only, Write returns fs.ErrPermission.
	Write(name string, contents []byte) (int64, error)

	// List returns the files with given ids ordered by id.
	// If an id has no file associated with it, the slice returned will not
	// have an Entry for that id.
	List(t db.Transaction, ids map[int64]bool) ([]Entry, error)

	// ReadOnly returns true if this instance is read-only.
	ReadOnly() bool

	private()
}

// NewImmutableFS creates a new ImmutableFS instance.
// fileSystem is where the contents of files from all owners are stored.
// store is where file meta data from all owners are stored such as size
// and timestamp. owner specifies the file owner. The returned instance
// will store and retrieve files only for that owner.
func NewImmutableFS(fileSystem FS, store Store, owner Owner) ImmutableFS {
	return &immutableFS{
		Store: store,
		aesFS: aesFS{
			FileSystem: fileSystem,
			Owner:      owner,
		},
	}
}

// ReadOnly creates a read-only wrapper around fileSystem.
// If fileSystem is already read-only, ReadOnly returns it unchanged.
func ReadOnly(fileSystem ImmutableFS) ImmutableFS {
	if fileSystem.ReadOnly() {
		return fileSystem
	}
	return &roImmutableFS{ImmutableFS: fileSystem}
}

type immutableFS struct {
	Store
	aesFS
}

func (f *immutableFS) Open(name string) (fs.File, error) {
	pathErr := &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	id, baseName, ok := parsePath(name)
	if !ok {
		return nil, pathErr
	}
	var entry Entry
	err := f.EntryById(nil, id, f.Owner.Id, &entry)
	if err != nil {
		return nil, pathErr
	}
	if baseName != entry.Name {
		return nil, pathErr
	}
	readCloser, err := f.aesFS.Open(entry.Checksum)
	if err != nil {
		return nil, pathErr
	}
	return &immutableFile{ReadCloser: readCloser, entry: &entry}, nil
}

func (f *immutableFS) Write(name string, contents []byte) (int64, error) {
	checksum, err := f.aesFS.Write(contents)
	if err != nil {
		return 0, err
	}
	entry := Entry{
		Name:     name,
		Size:     int64(len(contents)),
		Ts:       time.Now().Unix(),
		OwnerId:  f.Owner.Id,
		Checksum: checksum,
	}
	if err := f.AddEntry(nil, &entry); err != nil {
		return 0, err
	}
	return entry.Id, nil
}

func (f *immutableFS) List(
	t db.Transaction, ids map[int64]bool) ([]Entry, error) {
	var result []Entry
	var entry Entry
	for id, ok := range ids {
		if !ok {
			continue
		}
		err := f.EntryById(t, id, f.Owner.Id, &entry)
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

func (f *immutableFS) ReadOnly() bool {
	return false
}

func (f *immutableFS) private() {
}

type roImmutableFS struct {
	ImmutableFS
}

func (f *roImmutableFS) Write(
	name string, contents []byte) (int64, error) {
	return 0, fs.ErrPermission
}

func (f *roImmutableFS) ReadOnly() bool {
	return true
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
