package attachments

import (
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/keep94/toolbox/db"
	"github.com/keep94/toolbox/kdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errDatabase = errors.New("attachments: database error")
)

func TestImmutableFS(t *testing.T) {
	fakeFs := make(FakeFS)
	store := NewFakeStore()
	immutableFs1 := NewImmutableFS(fakeFs, store, 1)
	immutableFs2 := NewImmutableFS(fakeFs, store, 2, Key(kdf.Random(32)))
	firstId, err := immutableFs1.Write(
		"hello.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), firstId)

	// newId but contents not rewritten.
	secondId, err := immutableFs1.Write(
		"hello_again.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), secondId)

	thirdId, err := immutableFs2.Write(
		"hello2.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Equal(t, int64(3), thirdId)

	fourthId, err := immutableFs2.Write(
		"goodbye.txt", ([]byte)("Goodbye World!"))
	require.NoError(t, err)
	assert.Equal(t, int64(4), fourthId)

	fifthId, err := immutableFs2.Write(
		"solong.txt", ([]byte)("So long everyone!"))
	require.NoError(t, err)
	assert.Equal(t, int64(5), fifthId)

	// The two "Hello World!" files written to immutableFs1 have the same
	// content which is why we expect 4, not 5, files on the underlying file
	// system.
	assert.Len(t, fakeFs, 4)

	entries, err := immutableFs2.List(
		nil,
		map[int64]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "3/hello2.txt", entries[0].Path())
	assert.Equal(t, "4/goodbye.txt", entries[1].Path())
	assert.Equal(t, "5/solong.txt", entries[2].Path())

	entries, err = immutableFs2.List(
		nil,
		map[int64]bool{1: true, 2: true, 3: true, 4: true, 5: false, 6: true})
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "3/hello2.txt", entries[0].Path())
	assert.Equal(t, "4/goodbye.txt", entries[1].Path())

	// Test that List correctly handles database errors.
	immutableFs2.store = errorStore{}
	_, err = immutableFs2.List(nil, map[int64]bool{1: true})
	assert.Error(t, err)
	immutableFs2.store = store

	// Invalid path
	_, err = immutableFs1.Open("/x")
	assert.Equal(
		t, &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrNotExist}, err)

	// path that doesn't have two parts
	_, err = immutableFs1.Open("hello")
	assert.Equal(
		t, &fs.PathError{Op: "open", Path: "hello", Err: fs.ErrNotExist}, err)

	// Path dosn't start with number
	_, err = immutableFs1.Open("hello/goodbye")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "hello/goodbye", Err: fs.ErrNotExist},
		err)

	// File name is wrong
	_, err = immutableFs1.Open("1/goodbye.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "1/goodbye.txt", Err: fs.ErrNotExist},
		err)

	// File belongs to a different owner
	_, err = immutableFs1.Open("4/goodbye.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "4/goodbye.txt", Err: fs.ErrNotExist},
		err)

	contents, err := fs.ReadFile(immutableFs1, "1/hello.txt")
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(contents))

	// Verify immutableFs1 data stored in the clear
	files, err := immutableFs1.List(nil, map[int64]bool{1: true})
	require.NoError(t, err)
	require.Len(t, files, 1)
	rawPath := idToPath(files[0].Checksum, files[0].OwnerId)
	assert.Equal(t, contents, fakeFs[rawPath])

	contents, err = fs.ReadFile(immutableFs2, "4/goodbye.txt")
	require.NoError(t, err)
	assert.Equal(t, "Goodbye World!", string(contents))

	// Verify immutableFs2 data encrypted
	files, err = immutableFs2.List(nil, map[int64]bool{4: true})
	require.NoError(t, err)
	require.Len(t, files, 1)
	rawPath = idToPath(files[0].Checksum, files[0].OwnerId)
	assert.NotEqual(t, contents, fakeFs[rawPath])

	file, err := immutableFs2.Open("3/hello2.txt")
	require.NoError(t, err)
	defer file.Close()
	fileInfo, err := file.Stat()
	require.NoError(t, err)
	assert.Equal(t, "hello2.txt", fileInfo.Name())
	assert.Equal(t, int64(12), fileInfo.Size())
	assert.Equal(t, fs.FileMode(0400), fileInfo.Mode())
	assert.False(t, fileInfo.IsDir())
	assert.Nil(t, fileInfo.Sys())

	// Assert that timestamp is reasonably current
	assert.Less(t, time.Now().Sub(fileInfo.ModTime()), 5*time.Second)
}

func TestImmutableFS_WriteError(t *testing.T) {
	fakeFs := ReadOnlyFS(make(FakeFS))
	immutableFs := NewImmutableFS(fakeFs, NewFakeStore(), 1)
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	assert.Error(t, err)
}

func TestImmutableFS_ReadError(t *testing.T) {
	fakeFs := make(FakeFS)
	immutableFs := NewImmutableFS(fakeFs, NewFakeStore(), 1)
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)

	// Wipe out all files underneath
	for fileName := range fakeFs {
		delete(fakeFs, fileName)
	}

	// Should get an error reading
	_, err = immutableFs.Open("1/hello.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "1/hello.txt", Err: fs.ErrNotExist},
		err)
}

func TestImmutableFS_DBErrorOnWrite(t *testing.T) {
	store := ReadOnlyStore(NewFakeStore())
	immutableFs := NewImmutableFS(make(FakeFS), store, 1)
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	assert.Error(t, err)
}

func TestImmutableFS_DBErrorOnRead(t *testing.T) {
	immutableFs := NewImmutableFS(make(FakeFS), NewFakeStore(), 1)
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)

	immutableFs.store = NewFakeStore()

	// Should get an error reading
	_, err = immutableFs.Open("1/hello.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "1/hello.txt", Err: fs.ErrNotExist},
		err)
}

func TestImmutableFS_DefensiveCopy(t *testing.T) {
	fs := make(FakeFS)
	store := NewFakeStore()
	fs1 := NewImmutableFS(fs, store, 1)
	origKey := kdf.Random(32)
	key := make([]byte, len(origKey))
	copy(key, origKey)
	option := Key(key)
	key[0] ^= 0xFF
	fs2 := NewImmutableFS(fs, store, 2, option)
	assert.Nil(t, fs1.fileSystem.Key)
	assert.Equal(t, origKey, fs2.fileSystem.Key)
}

func TestReadOnlyStore(t *testing.T) {
	store := NewFakeStore()
	var entry Entry
	require.NoError(t, store.AddEntry(nil, &entry))
	roStore := ReadOnlyStore(store)
	var fetchedEntry Entry
	require.NoError(t, roStore.EntryById(nil, 1, 0, &fetchedEntry))
	assert.Equal(t, entry, fetchedEntry)
	assert.Equal(t, ErrReadOnly, roStore.AddEntry(nil, &entry))
}

func TestReadOnlyFS(t *testing.T) {
	fileSystem := FakeFS{"hello.txt": ([]byte)("Hello World!")}
	roFileSystem := ReadOnlyFS(fileSystem)
	assert.True(t, roFileSystem.Exists("hello.txt"))
	contents, err := readFile(roFileSystem, "hello.txt")
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(contents))
	_, err = roFileSystem.Write("goodbye.txt")
	assert.Equal(t, ErrReadOnly, err)
}

type errorStore struct {
}

func (errorStore) EntryById(
	t db.Transaction, id, ownerId int64, entry *Entry) error {
	return errDatabase
}

func (errorStore) AddEntry(t db.Transaction, entry *Entry) error {
	return errDatabase
}
