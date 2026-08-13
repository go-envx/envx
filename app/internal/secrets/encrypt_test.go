package secrets

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-envx/envx/app/internal/privatekey"
	"github.com/go-envx/envx/app/internal/secrets/internal/envelope"
	"github.com/go-envx/envx/app/internal/secrets/internal/store"
)

// newBulkManager builds a manager over the given store body using the age cipher
// and the provided private-key resolver.
func newBulkManager(
	t *testing.T, body string, resolver privatekey.Resolver,
) (manager *Manager, storePath string) {
	t.Helper()
	storePath = writeStore(t, body)
	var err error
	manager, err = New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                newTestCipher(t),
		PrivateKeyResolver:    resolver,
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return manager, storePath
}

// storedValue returns one stored secret value or fails the test.
func storedValue(t *testing.T, storePath, group, key string) string {
	t.Helper()
	document, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	secret, exists := document.Secret(group, key)
	if !exists {
		t.Fatalf("secret %q not found in group %q", key, group)
	}
	return secret.Value
}

// assertDecrypts fails unless the stored value round-trips to want under key.
func assertDecrypts(t *testing.T, storePath, group, key, privateKey, want string) {
	t.Helper()
	value := storedValue(t, storePath, group, key)
	_, payload, err := envelope.Decode(value)
	if err != nil {
		t.Fatalf("Decode(%q/%q): %v", group, key, err)
	}
	got, err := newTestCipher(t).Decrypt(payload, privateKey)
	if err != nil {
		t.Fatalf("Decrypt(%q/%q): %v", group, key, err)
	}
	if got != want {
		t.Errorf("round trip %q/%q = %q, want %q", group, key, got, want)
	}
}

// references collects an update result's identities into a comparable set.
func referenceSet(result UpdateResult) map[SecretReference]struct{} {
	set := make(map[SecretReference]struct{}, len(result.Secrets))
	for _, ref := range result.Secrets {
		set[ref] = struct{}{}
	}
	return set
}

// TestEncryptEncryptsMatchingPlaintext verifies a wildcard selector encrypts
// every plaintext value, reports each changed identity, and stores no plaintext.
func TestEncryptEncryptsMatchingPlaintext(t *testing.T) {
	t.Parallel()

	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\nsecrets:\n  production:\n"+
			"    api_key: plain-api\n    db_password: plain-db\n",
		pair.PublicKey,
	)
	manager, storePath := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: pair.PrivateKey,
	})

	result, err := manager.Encrypt("", "")
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}

	got := referenceSet(result)
	want := map[SecretReference]struct{}{
		{Group: "production", Key: "api_key"}:     {},
		{Group: "production", Key: "db_password"}: {},
	}
	if len(got) != len(want) {
		t.Fatalf("changed identities = %v, want %v", result.Secrets, want)
	}
	for ref := range want {
		if _, ok := got[ref]; !ok {
			t.Errorf("missing changed identity %v", ref)
		}
	}
	assertDecrypts(t, storePath, "production", "api_key", pair.PrivateKey, "plain-api")
	assertDecrypts(t, storePath, "production", "db_password", pair.PrivateKey, "plain-db")
}

// TestEncryptSkipsCiphertextAndReportsOnlyChanged verifies values already in the
// ciphertext target state are left untouched and excluded from the result.
func TestEncryptSkipsCiphertextAndReportsOnlyChanged(t *testing.T) {
	t.Parallel()

	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\nsecrets:\n  production:\n"+
			"    api_key: plain-api\n    token: plain-token\n",
		pair.PublicKey,
	)
	manager, storePath := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: pair.PrivateKey,
	})

	// Encrypt only api_key first, then a wildcard pass must skip that now-
	// ciphertext entry and change only the remaining plaintext value.
	if _, err := manager.Encrypt("production", "api_key"); err != nil {
		t.Fatalf("Encrypt() first: %v", err)
	}
	firstCiphertext := storedValue(t, storePath, "production", "api_key")

	result, err := manager.Encrypt("", "")
	if err != nil {
		t.Fatalf("Encrypt() second: %v", err)
	}
	if len(result.Secrets) != 1 ||
		result.Secrets[0] != (SecretReference{Group: "production", Key: "token"}) {
		t.Fatalf("changed identities = %v, want only production/token", result.Secrets)
	}
	if got := storedValue(t, storePath, "production", "api_key"); got != firstCiphertext {
		t.Error("Encrypt() re-encrypted an already-ciphertext value")
	}
}

