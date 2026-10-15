package secret

import (
	"errors"
	"sync"
	"testing"

	"github.com/zalando/go-keyring"
)

// lineUpProbes routes the probe through go-keyring's mock, serialised (the mock is
// an unsynchronised map), and holds every probe after its write until all n have
// written: the interleaving two grobase processes in one pipeline produce. It
// returns the item names the probes wrote.
func lineUpProbes(t *testing.T, n int) func() map[string]bool {
	t.Helper()
	keyring.MockInit()
	set, del := keyringSet, keyringDelete
	t.Cleanup(func() { keyringSet, keyringDelete = set, del })
	var mu sync.Mutex
	var written sync.WaitGroup
	written.Add(n)
	names := map[string]bool{}
	keyringSet = func(service, user, password string) error {
		mu.Lock()
		names[user] = true
		err := set(service, user, password)
		mu.Unlock()
		written.Done()
		written.Wait()
		return err
	}
	keyringDelete = func(service, user string) error {
		mu.Lock()
		defer mu.Unlock()
		return del(service, user)
	}
	return func() map[string]bool {
		mu.Lock()
		defer mu.Unlock()
		return names
	}
}

func TestKeyringProbeAnswersOnceTheWriteLands(t *testing.T) {
	unreachable := errors.New("dbus: no secret service")
	cases := []struct {
		name      string
		setErr    error
		deleteErr error
		wantErr   bool
	}{
		{"write and delete", nil, nil, false},
		{"item already gone", nil, keyring.ErrNotFound, false},
		{"delete fails", nil, errors.New("dbus timeout"), false},
		{"write fails", unreachable, nil, true},
	}
	set, del := keyringSet, keyringDelete
	t.Cleanup(func() { keyringSet, keyringDelete = set, del })
	for _, c := range cases {
		keyringSet = func(string, string, string) error { return c.setErr }
		keyringDelete = func(string, string) error { return c.deleteErr }
		err := Keyring{}.Probe()
		if (err != nil) != c.wantErr || (c.setErr != nil && !errors.Is(err, c.setErr)) {
			t.Errorf("%s: Probe = %v", c.name, err)
		}
	}
}

func TestKeyringConcurrentProbesAllAnswer(t *testing.T) {
	const n = 16
	written := lineUpProbes(t, n)
	answers := make(chan error, n)
	for range n {
		go func() { answers <- Keyring{}.Probe() }()
	}
	unavailable := 0
	for range n {
		if err := <-answers; err != nil {
			t.Log(err)
			unavailable++
		}
	}
	if unavailable > 0 {
		t.Errorf("%d of %d concurrent probes reported no keyring", unavailable, n)
	}
	names := written()
	if len(names) != n {
		t.Errorf("%d probes wrote %d distinct items, want one each", n, len(names))
	}
	for name := range names {
		if _, err := keyring.Get(keyringService, name); !errors.Is(err, keyring.ErrNotFound) {
			t.Errorf("probe item %q left in the keyring (err %v)", name, err)
		}
	}
}
