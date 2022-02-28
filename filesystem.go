// Package attachments is for storing and retrieving file attachments.
package attachments

import (
	"bytes"
	"io"
	"os"
	"path"
)

// FS is a very simple file system.
type FS interface {
	// Open opens a file
	Open(name string) (io.ReadCloser, error)

	// Write writes a file.
	Write(name string) (io.WriteCloser, error)

	// Exists returns true if file with given path exists.
	Exists(name string) bool
}

// NewFS returns a file system backed by disk rooted at path root.
func NewFS(root string) FS {
	fileInfo, err := os.Stat(root)
	if err != nil || !fileInfo.IsDir() {
		return nil
	}
	return &realFS{root: root}
}

// FakeFS is an in-memory implementation of FS for testing.
// The keys are the file names, the values are the file contents.
type FakeFS map[string][]byte

func (f FakeFS) Open(name string) (io.ReadCloser, error) {
	contents, ok := f[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(contents)), nil
}

func (f FakeFS) Write(name string) (io.WriteCloser, error) {
	return &fakeFSWriter{name: name, files: f}, nil
}

func (f FakeFS) Exists(name string) bool {
	_, ok := f[name]
	return ok
}

type fakeFSWriter struct {
	buffer   bytes.Buffer
	name     string
	files    FakeFS
	isClosed bool
}

func (f *fakeFSWriter) Write(p []byte) (n int, err error) {
	if f.isClosed {
		return 0, os.ErrClosed
	}
	return f.buffer.Write(p)
}

func (f *fakeFSWriter) Close() error {
	f.files[f.name] = f.buffer.Bytes()
	f.isClosed = true
	return nil
}

type realFS struct {
	root string
}

func (r *realFS) Open(name string) (io.ReadCloser, error) {
	return os.Open(r.fullPath(name))
}

func (r *realFS) Write(name string) (io.WriteCloser, error) {
	fullPath := r.fullPath(name)
	if err := os.MkdirAll(path.Dir(fullPath), 0700); err != nil {
		return nil, err
	}
	return os.Create(fullPath)
}

func (r *realFS) Exists(name string) bool {
	fileInfo, err := os.Stat(r.fullPath(name))
	return err == nil && !fileInfo.IsDir()
}

func (r *realFS) fullPath(name string) string {
	return path.Join(r.root, name)
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
