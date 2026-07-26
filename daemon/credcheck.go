package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// defaultCfgDir is the flash config directory. CRF_CFGDIR overrides it for
// tests; nothing else should ever point it somewhere else.
const defaultCfgDir = "/boot/config/plugins/ci-runner-farm"

// defaultAPIURL is overridable because a GHES install answers on its own host.
const defaultAPIURL = "https://api.github.com"

// credentialReport is what the webGUI Test button renders. It carries IDENTITY
// FACTS ONLY: never key bytes, never a token, never a JWT.
type credentialReport struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	AppSlug          string   `json:"app_slug,omitempty"`
	Account          string   `json:"account,omitempty"`
	AccountType      string   `json:"account_type,omitempty"`
	RepositorySelect string   `json:"repository_selection,omitempty"`
	Permissions      []string `json:"permissions,omitempty"`
	RunnerGroups     []string `json:"runner_groups,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
}

// resolveCredential derives the credential from which files exist on flash, the
// same rule the engine applies. The legacy bare `token` file IS credential
// "default", so an upgrade writes nothing and an existing install keeps working.
func resolveCredential(name string) (Credential, error) {
	dir := os.Getenv("CRF_CFGDIR")
	if dir == "" {
		dir = defaultCfgDir
	}
	c := Credential{Name: name}

	credDir := filepath.Join(dir, "credentials")
	tokenPath := filepath.Join(credDir, name+".token")
	keyPath := filepath.Join(credDir, name+".app.pem")
	// Credential `default` falls back to the legacy bare token file only when
	// NOTHING in credentials/ shadows it, byte for byte the engine's
	// credential_facts rule. Resolving `default` straight to <dir>/token made the
	// webGUI Test button report on a different secret than the listener
	// authenticates with: green button, 401 in the log.
	if name == "default" && !exists(tokenPath) && !exists(keyPath) {
		tokenPath = filepath.Join(dir, "token")
	}
	if exists(tokenPath) {
		c.TokenPath = tokenPath
	}
	if exists(keyPath) {
		c.AppKeyPath = keyPath
		c.AppMetaPath = filepath.Join(credDir, name+".app")
	}

	kind, err := credentialType(c)
	if err != nil {
		return c, err
	}
	c.Type = kind
	return c, nil
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// checkCredential is the diagnostic behind the webGUI Test button. It is never
// called from the daemon's own loop, so a slow or failing GitHub here degrades
// nothing but the button.
func checkCredential(ctx context.Context, name, apiURL string) error {
	cred, err := resolveCredential(name)
	if err != nil {
		return err
	}
	report := credentialReport{Name: cred.Name, Type: cred.Type}

	switch cred.Type {
	case credPAT:
		err = checkPAT(ctx, cred, apiURL, &report)
	case credApp:
		err = checkApp(ctx, cred, apiURL, &report)
	}
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func checkPAT(ctx context.Context, cred Credential, apiURL string, report *credentialReport) error {
	token, err := readSecretFile(cred.TokenPath)
	if err != nil {
		return err
	}
	var user struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	// The granted scopes only ever come back as a response header, which is why
	// this reads the header rather than any field of the body.
	hdr, err := githubJSON(ctx, apiURL+"/user", "token "+token, &user)
	if err != nil {
		return err
	}
	report.Account = user.Login
	report.AccountType = user.Type
	report.Scopes = splitList(hdr.Get("X-OAuth-Scopes"))
	return nil
}

func checkApp(ctx context.Context, cred Credential, apiURL string, report *credentialReport) error {
	auth, err := appAuth(cred)
	if err != nil {
		return err
	}
	assertion, err := appJWT(auth.ClientID, auth.PrivateKey)
	if err != nil {
		return err
	}

	var inst struct {
		AppSlug             string            `json:"app_slug"`
		RepositorySelection string            `json:"repository_selection"`
		Permissions         map[string]string `json:"permissions"`
		Account             struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	}
	url := fmt.Sprintf("%s/app/installations/%d", apiURL, auth.InstallationID)
	if _, err := githubJSON(ctx, url, "Bearer "+assertion, &inst); err != nil {
		return err
	}

	report.AppSlug = inst.AppSlug
	report.Account = inst.Account.Login
	report.AccountType = inst.Account.Type
	report.RepositorySelect = inst.RepositorySelection
	for scope, level := range inst.Permissions {
		report.Permissions = append(report.Permissions, scope+":"+level)
	}
	// Sorted so a diff between two Test button runs shows a real permission
	// change and not Go's randomised map iteration order.
	slices.Sort(report.Permissions)

	// The runner groups are the fact operators actually need: a scale set is
	// created inside a group, and a credential that cannot see the group fails at
	// create time with an opaque 404.
	if inst.Account.Type == "Organization" {
		groups, err := installationRunnerGroups(ctx, apiURL, assertion, auth.InstallationID, inst.Account.Login)
		if err != nil {
			return err
		}
		report.RunnerGroups = groups
	}
	return nil
}

func installationRunnerGroups(ctx context.Context, apiURL, assertion string, installationID int64, org string) ([]string, error) {
	var tok struct {
		Token string `json:"token"`
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", apiURL, installationID)
	if err := githubPostJSON(ctx, url, "Bearer "+assertion, &tok); err != nil {
		return nil, err
	}

	var groups struct {
		RunnerGroups []struct {
			Name string `json:"name"`
		} `json:"runner_groups"`
	}
	url = fmt.Sprintf("%s/orgs/%s/actions/runner-groups", apiURL, org)
	if _, err := githubJSON(ctx, url, "token "+tok.Token, &groups); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(groups.RunnerGroups))
	for _, g := range groups.RunnerGroups {
		out = append(out, g.Name)
	}
	return out, nil
}

// appJWT mints the short-lived assertion that authenticates as the app itself.
// Ten minutes is GitHub's hard ceiling and the 60 second backdate absorbs clock
// skew on a NAS that may not have finished its first NTP sync after a reboot.
func appJWT(clientID, pemKey string) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(pemKey))
	if err != nil {
		return "", fmt.Errorf("parsing app private key: %w", err)
	}
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
		Issuer:    clientID,
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
}

func githubJSON(ctx context.Context, url, authorization string, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return doGitHub(req, authorization, out)
}

func githubPostJSON(ctx context.Context, url, authorization string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	_, err = doGitHub(req, authorization, out)
	return err
}

func doGitHub(req *http.Request, authorization string, out any) (http.Header, error) {
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// The body can echo the request, so only the status is surfaced: this
		// output is rendered in the webGUI and read by whoever is on the LAN.
		return nil, fmt.Errorf("%s %s: %s", req.Method, req.URL.Path, resp.Status)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", req.URL.Path, err)
		}
	}
	return resp.Header, nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
