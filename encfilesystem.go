package attachments

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

// IdToPath converts a 64 digit hexadecimal ID and ownerId to a path.
func IdToPath(id string, ownerId int64) string {
	if len(id) < 2 {
		panic("ids should be 64 hexadecimal digits")
	}
	return fmt.Sprintf("%d/%s/%s", ownerId, id[:2], id)
}

// AESFS is an encrypted file system storing immutable data
type AESFS struct {

	// The underlying file system
	FileSystem FS

	// The owner's AES encryption key. If nil, no encryption is done.
	Key []byte

	// The owner ID. Each owner has their own data encrypted with their
	// encryption key.
	OwnerId int64
}

// Write writes data to the underlying file system and returns a 64 digit
// hexadecimal ID to that data. The ID is actually the SHA-256 checksum of
// that data.
func (a *AESFS) Write(contents []byte) (string, error) {
	binaryId := checksum(contents)
	id := hex.EncodeToString(binaryId)
	name := IdToPath(id, a.OwnerId)
	if a.FileSystem.Exists(name) {
		return id, nil
	}
	if a.Key == nil {
		if err := WriteFile(a.FileSystem, name, contents); err != nil {
			return "", err
		}
		return id, nil
	}
	if err := a.writeEncrypted(name, binaryId, contents); err != nil {
		return "", err
	}
	return id, nil
}

// Open returns a reader to retrieve data. id is the 64 digit hexadecimal
// ID to the data.
func (a *AESFS) Open(id string) (io.ReadCloser, error) {
	name := IdToPath(id, a.OwnerId)
	if a.Key == nil {
		return a.FileSystem.Open(name)
	}
	binaryId, err := hex.DecodeString(id)
	if err != nil {
		return nil, err
	}
	return a.openEncrypted(name, binaryId)
}

func (a *AESFS) writeEncrypted(
	name string, binaryId, contents []byte) error {
	writer, err := a.FileSystem.Write(name)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(a.Key)
	if err != nil {
		return err
	}
	stream := cipher.NewCFBEncrypter(block, iv(binaryId, a.OwnerId))
	streamWriter := cipher.StreamWriter{S: stream, W: writer}
	defer streamWriter.Close()
	_, err = io.Copy(streamWriter, bytes.NewReader(contents))
	return err
}

func (a *AESFS) openEncrypted(
	name string, binaryId []byte) (io.ReadCloser, error) {
	reader, err := a.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(a.Key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCFBDecrypter(block, iv(binaryId, a.OwnerId))
	streamReader := cipher.StreamReader{S: stream, R: reader}
	return &readerCloser{Reader: streamReader, Closer: reader}, nil
}

type readerCloser struct {
	io.Reader
	io.Closer
}

func checksum(contents []byte) []byte {
	hash := sha256.New()
	hash.Write(contents)
	return hash.Sum(nil)
}

func iv(checksum []byte, owner int64) []byte {
	hash := sha256.New()
	hash.Write(checksum)
	binary.Write(hash, binary.LittleEndian, owner)
	return hash.Sum(nil)[:aes.BlockSize]
}
