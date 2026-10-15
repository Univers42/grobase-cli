package secret

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fastKDF() KDF { return KDF{Time: 1, MemKiB: 64, Threads: 1} }

func pass(p string) func() ([]byte, error) {
	return func() ([]byte, error) { return []byte(p), nil }
}

func newFile(path, p string) *File { return NewFile(path, fastKDF(), pass(p)) }

func TestFileRoundTripAndNoPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "cred.enc")
	secretValue := "s3cr3t-" + t.Name() + "-value"
	f := newFile(path, "correct horse")
	if err := f.Set("prod/service_role", secretValue); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{secretValue, "prod/service_role", "service_role"} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Fatalf("file contains plaintext %q", needle)
		}
	}
	got, err := newFile(path, "correct horse").Get("prod/service_role")
	if err != nil || got != secretValue {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestFileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes only")
	}
	path := filepath.Join(t.TempDir(), "d", "cred.enc")
	if err := newFile(path, "pw").Set("a", "b"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", st.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode = %v, want 0700", dir.Mode().Perm())
	}
}

func TestFileWrongPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cred.enc")
	if err := newFile(path, "right").Set("a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := newFile(path, "wrong").Get("a"); !errors.Is(err, ErrBadPassphrase) {
		t.Fatalf("err = %v, want ErrBadPassphrase", err)
	}
}

func TestFileTamper(t *testing.T) {
	cases := map[string]func(d map[string]any){
		"ciphertext": func(d map[string]any) { d["ct"] = flip(d["ct"].(string)) },
		"nonce":      func(d map[string]any) { d["nonce"] = flip(d["nonce"].(string)) },
		"salt":       func(d map[string]any) { d["salt"] = flip(d["salt"].(string)) },
		"kdf params": func(d map[string]any) { d["params"].(map[string]any)["t"] = float64(2) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cred.enc")
			if err := newFile(path, "pw").Set("a", "b"); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(path)
			var d map[string]any
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			mutate(d)
			out, _ := json.Marshal(d)
			if err := os.WriteFile(path, out, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := newFile(path, "pw").Get("a"); err == nil {
				t.Fatal("tampered file was accepted")
			}
		})
	}
}

// flip changes one base64 character in the middle of s (staying valid base64).
func flip(s string) string {
	b := []byte(s)
	i := len(b) / 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

func TestFileRejectsHugeKDFHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cred.enc")
	hdr := `{"v":1,"cipher":"xchacha20poly1305","kdf":"argon2id","params":{"t":3,"m":4294967295,"p":4},"salt":"AA==","nonce":"AA==","ct":"AA=="}`
	if err := os.WriteFile(path, []byte(hdr), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newFile(path, "pw").Get("a"); err == nil {
		t.Fatal("expected rejection of unreasonable KDF memory")
	}
}

func TestFileDeleteAndNotFound(t *testing.T) {
	f := newFile(filepath.Join(t.TempDir(), "cred.enc"), "pw")
	if _, err := f.Get("x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := f.Set("x", "1"); err != nil {
		t.Fatal(err)
	}
	if err := f.Delete("x"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Get("x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestFreshNonceEverySave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cred.enc")
	f := newFile(path, "pw")
	var nonces [2]string
	for i := range nonces {
		if err := f.Set("k", "v"); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		var d fileDoc
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		nonces[i] = d.Nonce
	}
	if nonces[0] == nonces[1] {
		t.Fatal("nonce reused across saves")
	}
}

func TestOpenSelection(t *testing.T) {
	opts := Options{FilePath: filepath.Join(t.TempDir(), "c"), KDF: fastKDF(), Passphrase: pass("p")}
	cases := []struct {
		backend string
		kr      bool
		want    string
		wantErr bool
	}{
		{"auto", true, BackendKeyring, false},
		{"auto", false, BackendFile, false},
		{"keyring", false, "", true},
		{"file", true, BackendFile, false},
		{"bogus", true, "", true},
	}
	for _, c := range cases {
		o := opts
		o.Backend = c.backend
		o.KeyringProbe = func() error {
			if c.kr {
				return nil
			}
			return errors.New("no keyring")
		}
		_, got, err := Open(o)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%+v: got %q, %v", c, got, err)
		}
	}
}

func TestOpenAutoFallbackNamesTheProbeFailure(t *testing.T) {
	probeErr := errors.New("keyring set: dbus timeout")
	passErr := errors.New("credential file needs a passphrase")
	s, backend, err := Open(Options{
		FilePath:     filepath.Join(t.TempDir(), "c"),
		KDF:          fastKDF(),
		Passphrase:   func() ([]byte, error) { return nil, passErr },
		KeyringProbe: func() error { return probeErr },
	})
	if err != nil || backend != BackendFile {
		t.Fatalf("Open = %q, %v; want the file fallback", backend, err)
	}
	_, err = s.Get("prod/anon")
	if !errors.Is(err, probeErr) || !errors.Is(err, passErr) {
		t.Fatalf("Get error %q must carry both the probe and the passphrase failure", err)
	}
}
