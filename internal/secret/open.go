package secret

import "fmt"

// Backend names accepted by Open.
const (
	BackendAuto    = "auto"
	BackendKeyring = "keyring"
	BackendFile    = "file"
)

// Options selects and configures a backend.
type Options struct {
	Backend    string
	FilePath   string
	KDF        KDF
	Passphrase func() ([]byte, error)
	// KeyringAvailable overrides the keyring probe (tests).
	KeyringAvailable func() bool
}

// Open returns the requested Store and the backend actually used. "auto" prefers
// the OS keyring and falls back to the encrypted file when no keyring answers.
func Open(o Options) (Store, string, error) {
	probe := o.KeyringAvailable
	if probe == nil {
		probe = Keyring{}.Available
	}
	switch o.Backend {
	case "", BackendAuto:
		if probe() {
			return Keyring{}, BackendKeyring, nil
		}
		return o.file(), BackendFile, nil
	case BackendKeyring:
		if !probe() {
			return nil, "", fmt.Errorf("the OS keyring is not available; use --credential-store file")
		}
		return Keyring{}, BackendKeyring, nil
	case BackendFile:
		return o.file(), BackendFile, nil
	}
	return nil, "", fmt.Errorf("unknown credential store %q (want auto, keyring or file)", o.Backend)
}

func (o Options) file() *File { return NewFile(o.FilePath, o.KDF, o.Passphrase) }