// TestEncryptIsIdempotent verifies re-encrypting a fully encrypted store changes
// nothing and reports no identities.
func TestEncryptIsIdempotent(t *testing.T) {
	t.Parallel()

	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\nsecrets:\n  production:\n"+
			"    api_key: plain-api\n",
		pair.PublicKey,
	)
	manager, _ := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: pair.PrivateKey,
	})

	if _, err := manager.Encrypt("", ""); err != nil {
		t.Fatalf("Encrypt() first: %v", err)
	}
	result, err := manager.Encrypt("", "")
	if err != nil {
		t.Fatalf("Encrypt() second: %v", err)
	}
	if len(result.Secrets) != 0 {
		t.Errorf("second Encrypt changed %v, want nothing", result.Secrets)
	}
}

// TestEncryptRejectsSelectorMatchingNothing verifies an explicit selector that
// matches no stored entry is an error.
func TestEncryptRejectsSelectorMatchingNothing(t *testing.T) {
	t.Parallel()

	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\nsecrets:\n  production:\n"+
			"    api_key: plain-api\n",
		pair.PublicKey,
	)
	manager, _ := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: pair.PrivateKey,
	})

	for _, test := range []struct {
		name  string
		group string
		key   string
	}{
		{name: "unknown group", group: "staging"},
		{name: "unknown key", key: "missing"},
		{name: "unknown group and key", group: "production", key: "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := manager.Encrypt(test.group, test.key); err == nil {
				t.Errorf("Encrypt(%q, %q) accepted a selector matching nothing",
					test.group, test.key)
			}
		})
	}
}

// TestEncryptScopedByGroup verifies a group selector encrypts only that group.
func TestEncryptScopedByGroup(t *testing.T) {
	t.Parallel()

	prod, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair() production: %v", err)
	}
	shared, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair() shared: %v", err)
	}
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\n  shared: %s\n"+
			"secrets:\n  production:\n    api_key: plain-api\n"+
			"  shared:\n    token: plain-token\n",
		prod.PublicKey, shared.PublicKey,
	)
	manager, storePath := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: prod.PrivateKey,
	})

	result, err := manager.Encrypt("Production", "")
	if err != nil {
		t.Fatalf("Encrypt(): %v", err)
	}
	if len(result.Secrets) != 1 ||
		result.Secrets[0] != (SecretReference{Group: "production", Key: "api_key"}) {
		t.Fatalf("changed identities = %v, want only production/api_key", result.Secrets)
	}
	assertDecrypts(t, storePath, "production", "api_key", prod.PrivateKey, "plain-api")
	if got := storedValue(t, storePath, "shared", "token"); got != "plain-token" {
		t.Errorf("out-of-scope value = %q, want untouched plaintext", got)
	}
}

// TestEncryptValidatesBeforeWrite verifies a matching value whose group has no
// public key aborts the operation and leaves the store unchanged.
func TestEncryptValidatesBeforeWrite(t *testing.T) {
	t.Parallel()

	pair, err := newTestCipher(t).Keypair()
	if err != nil {
		t.Fatalf("Keypair(): %v", err)
	}
	// The keyed group precedes the keyless group in document order, so a naive
	// implementation would write the first before failing on the second.
	body := fmt.Sprintf(
		"public_keys:\n  production: %s\n"+
			"secrets:\n  production:\n    api_key: plain-api\n"+
			"  orphan:\n    token: plain-token\n",
		pair.PublicKey,
	)
	manager, storePath := newBulkManager(t, body, fixedPrivateKeyResolver{
		value: pair.PrivateKey,
	})

	if _, err := manager.Encrypt("", ""); err == nil {
		t.Fatal("Encrypt() succeeded with a group missing its public key")
	}
	if got := storedValue(t, storePath, "production", "api_key"); got != "plain-api" {
		t.Errorf("keyed group value = %q, want unchanged plaintext", got)
	}
}

