package main

import (
	"encoding/json"
	"errors"
	"testing"
)

// The engine is the authority on defaults, so this only checks that the shape it
// documents in contract section 2 round-trips into the struct the daemon uses.
func TestConfigParse(t *testing.T) {
	const doc = `{
	  "fleet": "default",
	  "fleet_mode": "scale-set",
	  "scale_set_name": "unraid",
	  "runner_group": "",
	  "runner_labels": "self-hosted,linux,x64",
	  "github_config_url": "https://github.com/myorg",
	  "scope": "org",
	  "max_runners": 16,
	  "min_runners": 0,
	  "drain_timeout": 600,
	  "credential": {
	    "name": "default",
	    "type": "pat",
	    "token_path": "/boot/config/plugins/ci-runner-farm/token",
	    "app_meta_path": "",
	    "app_key_path": ""
	  }
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(doc), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.ScaleSetName != "unraid" || cfg.MaxRunners != 16 || cfg.DrainTimeout != 600 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Credential.TokenPath == "" || cfg.Credential.AppKeyPath != "" {
		t.Fatalf("unexpected credential: %+v", cfg.Credential)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	pat := Credential{Name: "default", TokenPath: "/boot/token"}
	base := func() Config {
		return Config{
			Fleet:           "default",
			FleetMode:       "scale-set",
			ScaleSetName:    "unraid",
			GitHubConfigURL: "https://github.com/myorg",
			MaxRunners:      4,
			Credential:      pat,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"legacy fleet mode", func(c *Config) { c.FleetMode = "legacy" }, true},
		{"empty scale set name", func(c *Config) { c.ScaleSetName = "" }, true},
		{"no capacity", func(c *Config) { c.MaxRunners = 0 }, true},
		{"missing credential", func(c *Config) { c.Credential = Credential{Name: "gone"} }, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			err := cfg.validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("validate() = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// github_config_url is DERIVED by the engine from GH_SCOPE/GH_OWNER/GH_REPOS so
// that Go is never a fourth place a default lives. Go only rejects the shapes
// the scaleset client would reject later with a less useful message.
func TestValidateConfigURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://github.com/myorg", false},
		{"https://github.com/myorg/myrepo", false},
		{"https://ghes.example.com/myorg", false},
		{"", true},
		{"http://github.com/myorg", true}, // plain HTTP would send the PAT in clear
		{"https://github.com", true},      // no owner: scope was never resolved
		{"https://github.com/", true},     // trailing slash only, same failure
		{"myorg", true},                   // bare owner, a common hand-edit
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			err := validateConfigURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateConfigURL(%q) = %v, wantErr %v", tc.url, err, tc.wantErr)
			}
		})
	}
}

// The type is derived from which files exist, never declared, because a declared
// type is a field that can lie.
func TestCredentialType(t *testing.T) {
	tests := []struct {
		name string
		cred Credential
		want string
		err  bool
	}{
		{"pat", Credential{Name: "default", TokenPath: "/boot/token"}, credPAT, false},
		{"app", Credential{Name: "ci", AppKeyPath: "/boot/ci.app.pem", AppMetaPath: "/boot/ci.app"}, credApp, false},
		{"app without metadata", Credential{Name: "ci", AppKeyPath: "/boot/ci.app.pem"}, "", true},
		{"both", Credential{Name: "ci", TokenPath: "/boot/ci.token", AppKeyPath: "/boot/ci.app.pem"}, "", true},
		{"neither", Credential{Name: "ci"}, "", true},
		{"declared type is ignored", Credential{Name: "ci", Type: "app", TokenPath: "/boot/ci.token"}, credPAT, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := credentialType(tc.cred)
			if (err != nil) != tc.err {
				t.Fatalf("credentialType() error = %v, wantErr %v", err, tc.err)
			}
			if got != tc.want {
				t.Fatalf("credentialType() = %q, want %q", got, tc.want)
			}
		})
	}

	if _, err := credentialType(Credential{Name: "ci"}); !errors.Is(err, errNoCredential) {
		t.Fatalf("a missing credential must be distinguishable, got %v", err)
	}
}

func TestParseAppMeta(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ci.app"
	const body = "# app metadata\nCLIENT_ID=Iv1.abc123\nINSTALLATION_ID=\"12345\"\nUNKNOWN=ignored\n"
	if err := writeFile(path, body); err != nil {
		t.Fatal(err)
	}

	clientID, installationID, err := parseAppMeta(path)
	if err != nil {
		t.Fatalf("parseAppMeta: %v", err)
	}
	if clientID != "Iv1.abc123" || installationID != "12345" {
		t.Fatalf("got %q / %q", clientID, installationID)
	}

	incomplete := dir + "/broken.app"
	if err := writeFile(incomplete, "CLIENT_ID=Iv1.abc123\n"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseAppMeta(incomplete); err == nil {
		t.Fatal("a metadata file missing INSTALLATION_ID must fail loudly")
	}
}
