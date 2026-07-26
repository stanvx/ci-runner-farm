package main

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/actions/scaleset"
)

// systemName identifies this client to the Actions service. It shows up in
// GitHub-side telemetry, so it names the plugin rather than "listener".
const systemName = "unraid-ci-runner-farm"

// newClient builds the scaleset client from the credential PATHS the engine
// handed us. The secret bytes are read here and nowhere else, and they never
// reach a log line, a cfg file, or a container environment.
func newClient(cfg *Config, log *slog.Logger) (*scaleset.Client, error) {
	kind, err := credentialType(cfg.Credential)
	if err != nil {
		return nil, err
	}

	info := scaleset.SystemInfo{
		System:    systemName,
		Version:   buildVersion(),
		Subsystem: "scalesetd",
	}
	opts := []scaleset.HTTPOption{scaleset.WithLogger(log)}

	switch kind {
	case credPAT:
		token, err := readSecretFile(cfg.Credential.TokenPath)
		if err != nil {
			return nil, fmt.Errorf("reading PAT for credential %q: %w", cfg.Credential.Name, err)
		}
		return scaleset.NewClientWithPersonalAccessToken(scaleset.NewClientWithPersonalAccessTokenConfig{
			GitHubConfigURL:     cfg.GitHubConfigURL,
			PersonalAccessToken: token,
			SystemInfo:          info,
		}, opts...)

	case credApp:
		auth, err := appAuth(cfg.Credential)
		if err != nil {
			return nil, err
		}
		return scaleset.NewClientWithGitHubApp(scaleset.ClientWithGitHubAppConfig{
			GitHubConfigURL: cfg.GitHubConfigURL,
			GitHubAppAuth:   *auth,
			SystemInfo:      info,
		}, opts...)
	}
	return nil, fmt.Errorf("unreachable credential kind %q", kind)
}

func appAuth(c Credential) (*scaleset.GitHubAppAuth, error) {
	clientID, installationID, err := parseAppMeta(c.AppMetaPath)
	if err != nil {
		return nil, fmt.Errorf("credential %q: %w", c.Name, err)
	}
	id, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("credential %q: INSTALLATION_ID %q is not a number", c.Name, installationID)
	}
	pem, err := readSecretFile(c.AppKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading app key for credential %q: %w", c.Name, err)
	}
	auth := &scaleset.GitHubAppAuth{ClientID: clientID, InstallationID: id, PrivateKey: pem}
	if err := auth.Validate(); err != nil {
		return nil, fmt.Errorf("credential %q: %w", c.Name, err)
	}
	return auth, nil
}

// ensureScaleSet finds the scale set by name, creating it only if absent
// (ADR-0005). It is NEVER deleted: the name is chosen by the operator and is
// referenced from `runs-on:` in repositories this plugin does not control, so
// removing it would break workflows nobody here can see.
func ensureScaleSet(ctx context.Context, c *scaleset.Client, cfg *Config, log *slog.Logger) (*scaleset.RunnerScaleSet, error) {
	groupID, err := resolveRunnerGroup(ctx, c, cfg.RunnerGroup)
	if err != nil {
		return nil, err
	}

	existing, err := c.GetRunnerScaleSet(ctx, groupID, cfg.ScaleSetName)
	if err != nil {
		return nil, fmt.Errorf("looking up scale set %q: %w", cfg.ScaleSetName, err)
	}
	if existing != nil {
		log.Info("found scale set", "name", existing.Name, "id", existing.ID, "group", existing.RunnerGroupName)
		return existing, nil
	}

	created, err := c.CreateRunnerScaleSet(ctx, &scaleset.RunnerScaleSet{
		Name:          cfg.ScaleSetName,
		RunnerGroupID: groupID,
		// Labels is left nil deliberately: the client fills in the scale set name
		// as the single label, which is the value a workflow puts in `runs-on:`.
		// RUNNER_LABELS is a legacy-mode concept and adding it here would suggest
		// a runner can be selected by label, which scale sets do not do.
		//
		// DisableUpdate: the runner image is rebuilt by the plugin, and a runner
		// that self-updates mid-job diverges from the image the operator pinned.
		RunnerSetting: scaleset.RunnerSetting{DisableUpdate: true},
	})
	if err != nil {
		return nil, fmt.Errorf("creating scale set %q: %w", cfg.ScaleSetName, err)
	}
	log.Info("created scale set", "name", created.Name, "id", created.ID, "group", created.RunnerGroupName)
	return created, nil
}

func resolveRunnerGroup(ctx context.Context, c *scaleset.Client, name string) (int, error) {
	if name == "" {
		name = scaleset.DefaultRunnerGroup
	}
	group, err := c.GetRunnerGroupByName(ctx, name)
	if err != nil {
		return 0, fmt.Errorf("resolving runner group %q: %w", name, err)
	}
	if group == nil {
		return 0, fmt.Errorf("runner group %q does not exist", name)
	}
	return group.ID, nil
}
