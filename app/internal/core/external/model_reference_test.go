package external_test

import (
	"errors"
	"testing"

	"github.com/go-envx/envx/app/internal/core/external"
)

func TestParseReference(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw     string
		want    external.Reference
		wantErr error
	}{
		"op": {
			raw:  "op://vault/item name/field",
			want: external.Reference{Scheme: "op", Path: "vault/item name/field"},
		},
		"bws id": {
			raw:  "bws://0b5c6f9e",
			want: external.Reference{Scheme: "bws", Path: "0b5c6f9e"},
		},
		"field": {
			raw: "passwordstate://1990/a b#UserName",
			want: external.Reference{
				Scheme: "passwordstate", Path: "1990/a b", Field: "UserName",
			},
		},
		"encoded hash": {
			raw:  "bws://app/a%23b",
			want: external.Reference{Scheme: "bws", Path: "app/a%23b"},
		},
		"unknown scheme": {raw: "vault://a/b", wantErr: external.ErrNotReference},
		"plain":          {raw: "op", wantErr: external.ErrNotReference},
		"empty path":     {raw: "op://", wantErr: external.ErrInvalidReference},
		"empty segment":  {raw: "op://vault//field", wantErr: external.ErrInvalidReference},
		"trailing slash": {raw: "passwordstate://12/", wantErr: external.ErrInvalidReference},
		"control char":   {raw: "op://vault/a\nb", wantErr: external.ErrInvalidReference},
		"field only":     {raw: "bws://#f", wantErr: external.ErrInvalidReference},
		"empty field":    {raw: "bws://12#", wantErr: external.ErrInvalidReference},
		"second hash":    {raw: "bws://12#a#b", wantErr: external.ErrInvalidReference},
		"slash before #": {raw: "bws://12/#f", wantErr: external.ErrInvalidReference},
		"control field":  {raw: "bws://12#a\tb", wantErr: external.ErrInvalidReference},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := external.ParseReference(tt.raw)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if tt.wantErr == nil && got.String() != tt.raw {
				t.Errorf("String() = %q, want %q", got.String(), tt.raw)
			}
		})
	}
}
