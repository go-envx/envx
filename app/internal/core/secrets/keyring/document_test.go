package keyring

import (
	"strings"
	"testing"
)

// TestParseDocumentErrors verifies malformed entries are rejected without
// echoing the entry name, which may be misplaced key material.
func TestParseDocumentErrors(t *testing.T) {
	t.Parallel()
	const material = "AGE-SECRET-KEY-EXAMPLE"

	tests := map[string]string{
		"missing equals":  material + "\n",
		"empty name":      "=" + material + "\n",
		"empty version":   "API/=" + material + "\n",
		"empty value":     material + "=\n",
		"duplicate entry": material + "=a\n" + strings.ToLower(material) + "=b\n",
	}
	for name, content := range tests {
		_, err := parseDocument(content, "envx.keys")
		if err == nil {
			t.Errorf("%s: parseDocument succeeded, want error", name)
			continue
		}
		if strings.Contains(strings.ToUpper(err.Error()), material) {
			t.Errorf("%s: error %q discloses the entry", name, err)
		}
	}
}

// TestLookup verifies versioned keys are selected by public key, legacy keys
// are the only fallback, and selection without a public key is unambiguous.
func TestLookup(t *testing.T) {
	t.Parallel()
	versioned := "API" + versionSeparator + fingerprint("pub-new") + "=new\n"

	tests := []struct {
		name      string
		content   string
		publicKey string
		want      string
		found     bool
		wantErr   bool
	}{
		{"legacy", "API=old\n", "pub-old", "old", true, false},
		{"versioned match", "API=old\n" + versioned, "pub-new", "new", true, false},
		{"legacy fallback", "API=old\n" + versioned, "pub-old", "old", true, false},
		{"mismatched version", versioned, "pub-old", "", false, false},
		{"other group", "WEB=web\n", "pub", "", false, false},
		{"sole key without identity", versioned, "", "new", true, false},
		{"ambiguous without identity", "API=old\n" + versioned, "", "", false, true},
	}
	for _, tt := range tests {
		doc, err := parseDocument(tt.content, "envx.keys")
		if err != nil {
			t.Fatalf("%s: parseDocument: %v", tt.name, err)
		}
		got, found, err := doc.lookup("API", tt.publicKey)
		if (err != nil) != tt.wantErr || found != tt.found || got != tt.want {
			t.Errorf("%s: lookup = %q, %v, %v", tt.name, got, found, err)
		}
	}
}

// TestRetainRendering verifies retaining preserves other lines, replaces only
// the same public key's entry, and ends with one newline.
func TestRetainRendering(t *testing.T) {
	t.Parallel()
	entry := "API" + versionSeparator + fingerprint("pub") + "="

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"empty file", "", entry + "key\n"},
		{
			"appends after trailing blanks",
			"# keys\nAPI=old\n\n\n",
			"# keys\nAPI=old\n" + entry + "key\n",
		},
		{
			"replaces same version in place",
			entry + "stale\nWEB=web\n",
			entry + "key\nWEB=web\n",
		},
	}
	for _, tt := range tests {
		doc, err := parseDocument(tt.content, "envx.keys")
		if err != nil {
			t.Fatalf("%s: parseDocument: %v", tt.name, err)
		}
		if got := doc.retain("API", "pub", "key"); got != tt.want {
			t.Errorf("%s: retain =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

// FuzzRetainLookup verifies any retained key is found again by its public key.
func FuzzRetainLookup(f *testing.F) {
	f.Add("API=old\n", "pub", "key")
	f.Add("# comment\n", "age1xyz", "AGE-SECRET-KEY-1")
	f.Fuzz(func(t *testing.T, content, publicKey, privateKey string) {
		if publicKey == "" || strings.TrimSpace(privateKey) == "" ||
			strings.ContainsAny(privateKey, "\r\n") {
			return
		}
		doc, err := parseDocument(content, "envx.keys")
		if err != nil {
			return
		}
		updated, err := parseDocument(doc.retain("API", publicKey, privateKey), "envx.keys")
		if err != nil {
			t.Fatalf("retained document does not parse: %v", err)
		}
		got, found, err := updated.lookup("API", publicKey)
		if err != nil || !found || got != privateKey {
			t.Errorf("lookup = %q, %v, %v; want %q", got, found, err, privateKey)
		}
	})
}
