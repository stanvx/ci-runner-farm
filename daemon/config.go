package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config mirrors the object emitted by `runner-farm.sh --fleet <name> config-json`.
// Every default in it is resolved by the engine on purpose: the plugin already
// keeps defaults in three hand-written places that config-parity.sh diffs in CI,
// and a fourth copy living in Go would drift silently because that test cannot
// see it.
type Config struct {
	Fleet           string     `json:"fleet"`
	FleetMode       string     `json:"fleet_mode"`
	ScaleSetName    string     `json:"scale_set_name"`
	RunnerGroup     string     `json:"runner_group"`
	RunnerLabels    string     `json:"runner_labels"`
	GitHubConfigURL string     `json:"github_config_url"`
	Scope           string     `json:"scope"`
	MaxRunners      int        `json:"max_runners"`
	MinRunners      int        `json:"min_runners"`
	DrainTimeout    int        `json:"drain_timeout"`
	Credential      Credential `json:"credential"`
}

// Credential carries PATHS only. The engine never puts secret material in the
// config JSON, because that JSON is logged, tailed by the UI, and would end up
// in a support bundle. The daemon opens the files itself.
type Credential struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	TokenPath   string `json:"token_path"`
	AppMetaPath string `json:"app_meta_path"`
	AppKeyPath  string `json:"app_key_path"`
}

const (
	credPAT = "pat"
	credApp = "app"
)

var errNoCredential = errors.New("credential does not exist")

// loadConfig shells out to the engine rather than reading the cfg files, so the
// two layers (legacy cfg + fleets/<name>.cfg) are merged by the one piece of
// code that knows how they overlay.
func loadConfig(ctx context.Context, eng *Engine) (*Config, error) {
	out, err := eng.output(ctx, "config-json")
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(out, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config-json: %w", err)
	}
	return &cfg, nil
}

// validate refuses to start rather than half-starting. A daemon that attaches to
// the wrong fleet mode would provision JIT runners into a fleet whose containers
// are managed by the legacy registration-token path, and the two removers would
// fight over the same container names.
func (c *Config) validate() error {
	if c.FleetMode != "scale-set" {
		return fmt.Errorf("fleet %q is mode %q, not scale-set", c.Fleet, c.FleetMode)
	}
	if c.ScaleSetName == "" {
		return fmt.Errorf("fleet %q has an empty SCALESET_NAME", c.Fleet)
	}
	if err := validateConfigURL(c.GitHubConfigURL); err != nil {
		return err
	}
	if c.MaxRunners < 1 {
		return fmt.Errorf("max_runners is %d, refusing to advertise no capacity at all", c.MaxRunners)
	}
	if _, err := credentialType(c.Credential); err != nil {
		return err
	}
	return nil
}

// validateConfigURL only sanity-checks: the engine derives this per-fleet from
// GH_SCOPE/GH_OWNER/GH_REPOS and the scaleset client parses it internally with
// an unexported parser, so all Go can usefully do is reject the empty and the
// obviously wrong before the client returns ErrInvalidGitHubConfigURL.
func validateConfigURL(raw string) error {
	if raw == "" {
		return errors.New("github_config_url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("github_config_url %q: %w", raw, err)
	}
	if u.Scheme != "https" || u.Host == "" || strings.Trim(u.Path, "/") == "" {
		return fmt.Errorf("github_config_url %q is not an https owner or owner/repo URL", raw)
	}
	return nil
}

// credentialType DERIVES the type from which files the engine found, never from
// the declared "type" field. A declared type is a field that can lie: an operator
// who drops a .app.pem next to a stale .token would otherwise get a PAT client
// authenticating as the wrong identity with no error anywhere.
func credentialType(c Credential) (string, error) {
	hasApp := c.AppKeyPath != ""
	hasPAT := c.TokenPath != ""

	switch {
	case hasApp && hasPAT:
		return "", fmt.Errorf("credential %q has both an app key and a token; delete one", c.Name)
	case hasApp:
		if c.AppMetaPath == "" {
			return "", fmt.Errorf("credential %q has an app key but no .app metadata file", c.Name)
		}
		return credApp, nil
	case hasPAT:
		return credPAT, nil
	default:
		return "", fmt.Errorf("credential %q: %w", c.Name, errNoCredential)
	}
}

// readSecretFile trims trailing whitespace because these files are edited by
// hand on flash and a trailing newline in a PAT produces a 401 with no hint.
func readSecretFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	return s, nil
}

// parseAppMeta reads the KEY=VALUE sidecar written next to an app private key.
// Unknown keys are ignored so a future field does not break an older binary.
func parseAppMeta(path string) (clientID, installationID string, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "CLIENT_ID":
			clientID = strings.Trim(strings.TrimSpace(v), `"'`)
		case "INSTALLATION_ID":
			installationID = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	if clientID == "" || installationID == "" {
		return "", "", fmt.Errorf("%s needs both CLIENT_ID and INSTALLATION_ID", path)
	}
	return clientID, installationID, nil
}
