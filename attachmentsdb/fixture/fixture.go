// Package fixture provides test suites to test implementations of the
// datastore for attachments.
package fixture

import (
	"testing"

	"github.com/keep94/attachments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func EntryById(t *testing.T, store attachments.Store) {
	entry := attachments.Entry{
		Name:     "name",
		Size:     123,
		Ts:       1604123456,
		OwnerId:  2,
		Checksum: "123456789A",
	}
	require.NoError(t, store.AddEntry(nil, &entry))
	assert.Equal(t, int64(1), entry.Id)
	var fetchedEntry attachments.Entry
	require.NoError(t, store.EntryById(nil, 1, 2, &fetchedEntry))
	assert.Equal(t, entry, fetchedEntry)
	assert.Equal(
		t, attachments.ErrNoSuchId, store.EntryById(nil, 1, 3, &fetchedEntry))
	assert.Equal(
		t, attachments.ErrNoSuchId, store.EntryById(nil, 2, 2, &fetchedEntry))
}
