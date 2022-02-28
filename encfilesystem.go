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

// aesFS is an encrypted file system storing immutable data
type aesFS struct {

	// The underlying file system
	FileSystem FS

	// The owner's AES encryption key. If nil, no encryption is done.
	Key []byte

	// The owner ID. Each owner has their own data encrypted with their
	// encryption key.
	OwnerId int64
}

// Write writes data to the underlying file system and returns the 64 digit
// hexadecimal SHA-256 checksum of that data.
func (a *aesFS) Write(contents []byte) (string, error) {
	binaryId := checksum(contents)
	id := hex.EncodeToString(binaryId)
	name := idToPath(id, a.OwnerId)
	if a.FileSystem.Exists(name) {
		return id, nil
	}
	if err := a.write(name, binaryId, contents); err != nil {
		return "", err
	}
	return id, nil
}

// Open returns a reader to retrieve data. checksum is the 64 digit hexadecimal
// checksum of the data that Write returned.
func (a *aesFS) Open(checksum string) (io.ReadCloser, error) {
	name := idToPath(checksum, a.OwnerId)
	var binaryId []byte
	var block cipher.Block
	var err error
	if a.Key != nil {
		binaryId, err = hex.DecodeString(checksum)
		if err != nil {
			return nil, err
		}
		block, err = aes.NewCipher(a.Key)
		if err != nil {
			return nil, err
		}
	}
	reader, err := a.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	if block != nil {
		reader = a.addDecryption(reader, block, binaryId)
	}
	return reader, nil
}

func (a *aesFS) write(
	name string, binaryId, contents []byte) error {
	var block cipher.Block
	var err error
	if a.Key != nil {
		block, err = aes.NewCipher(a.Key)
		if err != nil {
			return err
		}
	}
	writer, err := a.FileSystem.Write(name)
	if err != nil {
		return err
	}
	if block != nil {
		writer = a.addEncryption(writer, block, binaryId)
	}
	defer writer.Close()
	_, err = io.Copy(writer, bytes.NewReader(contents))
	return err
}

func (a *aesFS) addEncryption(
	writer io.WriteCloser,
	block cipher.Block,
	binaryId []byte) io.WriteCloser {
	stream := cipher.NewCFBEncrypter(block, iv(binaryId, a.OwnerId))
	return cipher.StreamWriter{S: stream, W: writer}
}

func (a *aesFS) addDecryption(
	reader io.ReadCloser,
	block cipher.Block,
	binaryId []byte) io.ReadCloser {
	stream := cipher.NewCFBDecrypter(block, iv(binaryId, a.OwnerId))
	streamReader := cipher.StreamReader{S: stream, R: reader}
	return &readerCloser{Reader: streamReader, Closer: reader}
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

// idToPath converts a 64 digit hexadecimal ID and ownerId to a path.
func idToPath(id string, ownerId int64) string {
	if len(id) < 2 {
		panic("ids should be 64 hexadecimal digits")
	}
	return fmt.Sprintf("%d/%s/%s", ownerId, id[:2], id)
}
