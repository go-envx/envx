package secrets

import (
	"bytes"
	"strings"
	"testing"
)

// FuzzEnvelopeRoundTrip verifies every encodable algorithm and payload decodes
// back unchanged.
func FuzzEnvelopeRoundTrip(f *testing.F) {
	f.Add("age", []byte{1, 2, 3})
	f.Add("naclbox", []byte("payload"))
	f.Fuzz(func(t *testing.T, algorithm string, payload []byte) {
		encoded, err := envelope{algorithm: algorithm, payload: payload}.encode()
		if err != nil {
			return
		}
		if !isEnvelope(encoded) || strings.ContainsAny(encoded, " \n") {
			t.Fatalf("encode(%q) = %q, not a single-token envelope", algorithm, encoded)
		}
		env, err := decodeEnvelope(encoded)
		if err != nil || env.algorithm != algorithm || !bytes.Equal(env.payload, payload) {
			t.Errorf("decode(%q) = %+v, %v", encoded, env, err)
		}
	})
}

// TestOpenEnvelope verifies stored values are rejected unless they are
// envelopes for the configured algorithm.
func TestOpenEnvelope(t *testing.T) {
	t.Parallel()
	ref := Reference{Group: "api", Key: "TOKEN"}

	tests := []struct {
		stored  string
		wantErr string
	}{
		{stored: "encrypted-age:AAEC"},
		{stored: "plaintext", wantErr: "not encrypted"},
		{stored: "encrypted-naclbox:AAEC", wantErr: `uses algorithm "naclbox"`},
	}
	for _, tt := range tests {
		_, err := openEnvelope(ref, tt.stored, "age")
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("openEnvelope(%q): %v", tt.stored, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("openEnvelope(%q) error = %v, want %q", tt.stored, err, tt.wantErr)
		}
	}
}

// TestValidateEnvelope verifies legacy plaintext is accepted and malformed
// envelopes are rejected.
func TestValidateEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stored  string
		wantErr bool
	}{
		{stored: "legacy plaintext"},
		{stored: "encrypted-age:AAEC"},
		{stored: "encrypted-age:", wantErr: true},
		{stored: "encrypted-age", wantErr: true},
		{stored: "encrypted-:AA", wantErr: true},
		{stored: "encrypted-age:AA==", wantErr: true},
		{stored: "encrypted-age:a:b", wantErr: true},
	}
	for _, tt := range tests {
		if err := ValidateEnvelope(tt.stored); (err != nil) != tt.wantErr {
			t.Errorf("ValidateEnvelope(%q) = %v, wantErr %v", tt.stored, err, tt.wantErr)
		}
	}
}
