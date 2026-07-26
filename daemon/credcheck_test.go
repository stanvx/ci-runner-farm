package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// resolveCredential must agree with the engine's credential_facts exactly. When
// it did not, check-credential tested a different secret than the listener
// authenticates with, so the webGUI Test button went green while the daemon 401'd.
func TestResolveCredential(t *testing.T) {
	tests := []struct {
		name    string
		cred    string
		files   []string // relative to the cfg dir
		want    string   // credential type, "" when an error is expected
		wantTok string   // relative token path, "" when none
		wantKey string   // relative app key path, "" when none
	}{
		{
			name:    "default falls back to the legacy bare token",
			cred:    "default",
			files:   []string{"token"},
			want:    credPAT,
			wantTok: "token",
		},
		{
			name:    "credentials entry shadows the legacy token",
			cred:    "default",
			files:   []string{"token", "credentials/default.token"},
			want:    credPAT,
			wantTok: "credentials/default.token",
		},
		{
			name:    "an app key for default wins over the legacy token",
			cred:    "default",
			files:   []string{"token", "credentials/default.app.pem", "credentials/default.app"},
			want:    credApp,
			wantKey: "credentials/default.app.pem",
		},
		{
			name:  "both a PAT and an app key is a hard error, not a guess",
			cred:  "default",
			files: []string{"credentials/default.token", "credentials/default.app.pem", "credentials/default.app"},
		},
		{
			name:    "a named credential never touches the legacy token",
			cred:    "ci",
			files:   []string{"token", "credentials/ci.token"},
			want:    credPAT,
			wantTok: "credentials/ci.token",
		},
		{
			name:  "a named credential with only the legacy token does not exist",
			cred:  "ci",
			files: []string{"token"},
		},
		{
			// The engine emits the metadata path without stat'ing it, so this
			// resolves to app here too and fails later in parseAppMeta with the
			// path in the message. Diverging would mean the Test button and the
			// listener disagree about which credential even exists.
			name:    "an app key resolves even when its metadata sidecar is missing",
			cred:    "ci",
			files:   []string{"credentials/ci.app.pem"},
			want:    credApp,
			wantKey: "credentials/ci.app.pem",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "credentials"), 0o700); err != nil {
				t.Fatal(err)
			}
			for _, f := range tc.files {
				if err := writeFile(filepath.Join(dir, f), "x"); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("CRF_CFGDIR", dir)

			got, err := resolveCredential(tc.cred)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("want an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCredential: %v", err)
			}
			if got.Type != tc.want {
				t.Fatalf("type = %q, want %q", got.Type, tc.want)
			}
			if want := abs(dir, tc.wantTok); got.TokenPath != want {
				t.Fatalf("token path = %q, want %q", got.TokenPath, want)
			}
			if want := abs(dir, tc.wantKey); got.AppKeyPath != want {
				t.Fatalf("app key path = %q, want %q", got.AppKeyPath, want)
			}
		})
	}
}

func abs(dir, rel string) string {
	if rel == "" {
		return ""
	}
	return filepath.Join(dir, rel)
}

// A missing credential must be errNoCredential rather than a bare failure: the
// caller reports the credential NAME, which is what an operator searches for.
func TestResolveCredentialMissing(t *testing.T) {
	t.Setenv("CRF_CFGDIR", t.TempDir())
	if _, err := resolveCredential("nope"); !errors.Is(err, errNoCredential) {
		t.Fatalf("want errNoCredential, got %v", err)
	}
}
