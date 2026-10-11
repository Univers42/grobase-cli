package secret

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// KDF holds the argon2id cost parameters; they are written into the file header
// so a file stays readable if the defaults are raised later.
type KDF struct {
	Time    uint32 `json:"t"`
	MemKiB  uint32 `json:"m"`
	Threads uint8  `json:"p"`
}

// DefaultKDF follows the RFC 9106 "second recommended" profile (64 MiB, t=3, p=4).
func DefaultKDF() KDF { return KDF{Time: 3, MemKiB: 64 * 1024, Threads: 4} }

const (
	fileVersion = 1
	saltLen     = 16
	keyLen      = chacha20poly1305.KeySize
	maxKDFMem   = 1 << 21 // 2 GiB in KiB: refuse headers that would exhaust memory
)

// ErrBadPassphrase is returned when decryption fails: wrong passphrase or tampering.
var ErrBadPassphrase = errors.New("cannot decrypt credential file: wrong passphrase or file was modified")

type fileDoc struct {
	Version int    `json:"v"`
	Cipher  string `json:"cipher"`
	KDFName string `json:"kdf"`
	KDF     KDF    `json:"params"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	Data    string `json:"ct"`
}

// File is a Store backed by one encrypted file (argon2id + XChaCha20-Poly1305).
// The whole map is encrypted as one blob, so names are not visible on disk either.
type File struct {
	path       string
	kdf        KDF
	passphrase func() ([]byte, error)

	mu    sync.Mutex
	key   []byte
	salt  []byte
	items map[string]string
	ready bool
}

// NewFile returns a File store. passphrase is called lazily, at most once.
func NewFile(path string, kdf KDF, passphrase func() ([]byte, error)) *File {
	return &File{path: path, kdf: kdf, passphrase: passphrase}
}

// Get implements Store.
func (f *File) Get(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.load(); err != nil {
		return "", err
	}
	v, ok := f.items[name]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

// Set implements Store.
func (f *File) Set(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.load(); err != nil {
		return err
	}
	f.items[name] = value
	return f.flush()
}

// Delete implements Store.
func (f *File) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.load(); err != nil {
		return err
	}
	if _, ok := f.items[name]; !ok {
		return nil
	}
	delete(f.items, name)
	return f.flush()
}

func (f *File) derive(salt []byte, kdf KDF) error {
	pass, err := f.passphrase()
	if err != nil {
		return err
	}
	if len(pass) == 0 {
		return errors.New("empty passphrase")
	}
	f.key = argon2.IDKey(pass, salt, kdf.Time, kdf.MemKiB, kdf.Threads, keyLen)
	for i := range pass {
		pass[i] = 0
	}
	f.salt, f.kdf = salt, kdf
	return nil
}

func (f *File) load() error {
	if f.ready {
		return nil
	}
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		salt := make([]byte, saltLen)
		if _, err := rand.Read(salt); err != nil {
			return fmt.Errorf("generate salt: %w", err)
		}
		if err := f.derive(salt, f.kdf); err != nil {
			return err
		}
		f.items, f.ready = map[string]string{}, true
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", f.path, err)
	}
	return f.decrypt(raw)
}

func (f *File) decrypt(raw []byte) error {
	var d fileDoc
	if err := json.Unmarshal(raw, &d); err != nil || d.Version != fileVersion || d.KDFName != "argon2id" {
		return fmt.Errorf("%s is not a grobase credential file", f.path)
	}
	if d.KDF.MemKiB > maxKDFMem || d.KDF.Time == 0 || d.KDF.Threads == 0 || d.KDF.MemKiB == 0 {
		return fmt.Errorf("%s has unreasonable KDF parameters", f.path)
	}
	salt, e1 := base64.StdEncoding.DecodeString(d.Salt)
	nonce, e2 := base64.StdEncoding.DecodeString(d.Nonce)
	ct, e3 := base64.StdEncoding.DecodeString(d.Data)
	if err := errors.Join(e1, e2, e3); err != nil || len(nonce) != chacha20poly1305.NonceSizeX {
		return fmt.Errorf("%s is corrupt", f.path)
	}
	if err := f.derive(salt, d.KDF); err != nil {
		return err
	}
	aead, err := chacha20poly1305.NewX(f.key)
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}
	pt, err := aead.Open(nil, nonce, ct, header(d))
	if err != nil {
		f.key = nil
		return ErrBadPassphrase
	}
	items := map[string]string{}
	if err := json.Unmarshal(pt, &items); err != nil {
		return fmt.Errorf("%s decrypted but is corrupt: %w", f.path, err)
	}
	f.items, f.ready = items, true
	return nil
}

// header is the associated data: the KDF parameters and salt are authenticated, so
// downgrading the cost parameters in the file invalidates the tag.
func header(d fileDoc) []byte {
	return fmt.Appendf(nil, "grobase/%d/%s/%s/%d/%d/%d/%s", d.Version, d.Cipher, d.KDFName,
		d.KDF.Time, d.KDF.MemKiB, d.KDF.Threads, d.Salt)
}

func (f *File) flush() error {
	pt, err := json.Marshal(f.items)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	aead, err := chacha20poly1305.NewX(f.key)
	if err != nil {
		return fmt.Errorf("init cipher: %w", err)
	}
	nonce := make([]byte, chacha20poly1305.NonceSizeX)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}
	d := fileDoc{
		Version: fileVersion, Cipher: "xchacha20poly1305", KDFName: "argon2id", KDF: f.kdf,
		Salt: base64.StdEncoding.EncodeToString(f.salt), Nonce: base64.StdEncoding.EncodeToString(nonce),
	}
	d.Data = base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, pt, header(d)))
	out, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode credential file: %w", err)
	}
	return writePrivate(f.path, out)
}

// writePrivate writes atomically with mode 0600 in a 0700 directory.
func writePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".cred-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return errors.Join(fmt.Errorf("write credentials: %w", err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Join(fmt.Errorf("replace %s: %w", path, err), os.Remove(tmp.Name()))
	}
	return nil
}
