// Package attachments is for storing and retrieving file attachments.
package attachments

import (
	"bytes"
	"io"
	"os"
	"path"
	"sync"
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
// If root does not exist or is not a directory, NewFS returns os.ErrNotExist.
func NewFS(root string) (FS, error) {
	fileInfo, err := os.Stat(root)
	if err != nil || !fileInfo.IsDir() {
		return nil, os.ErrNotExist
	}
	return &realFS{root: root}, nil
}

// NilFS returns an empty file system that cannot be written to.
func NilFS() FS {
	return nilFS{}
}

// NewInMemoryFS returns a new in memory file system that can be used
// with multiple goroutines.
func NewInMemoryFS() FS {
	return &fakeFS{files: make(map[string][]byte)}
}

type fakeFS struct {
	lock  sync.Mutex
	files map[string][]byte
}

func (f *fakeFS) Open(name string) (io.ReadCloser, error) {
	contents := f.get(name)
	if contents == nil {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(contents)), nil
}

func (f *fakeFS) Write(name string) (io.WriteCloser, error) {
	return &fakeFSWriter{name: name, fs: f}, nil
}

func (f *fakeFS) Exists(name string) bool {
	contents := f.get(name)
	return contents != nil
}

func (f *fakeFS) get(key string) []byte {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.files[key]
}

func (f *fakeFS) put(key string, contents []byte) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.files[key] = contents
}

func (f *fakeFS) numFiles() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	return len(f.files)
}

func numFiles(fileSystem FS) int {
	return fileSystem.(*fakeFS).numFiles()
}

type fakeFSWriter struct {
	buffer bytes.Buffer
	name   string
	fs     *fakeFS
}

func (f *fakeFSWriter) Write(p []byte) (n int, err error) {
	if f.fs == nil {
		return 0, os.ErrClosed
	}
	return f.buffer.Write(p)
}

func (f *fakeFSWriter) Close() error {
	if f.fs != nil {
		f.fs.put(f.name, f.buffer.Bytes())
		f.fs = nil
	}
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

type nilFS struct {
}

func (n nilFS) Open(name string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (n nilFS) Exists(name string) bool {
	return false
}

func (n nilFS) Write(name string) (io.WriteCloser, error) {
	return nil, os.ErrPermission
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

func readBytes(fileSystem rofs, name string) []byte {
	result, err := readFile(fileSystem, name)
	if err != nil {
		return nil
	}
	return result
}