// TestEncryptRejectsInvalidSelector verifies malformed selector dimensions are
// rejected before the store is opened.
func TestEncryptRejectsInvalidSelector(t *testing.T) {
	t.Parallel()

	manager, _ := newBulkManager(t, "public_keys: {}\n", fixedPrivateKeyResolver{})

	if _, err := manager.Encrypt("bad/group", ""); err == nil {
		t.Error("Encrypt() accepted an invalid group")
	}
	if _, err := manager.Encrypt("", "bad/key"); err == nil {
		t.Error("Encrypt() accepted an invalid key")
	}
}

// TestDecryptDecryptsMatchingCiphertext verifies a wildcard selector decrypts
// every ciphertext value in place and reports each changed identity.
func TestDecryptDecryptsMatchingCiphertext(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})
	set := func(key, value string) {
		if err := manager.Set("production", key, func() (string, error) {
			return value, nil
		}); err != nil {
			t.Fatalf("Set(%q): %v", key, err)
		}
	}
	set("api_key", "plain-api")
	set("db_password", "plain-db")

	result, err := manager.Decrypt("", "")
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if len(result.Secrets) != 2 {
		t.Fatalf("changed identities = %v, want two", result.Secrets)
	}

	path := manager.params.SecretsPath
	if got := storedValue(t, path, "production", "api_key"); got != "plain-api" {
		t.Errorf("api_key = %q, want decrypted plaintext", got)
	}
	if got := storedValue(t, path, "production", "db_password"); got != "plain-db" {
		t.Errorf("db_password = %q, want decrypted plaintext", got)
	}
}

// TestDecryptSkipsPlaintextIdempotent verifies decrypting an already-plaintext
// store changes nothing and reports no identities.
func TestDecryptSkipsPlaintextIdempotent(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})
	if err := manager.Set("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	if _, err := manager.Decrypt("", ""); err != nil {
		t.Fatalf("Decrypt() first: %v", err)
	}

	result, err := manager.Decrypt("", "")
	if err != nil {
		t.Fatalf("Decrypt() second: %v", err)
	}
	if len(result.Secrets) != 0 {
		t.Errorf("second Decrypt changed %v, want nothing", result.Secrets)
	}
}

// TestDecryptRejectsSelectorMatchingNothing verifies an explicit selector that
// matches no stored entry is an error.
func TestDecryptRejectsSelectorMatchingNothing(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, fixedPrivateKeyResolver{})
	if err := manager.Set("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	if _, err := manager.Decrypt("staging", ""); err == nil {
		t.Error("Decrypt() accepted a selector matching nothing")
	}
}

// TestDecryptResolvesKeysLazilyByGroup verifies a group selector decrypts only
// that group and resolves its private key once, never touching other groups.
func TestDecryptResolvesKeysLazilyByGroup(t *testing.T) {
	t.Parallel()

	selected := newTestCipher(t)
	prod, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() production: %v", err)
	}
	shared, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() shared: %v", err)
	}
	resolver := &recordingResolver{
		keys: map[string]string{
			"production": prod.PrivateKey,
			"shared":     shared.PrivateKey,
		},
		calls: map[string]int{},
	}
	storePath := writeStore(t, fmt.Sprintf(
		"public_keys:\n  production: %s\n  shared: %s\n",
		prod.PublicKey, shared.PublicKey,
	))
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    resolver,
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	set := func(group, key, value string) {
		if err := manager.Set(group, key, func() (string, error) {
			return value, nil
		}); err != nil {
			t.Fatalf("Set(%q/%q): %v", group, key, err)
		}
	}
	set("production", "first", "prod-first")
	set("production", "second", "prod-second")
	set("shared", "token", "shared-token")

	result, err := manager.Decrypt("production", "")
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if len(result.Secrets) != 2 {
		t.Fatalf("changed identities = %v, want two production entries", result.Secrets)
	}
	if resolver.calls["production"] != 1 {
		t.Errorf("production key resolved %d times, want 1 (cached)",
			resolver.calls["production"])
	}
	if resolver.calls["shared"] != 0 {
		t.Errorf("shared key resolved %d times, want 0 (out of scope)",
			resolver.calls["shared"])
	}
	if got := storedValue(t, storePath, "shared", "token"); !envelope.IsCiphertext(got) {
		t.Error("Decrypt() modified an out-of-scope group")
	}
}

