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
	fakeFs := NewInMemoryFS()
	store := NewFakeStore()
	immutableFs1 := NewImmutableFS(fakeFs, store, Owner{Id: 1})
	immutableFs2 := NewImmutableFS(
		fakeFs, store, Owner{Id: 2, Key: kdf.Random(32)})
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
	assert.Equal(t, 4, numFiles(fakeFs))

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

	// Test that we get nothing back if no ids found.
	entries, err = immutableFs2.List(nil, map[int64]bool{1: true})
	require.NoError(t, err)
	assert.Empty(t, entries)

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
	assert.Equal(t, contents, readBytes(fakeFs, rawPath))

	contents, err = fs.ReadFile(immutableFs2, "4/goodbye.txt")
	require.NoError(t, err)
	assert.Equal(t, "Goodbye World!", string(contents))

	// Verify immutableFs2 data encrypted
	files, err = immutableFs2.List(nil, map[int64]bool{4: true})
	require.NoError(t, err)
	require.Len(t, files, 1)
	rawPath = idToPath(files[0].Checksum, files[0].OwnerId)
	encContents := readBytes(fakeFs, rawPath)
	assert.NotNil(t, encContents)
	assert.NotEqual(t, contents, encContents)

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

func TestImmutableFS_ListError(t *testing.T) {
	fileSystem := NewImmutableFS(NewInMemoryFS(), errorStore{}, Owner{Id: 1})
	_, err := fileSystem.List(nil, map[int64]bool{1: true})
	assert.Error(t, err)
}

func TestImmutableFS_WriteError(t *testing.T) {
	immutableFs := NewImmutableFS(NilFS(), NewFakeStore(), Owner{Id: 1})
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	assert.Error(t, err)
}

func TestImmutableFS_ReadError(t *testing.T) {
	// Prime fakeStore so that we can test error reading filesystem
	fakeStore := NewFakeStore()
	immutableFs := NewImmutableFS(NewInMemoryFS(), fakeStore, Owner{Id: 1})
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)

	// Now do our real test
	immutableFs = NewImmutableFS(NilFS(), fakeStore, Owner{Id: 1})

	// Should get an error reading
	_, err = immutableFs.Open("1/hello.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "1/hello.txt", Err: fs.ErrNotExist},
		err)
}

func TestImmutableFS_DBErrorOnWrite(t *testing.T) {
	immutableFs := NewImmutableFS(NewInMemoryFS(), errorStore{}, Owner{Id: 1})
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	assert.Error(t, err)
}

func TestImmutableFS_DBErrorOnRead(t *testing.T) {
	immutableFs := NewImmutableFS(NewInMemoryFS(), NewFakeStore(), Owner{Id: 1})

	// Should get an error reading
	_, err := immutableFs.Open("1/hello.txt")
	assert.Equal(
		t,
		&fs.PathError{Op: "open", Path: "1/hello.txt", Err: fs.ErrNotExist},
		err)
}

func TestImmutableFS_ReadOnly(t *testing.T) {
	fakeFs := NewInMemoryFS()
	store := NewFakeStore()
	immutableFs := NewImmutableFS(fakeFs, store, Owner{Id: 1})
	readOnlyFs := ReadOnly(immutableFs)
	assert.Equal(t, readOnlyFs, ReadOnly(readOnlyFs))
	_, err := immutableFs.Write("hello.txt", ([]byte)("Hello World!"))
	require.NoError(t, err)

	files, err := readOnlyFs.List(nil, map[int64]bool{1: true})
	require.NoError(t, err)
	require.Len(t, files, 1)

	contents, err := fs.ReadFile(readOnlyFs, files[0].Path())
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(contents))

	_, err = readOnlyFs.Write("goodbye.txt", ([]byte)("Goodbye World!"))
	assert.Equal(t, fs.ErrPermission, err)
}

func TestEntry_FormatTime(t *testing.T) {
	atime := time.Date(2022, 3, 1, 16, 43, 54, 0, time.Local)
	entry := Entry{Ts: atime.Unix()}
	assert.Equal(t, "01-Mar-22 16:43", entry.FormatTime())
}

func TestEntry_FormatSize(t *testing.T) {
	assert.Equal(t, "--", sizeString(-35))
	assert.Equal(t, "0 B", sizeString(0))
	assert.Equal(t, "9 B", sizeString(9))
	assert.Equal(t, "70 B", sizeString(70))
	assert.Equal(t, "520 B", sizeString(520))
	assert.Equal(t, "995 B", sizeString(995))
	assert.Equal(t, "9.87 KB", sizeString(9874))
	assert.Equal(t, "43.5 KB", sizeString(43450))
	assert.Equal(t, "127 KB", sizeString(126999))
	assert.Equal(t, "1.00 MB", sizeString(999512))
	assert.Equal(t, "13.2 MB", sizeString(13201489))
	assert.Equal(t, "576 MB", sizeString(575500001))
	assert.Equal(t, "4.35 GB", sizeString(4350123456))
	assert.Equal(t, "43.5 GB", sizeString(43541234567))
	assert.Equal(t, "435 GB", sizeString(435412345678))
	assert.Equal(t, "4350 GB", sizeString(4354123456789))
	assert.Equal(t, "43600 GB", sizeString(43551234567890))
}

func sizeString(size int64) string {
	entry := Entry{Size: size}
	return entry.FormatSize()
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
