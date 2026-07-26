// Command crf-scalesetd runs one GitHub Actions scale set listener per fleet for
// the Unraid ci-runner-farm plugin. It never talks to the Docker socket and never
// reads a .cfg file: every host-side action goes through runner-farm.sh, which is
// the one place that knows about locks, cache roots and path safety.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "crf-scalesetd: "+err.Error())
		os.Exit(1)
	}
}

func usage() error {
	return errors.New("usage: crf-scalesetd run --fleet <name> | check-credential <name> | version")
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}

	// SIGTERM is the only stop signal that matters: it is what the engine's
	// scaleset_stop sends, and it starts the drain rather than killing jobs.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fleet := fs.String("fleet", "", "fleet name (required)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *fleet == "" {
			return errors.New("run requires --fleet <name>")
		}
		// Logs go to stderr so stdout stays clean for the JSON-emitting
		// subcommands. The engine redirects both into $FARM_LOG, which is what
		// the UI tails, so this needs no separate log file.
		log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})).
			With("fleet", *fleet)
		return runListener(ctx, *fleet, log)

	case "check-credential":
		fs := flag.NewFlagSet("check-credential", flag.ContinueOnError)
		apiURL := fs.String("api-url", defaultAPIURL, "GitHub API base URL")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("check-credential requires exactly one credential name")
		}
		return checkCredential(ctx, fs.Arg(0), *apiURL)

	case "version":
		fmt.Println(buildVersion())
		return nil
	}
	return usage()
}

// buildVersion reports the scaleset module version rather than a plugin version.
// The release binary is built with -buildid= for byte reproducibility, so there
// is no place to stamp a version into, and the dependency version is the fact
// that actually matters when the Actions API moves under a public preview module.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/actions/scaleset" {
			return "scaleset " + dep.Version
		}
	}
	return "unknown"
}
