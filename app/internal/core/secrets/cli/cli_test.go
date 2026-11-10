package cli

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/core/secrets"
	workspacecli "github.com/go-envx/envx/app/internal/core/workspace/cli"
	"github.com/go-envx/envx/app/internal/lib/crypt"
	"github.com/go-envx/envx/app/internal/lib/crypt/age"
	"github.com/spf13/cobra"
)

// testConfig is the manifest path every command test passes with --config.
const testConfig = "/ws/envx.yaml"

// fakeFactory builds a real secrets service over an in-memory store and
// keyring with the age cipher.
type fakeFactory struct {
	// configPath records the manifest path the command requested.
	configPath string
	// err, when set, is returned instead of a service.
	err error
	// store holds the secret groups.
	store *memStore
	// keys holds the retained private keys.
	keys *memKeys
}

// memStore is an in-memory secrets.Store with integer revisions.
type memStore struct {
	// groups holds stored groups by lowercase name.
	groups map[string]secrets.Group
	// revisions counts writes per group.
	revisions map[string]int
}

// memKeys indexes retained private keys by group and public key.
type memKeys struct {
	// byPublic maps group, then public key, to the retained private key.
	byPublic map[string]map[string]string
}

// commandCase is one secrets command with arguments that succeed against
// seededFactory.
type commandCase struct {
	// name labels the command.
	name string
	// args are the subcommand path, positional arguments, and flags.
	args []string
}

// newFakeFactory returns a factory with an empty store and keyring.
func newFakeFactory() *fakeFactory {
	return &fakeFactory{
		store: &memStore{
			groups:    make(map[string]secrets.Group),
			revisions: make(map[string]int),
		},
		keys: &memKeys{byPublic: make(map[string]map[string]string)},
	}
}

// seededFactory returns a factory whose "production" group has a keypair and
// one encrypted secret, api_key = "s3cret".
func seededFactory(t *testing.T) *fakeFactory {
	t.Helper()
	f := newFakeFactory()
	service := f.service(t)
	if _, err := service.GenerateKeypair("production"); err != nil {
		t.Fatalf("GenerateKeypair(): %v", err)
	}
	if _, err := service.SetSecret("production", "api_key", func() (string, error) {
		return "s3cret", nil
	}); err != nil {
		t.Fatalf("SetSecret(): %v", err)
	}
	return f
}

// NewSecretsService implements Factory.
func (f *fakeFactory) NewSecretsService(configPath string) (*secrets.Service, error) {
	f.configPath = configPath
	if f.err != nil {
		return nil, f.err
	}
	return secrets.New(f.store, f.keys, age.Cipher{})
}

// Location implements secrets.Store.
func (s *memStore) Location() string {
	return "mem://secrets"
}

// Read implements secrets.Reader, returning copies of stored values.
func (s *memStore) Read(group string) (secrets.Snapshot, error) {
	name := strings.ToLower(group)
	stored, ok := s.groups[name]
	if !ok {
		return secrets.Snapshot{}, nil
	}
	stored.Values = maps.Clone(stored.Values)
	return secrets.Snapshot{
		Group:    stored,
		Revision: fmt.Sprint(s.revisions[name]),
		Exists:   true,
	}, nil
}

// List implements secrets.Store in group-name order.
func (s *memStore) List() ([]secrets.Snapshot, error) {
	var out []secrets.Snapshot
	for _, name := range slices.Sorted(maps.Keys(s.groups)) {
		snap, _ := s.Read(name)
		out = append(out, snap)
	}
	return out, nil
}

// Commit implements secrets.Store with revision checking.
func (s *memStore) Commit(expected secrets.Snapshot, replacement secrets.Group) error {
	current, _ := s.Read(replacement.Name)
	if current.Exists != expected.Exists || current.Revision != expected.Revision {
		return secrets.ErrConflict
	}
	name := strings.ToLower(replacement.Name)
	replacement.Values = maps.Clone(replacement.Values)
	s.groups[name] = replacement
	s.revisions[name]++
	return nil
}

// Location implements secrets.Keys.
func (k *memKeys) Location() string {
	return "mem://keys"
}

// Resolve implements secrets.KeySource.
func (k *memKeys) Resolve(group, publicKey string) (secrets.PrivateKey, error) {
	if key, ok := k.byPublic[group][publicKey]; ok {
		return secrets.PrivateKey{Value: key, Origin: secrets.OriginStore}, nil
	}
	return secrets.PrivateKey{}, secrets.ErrKeyNotAvailable
}

// Retain implements secrets.Keys.
func (k *memKeys) Retain(group string, pair crypt.Keypair) error {
	if k.byPublic[group] == nil {
		k.byPublic[group] = make(map[string]string)
	}
	k.byPublic[group][pair.PublicKey] = pair.PrivateKey
	return nil
}

// service builds a service for seeding and inspection without recording a
// request.
func (f *fakeFactory) service(t *testing.T) *secrets.Service {
	t.Helper()
	service, err := secrets.New(f.store, f.keys, age.Cipher{})
	if err != nil {
		t.Fatalf("secrets.New(): %v", err)
	}
	return service
}

// stored returns the raw stored value of group/key.
func (f *fakeFactory) stored(group, key string) string {
	return f.store.groups[group].Values[key]
}

// execute builds the "secrets" and "keypair" commands under one parent with
// --config bound, runs args, and returns the captured stdout and stderr.
func execute(
	t *testing.T, factory Factory, stdin string, args ...string,
) (stdout, stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "envx", SilenceUsage: true}
	cmd.AddCommand(NewSecretsCommand(factory), NewKeypairCommand(factory))
	workspacecli.BindConfig(cmd)
	cmd.SetArgs(append([]string{"--config", testConfig}, args...))

	var out, errBuf bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// commandCases lists every secrets and keypair command.
func commandCases() []commandCase {
	return []commandCase{
		{
			name: "secrets get",
			args: []string{"secrets", "get", "production", "api_key"},
		},
		{name: "secrets set", args: []string{"secrets", "set", "production", "k", "v"}},
		{
			name: "secrets delete",
			args: []string{"secrets", "delete", "production", "api_key"},
		},
		{name: "secrets encrypt", args: []string{"secrets", "encrypt"}},
		{name: "secrets decrypt", args: []string{"secrets", "decrypt"}},
		{name: "keypair generate", args: []string{"keypair", "generate", "staging"}},
		{name: "keypair inspect", args: []string{"keypair", "inspect", "production"}},
		{name: "keypair rotate", args: []string{"keypair", "rotate", "production"}},
	}
}

// TestCommandsPassConfigPath verifies every command builds its service from the
// --config path.
func TestCommandsPassConfigPath(t *testing.T) {
	t.Parallel()

	for _, tt := range commandCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := seededFactory(t)
			if _, _, err := execute(t, factory, "", tt.args...); err != nil {
				t.Fatalf("Execute(): %v", err)
			}
			if factory.configPath != testConfig {
				t.Errorf("configPath = %q, want %q", factory.configPath, testConfig)
			}
		})
	}
}

// TestCommandsReturnFactoryError verifies every command returns a factory
// failure unchanged and writes nothing.
func TestCommandsReturnFactoryError(t *testing.T) {
	t.Parallel()

	for _, tt := range commandCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want := errors.New("factory failure")
			factory := newFakeFactory()
			factory.err = want
			stdout, _, err := execute(t, factory, "", tt.args...)
			if !errors.Is(err, want) {
				t.Fatalf("err = %v, want %v", err, want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing", stdout)
			}
		})
	}
}
