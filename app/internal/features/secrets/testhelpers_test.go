package secrets

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-envx/envx/app/internal/features/privatekey"
	"github.com/go-envx/envx/app/internal/resources/cipher"
	"github.com/go-envx/envx/app/internal/shared/value"
	"gopkg.in/yaml.v3"
)

var (
	testStoreMu    sync.Mutex
	testStoreMap   = make(map[string]*fakeRepository)
	testStoreCount int
)

// parseStoreYAML parses a YAML string into an in-memory fakeRepository.
func parseStoreYAML(body string) *fakeRepository {
	repo := newFakeRepository()
	var doc struct {
		PublicKeys map[string]string            `yaml:"public-keys"`
		Secrets    map[string]map[string]string `yaml:"secrets"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		repo.err = err
		return repo
	}
	for g, k := range doc.PublicKeys {
		_ = repo.SetPublicKey(g, k)
	}
	for g, keys := range doc.Secrets {
		for k, val := range keys {
			_ = repo.SetSecret(SecretRecord{
				Group:      g,
				Key:        k,
				Ciphertext: val,
			})
		}
	}
	return repo
}

// writeStore writes body into an in-memory test store and returns an identifier.
func writeStore(t *testing.T, body string) string {
	t.Helper()
	testStoreMu.Lock()
	defer testStoreMu.Unlock()
	testStoreCount++
	id := fmt.Sprintf("/fake/store-%d.yaml", testStoreCount)
	repo := parseStoreYAML(body)
	repo.location = id
	testStoreMap[id] = repo
	return id
}

// getTestStore retrieves or initializes the in-memory fakeRepository for path.
func getTestStore(id string) *fakeRepository {
	testStoreMu.Lock()
	defer testStoreMu.Unlock()
	if repo, ok := testStoreMap[id]; ok {
		return repo
	}
	repo := newFakeRepository()
	repo.location = id
	testStoreMap[id] = repo
	return repo
}

// newTestStore returns the fake repository for id.
func newTestStore(t *testing.T, id string) *fakeRepository {
	t.Helper()
	return getTestStore(id)
}

// newTestCipher creates the default cipher for service construction tests.
func newTestCipher(t *testing.T) CipherClient {
	t.Helper()
	selected, err := cipher.New(cipher.Params{
		Algorithm: cipher.Age,
		Options:   cipher.AgeOptions{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

// testKeypairFor generates a keypair with the default test cipher.
func testKeypairFor(t *testing.T) value.Keypair {
	t.Helper()
	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	return pair
}

// testKeys generates a keypair with c and returns its halves.
func testKeys(t *testing.T, c CipherClient) (publicKey, privateKey string) {
	t.Helper()
	pair, err := c.Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	return pair.PublicKey, pair.PrivateKey
}

// newPrivateKeyTestService creates a service double for service construction tests.
func newPrivateKeyTestService() PrivateKeyService {
	return testPrivateKeyService{}
}

// newPrivateKeyTestResolver creates a service double for test helpers.
func newPrivateKeyTestResolver() PrivateKeyService {
	return testPrivateKeyService{}
}

// testPrivateKeyService is a no-op private-key service for service construction tests.
type testPrivateKeyService struct{}

// Location reports a dummy location for test service.
func (testPrivateKeyService) Location() string { return "test-keys-location" }

// Resolve reports that no private key is available.
func (testPrivateKeyService) Resolve(string) (privatekey.PrivateKey, error) {
	return privatekey.PrivateKey{}, privatekey.ErrNotAvailable
}

// Set accepts private-key material without storing it.
func (testPrivateKeyService) Set(string, string) error { return nil }

// fakeRepository provides an in-memory repository for unit testing without disk I/O.
type fakeRepository struct {
	location   string
	publicKeys map[string]string
	secrets    map[string]SecretRecord
	err        error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		location:   "/fake/store.yaml",
		publicKeys: make(map[string]string),
		secrets:    make(map[string]SecretRecord),
	}
}

func (f *fakeRepository) GetPublicKey(
	group string,
) (key string, found bool, err error) {
	if f.err != nil {
		return "", false, f.err
	}
	key, ok := f.publicKeys[strings.ToLower(group)]
	return key, ok, nil
}

func (f *fakeRepository) SetPublicKey(group, publicKey string) error {
	if f.err != nil {
		return f.err
	}
	f.publicKeys[strings.ToLower(group)] = publicKey
	return nil
}

func (f *fakeRepository) ListKeypairs() ([]KeypairRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	records := make([]KeypairRecord, 0, len(f.publicKeys))
	for g, k := range f.publicKeys {
		records = append(records, KeypairRecord{Group: g, PublicKey: k})
	}
	return records, nil
}

func (f *fakeRepository) GetSecret(group, key string) (SecretRecord, bool, error) {
	if f.err != nil {
		return SecretRecord{}, false, f.err
	}
	sec, ok := f.secrets[strings.ToLower(group)+"/"+key]
	return sec, ok, nil
}

func (f *fakeRepository) SetSecret(record SecretRecord) error {
	if f.err != nil {
		return f.err
	}
	rec := SecretRecord{
		Group:      record.Group,
		Key:        record.Key,
		Ciphertext: record.Ciphertext,
	}
	f.secrets[strings.ToLower(record.Group)+"/"+record.Key] = rec
	return nil
}

func (f *fakeRepository) SetSecrets(records []SecretRecord) error {
	for _, rec := range records {
		if err := f.SetSecret(rec); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeRepository) DeleteSecret(group, key string) error {
	if f.err != nil {
		return f.err
	}
	k := strings.ToLower(group) + "/" + key
	if _, ok := f.secrets[k]; !ok {
		return ErrSecretNotFound
	}
	delete(f.secrets, k)
	return nil
}

func (f *fakeRepository) ListSecrets() ([]SecretRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	records := make([]SecretRecord, 0, len(f.secrets))
	for _, s := range f.secrets {
		records = append(records, s)
	}
	return records, nil
}

func (f *fakeRepository) Location() string {
	if f.location != "" {
		return f.location
	}
	return "/fake/store.yaml"
}

// storedValueFrom retrieves a secret value from manager's repository.
func storedValueFrom(t *testing.T, manager *Service, group, key string) string {
	t.Helper()
	rec, found, err := manager.params.Repository.GetSecret(group, key)
	if err != nil {
		t.Fatalf("GetSecret(): %v", err)
	}
	if !found {
		t.Fatalf("secret %q not found in group %q", key, group)
	}
	return rec.Ciphertext
}
