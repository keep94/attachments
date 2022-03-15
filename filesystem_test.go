package attachments

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilFileSystem(t *testing.T) {
	nfs := NilFS()
	_, err := nfs.Open("abcd")
	assert.Equal(t, os.ErrNotExist, err)
	assert.False(t, nfs.Exists("abcd"))
	_, err = nfs.Write("abcd")
	assert.Equal(t, os.ErrPermission, err)
}