// TestDecryptSkipsUnavailableKey verifies decryption skips a group whose private
// key is unavailable, reports it, and leaves its ciphertext intact instead of
// failing the whole operation.
func TestDecryptSkipsUnavailableKey(t *testing.T) {
	t.Parallel()

	manager := newGetManager(t, newPrivateKeyTestResolver())
	if err := manager.Set("production", "api_key", func() (string, error) {
		return "plain-api", nil
	}); err != nil {
		t.Fatalf("Set(): %v", err)
	}

	path := manager.params.SecretsPath
	before := storedValue(t, path, "production", "api_key")
	if !strings.HasPrefix(before, "encrypted-") {
		t.Fatalf("precondition: stored value %q is not ciphertext", before)
	}

	result, err := manager.Decrypt("", "")
	if err != nil {
		t.Fatalf("Decrypt() failed on an unavailable key: %v", err)
	}
	if len(result.Secrets) != 0 {
		t.Errorf("changed identities = %v, want none", result.Secrets)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != "production" {
		t.Errorf("unavailable groups = %v, want [production]", result.Unavailable)
	}
	if got := storedValue(t, path, "production", "api_key"); got != before {
		t.Error("Decrypt() modified a group it could not decrypt")
	}
}

// TestDecryptPartiallyDecryptsAvailableGroups verifies an available key decrypts
// its own group while a group with no key is skipped and reported.
func TestDecryptPartiallyDecryptsAvailableGroups(t *testing.T) {
	t.Parallel()

	selected := newTestCipher(t)
	prod, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() production: %v", err)
	}
	shared, err := selected.Keypair()
	if err != nil {
		t.Fatalf("Keypair() shared: %v", err)
	}
	// Only production's private key is available; shared's is absent.
	resolver := &recordingResolver{
		keys:  map[string]string{"production": prod.PrivateKey},
		calls: map[string]int{},
	}
	storePath := writeStore(t, fmt.Sprintf(
		"public_keys:\n  production: %s\n  shared: %s\n",
		prod.PublicKey, shared.PublicKey,
	))
	manager, err := New(Params{
		SecretsPath:           storePath,
		KeysPath:              filepath.Join(filepath.Dir(storePath), "envx.keys"),
		DefaultIndent:         2,
		Cipher:                selected,
		PrivateKeyResolver:    resolver,
		PrivateKeyDestination: newPrivateKeyTestDestination(),
	})
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	set := func(group, key, value string) {
		if err := manager.Set(group, key, func() (string, error) {
			return value, nil
		}); err != nil {
			t.Fatalf("Set(%q/%q): %v", group, key, err)
		}
	}
	set("production", "api_key", "plain-api")
	set("shared", "token", "shared-token")

	result, err := manager.Decrypt("", "")
	if err != nil {
		t.Fatalf("Decrypt(): %v", err)
	}
	if len(result.Secrets) != 1 ||
		result.Secrets[0] != (SecretReference{Group: "production", Key: "api_key"}) {
		t.Fatalf("changed = %v, want only production/api_key", result.Secrets)
	}
	if len(result.Unavailable) != 1 || result.Unavailable[0] != "shared" {
		t.Errorf("unavailable groups = %v, want [shared]", result.Unavailable)
	}
	if got := storedValue(t, storePath, "production", "api_key"); got != "plain-api" {
		t.Errorf("production value = %q, want decrypted plaintext", got)
	}
	if got := storedValue(t, storePath, "shared", "token"); !envelope.IsCiphertext(got) {
		t.Error("Decrypt() modified a group whose key was unavailable")
	}
}
