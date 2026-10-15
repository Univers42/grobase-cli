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
	// KeyringProbe overrides the keyring probe (tests); nil means a keyring answered.
	KeyringProbe func() error
}

// Open returns the requested Store and the backend actually used. "auto" prefers
// the OS keyring and falls back to the encrypted file when no keyring answers.
func Open(o Options) (Store, string, error) {
	probe := o.KeyringProbe
	if probe == nil {
		probe = Keyring{}.Probe
	}
	switch o.Backend {
	case "", BackendAuto:
		err := probe()
		if err == nil {
			return Keyring{}, BackendKeyring, nil
		}
		return o.fallback(err), BackendFile, nil
	case BackendKeyring:
		if err := probe(); err != nil {
			return nil, "", fmt.Errorf("the OS keyring is not available (%w); use --credential-store file", err)
		}
		return Keyring{}, BackendKeyring, nil
	case BackendFile:
		return o.file(), BackendFile, nil
	}
	return nil, "", fmt.Errorf("unknown credential store %q (want auto, keyring or file)", o.Backend)
}

func (o Options) file() *File { return NewFile(o.FilePath, o.KDF, o.Passphrase) }

// fallback is the file store auto mode settles for when the keyring probe failed. A
// failed passphrase is where that choice bites, credentials kept in a keyring that
// only hiccuped included, so its error carries the probe's: the switch is never silent.
func (o Options) fallback(probeErr error) *File {
	return NewFile(o.FilePath, o.KDF, func() ([]byte, error) {
		p, err := o.Passphrase()
		if err != nil {
			return nil, fmt.Errorf("no OS keyring answered (%w), so the credential file is used: %w", probeErr, err)
		}
		return p, nil
	})
}
