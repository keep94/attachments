package attachments

import (
	"io"
	"os"
	"testing"

	"github.com/keep94/toolbox/kdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	kNotFoundId = "123456789A"
)

func TestIdToPath(t *testing.T) {
	assert.Equal(t, "1/35/3567", idToPath("3567", 1))
}

func TestEncFileSystem_NoEncryption(t *testing.T) {
	fakeFS := make(FakeFS)
	fileSystem := &aesFS{FileSystem: fakeFS}
	helloId, err := fileSystem.Write(([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Len(t, helloId, 64)
	helloId2, err := fileSystem.Write(([]byte)("Hello World!"))
	require.NoError(t, err)

	assert.Equal(t, helloId, helloId2)
	assert.Len(t, fakeFS, 1)

	goodbyeId, err := fileSystem.Write(([]byte)("Goodbye World!"))
	require.NoError(t, err)
	assert.NotEqual(t, helloId, goodbyeId)

	contents, err := readFile(fileSystem, helloId)
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(contents))

	contents, err = readFile(fileSystem, goodbyeId)
	require.NoError(t, err)
	assert.Equal(t, "Goodbye World!", string(contents))

	_, err = readFile(fileSystem, kNotFoundId)
	assert.Equal(t, os.ErrNotExist, err)

	// Assert contents not encrypted
	contents = fakeFS[idToPath(helloId, 0)]
	assert.Equal(t, "Hello World!", string(contents))
}

func TestEncFileSystem_Encryption(t *testing.T) {
	key1 := kdf.Random(32)
	key2 := kdf.Random(32)

	fakeFS := make(FakeFS)
	fileSystem1 := &aesFS{
		FileSystem: fakeFS,
		Owner:      Owner{Key: key1, Id: 1},
	}
	fileSystem2 := &aesFS{
		FileSystem: fakeFS,
		Owner:      Owner{Key: key2, Id: 2},
	}
	helloId, err := fileSystem1.Write(([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Len(t, helloId, 64)

	contents, err := readFile(fileSystem1, helloId)
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(contents))

	// Verify that contents is actually encrypted in underlying file system
	encContents := fakeFS[idToPath(helloId, 1)]
	assert.NotEqual(t, encContents, contents)

	// Assert hello world checksum is the same for both users and that we
	// write hello world again encrypted for second user
	oldFileCount := len(fakeFS)
	helloId2, err := fileSystem2.Write(([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Equal(t, helloId, helloId2)
	assert.Len(t, fakeFS, oldFileCount+1)

	// Assert that using the wrong encryption key to read does not work.
	fileSystem1.Owner.Key = key2
	contents, err = readFile(fileSystem1, helloId)
	require.NoError(t, err)
	assert.NotEqual(t, "Hello World!", string(contents))

	// Assert that we get ErrNotExist
	fileSystem1.Owner.Key = key1
	_, err = readFile(fileSystem1, kNotFoundId)
	assert.Equal(t, os.ErrNotExist, err)
}

type rofs interface {

	// Open opens a file
	Open(name string) (io.ReadCloser, error)
}

func readFile(fileSystem rofs, name string) ([]byte, error) {
	reader, err := fileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
