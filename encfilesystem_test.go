package attachments

import (
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
	fakeFS := NewInMemoryFS()
	fileSystem := &aesFS{FileSystem: fakeFS}
	helloId, err := fileSystem.Write(([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Len(t, helloId, 64)
	helloId2, err := fileSystem.Write(([]byte)("Hello World!"))
	require.NoError(t, err)

	assert.Equal(t, helloId, helloId2)
	assert.Equal(t, 1, numFiles(fakeFS))

	goodbyeId, err := fileSystem.Write(([]byte)("Goodbye World!"))
	require.NoError(t, err)
	assert.NotEqual(t, helloId, goodbyeId)

	assert.Equal(t, "Hello World!", string(readBytes(fileSystem, helloId)))
	assert.Equal(t, "Goodbye World!", string(readBytes(fileSystem, goodbyeId)))

	_, err = readFile(fileSystem, kNotFoundId)
	assert.Equal(t, os.ErrNotExist, err)
	assert.Nil(t, readBytes(fileSystem, kNotFoundId))

	// Assert contents not encrypted
	contents := readBytes(fakeFS, idToPath(helloId, 0))
	assert.Equal(t, "Hello World!", string(contents))
}

func TestEncFileSystem_Encryption(t *testing.T) {
	key1 := kdf.Random(32)
	key2 := kdf.Random(32)

	fakeFS := NewInMemoryFS()
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

	contents := readBytes(fileSystem1, helloId)
	assert.Equal(t, "Hello World!", string(contents))

	// Verify that contents is actually encrypted in underlying file system
	encContents := readBytes(fakeFS, idToPath(helloId, 1))
	assert.NotNil(t, encContents)
	assert.NotEqual(t, encContents, contents)

	// Assert hello world checksum is the same for both users and that we
	// write hello world again encrypted for second user
	oldFileCount := numFiles(fakeFS)
	helloId2, err := fileSystem2.Write(([]byte)("Hello World!"))
	require.NoError(t, err)
	assert.Equal(t, helloId, helloId2)
	assert.Equal(t, oldFileCount+1, numFiles(fakeFS))

	// Assert that using the wrong encryption key to read does not work.
	fileSystem1.Owner.Key = key2
	contents = readBytes(fileSystem1, helloId)
	assert.NotNil(t, contents)
	assert.NotEqual(t, "Hello World!", string(contents))

	// Assert that we get ErrNotExist
	fileSystem1.Owner.Key = key1
	_, err = readFile(fileSystem1, kNotFoundId)
	assert.Equal(t, os.ErrNotExist, err)
}

func TestEncFileSystem_ReadBadId(t *testing.T) {
	key1 := kdf.Random(32)

	fakeFS := NewInMemoryFS()
	fileSystem1 := &aesFS{
		FileSystem: fakeFS,
		Owner:      Owner{Key: key1, Id: 1},
	}
	_, err := readFile(fileSystem1, "")
	assert.Error(t, err)
	_, err = readFile(fileSystem1, "a_bad_id")
	assert.Error(t, err)
}
