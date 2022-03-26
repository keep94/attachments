package attachments

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNilFileSystem(t *testing.T) {
	nfs := NilFS()
	_, err := nfs.Open("abcd")
	assert.Equal(t, os.ErrNotExist, err)
	assert.False(t, nfs.Exists("abcd"))
	_, err = nfs.Write("abcd")
	assert.Equal(t, os.ErrPermission, err)
}

func TestWriteEmptyFile(t *testing.T) {
	fileSystem := NewInMemoryFS()
	writer, err := fileSystem.Write("empty")
	require.NoError(t, err)
	writer.Close()
	_, err = writer.Write([]byte("Hello"))
	assert.Equal(t, os.ErrClosed, err)
	assert.True(t, fileSystem.Exists("empty"))
	assert.False(t, fileSystem.Exists("not_exists"))
	_, err = readFile(fileSystem, "empty")
	assert.NoError(t, err)
	_, err = readFile(fileSystem, "not_exists")
	assert.Equal(t, os.ErrNotExist, err)
}
