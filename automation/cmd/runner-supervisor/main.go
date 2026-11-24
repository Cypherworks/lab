// Command runner-supervisor keeps one Docker runner VM taking one job per
// boot: it restores the VM to its golden snapshot, registers it with a
// single-job JIT config, and resets it when the job ends. Run one per VM.
// SIGTERM drains: an idle VM is deregistered and stopped, a busy one is
// stopped once its job ends. Once a week, between jobs, it patches the
// golden snapshot. See lab-deploy docs/docker-runner-reset.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Cypherworks/lab/automation/internal/incus"
	"github.com/Cypherworks/lab/automation/internal/runners"
	"github.com/Cypherworks/lab/automation/internal/supervisor"
)

// exitFailed tells systemd not to restart a VM whose reset kept failing.
const exitFailed = 3

// timing is the loop's timing, swapped in tests.
var timing = supervisor.DefaultTiming

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stderr))
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("runner-supervisor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	vm := fs.String("vm", "", "the Incus VM to supervise (required)")
	org := fs.String("org", "", "the GitHub org the runner joins (required)")
	remote := fs.String("remote", "", "the incus remote holding the VM (required)")
	project := fs.String("project", "", "the Incus project holding the VM (required)")
	labels := fs.String("labels", "self-hosted,docker", "comma-separated runner labels")
	tokenFile := fs.String("token-file", "", "file holding the GitHub token that mints runners (required)")
	incusBin := fs.String("incus", "/usr/bin/incus", "path to the incus CLI")
	metricsDir := fs.String("metrics-dir", "", "node_exporter textfile directory; empty writes no metrics")
	githubURL := fs.String("github-url", "https://api.github.com", "GitHub REST API base URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	required := []struct{ name, value string }{
		{"vm", *vm}, {"org", *org}, {"remote", *remote}, {"project", *project},
		{"token-file", *tokenFile},
	}
	for _, f := range required {
		if f.value == "" {
			fmt.Fprintf(stderr, "runner-supervisor: -%s is required\n", f.name)
			return 2
		}
	}
	raw, err := os.ReadFile(*tokenFile)
	if err != nil {
		fmt.Fprintf(stderr, "runner-supervisor: reading the token: %v\n", err)
		return 2
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		fmt.Fprintf(stderr, "runner-supervisor: the token file %s is empty\n", *tokenFile)
		return 2
	}

	log := slog.New(slog.NewTextHandler(stderr, nil))
	gh := &runners.Client{
		BaseURL: *githubURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Token:   token,
		Org:     *org,
	}
	v := &supervisor.VM{
		Name:       *vm,
		Labels:     strings.Split(*labels, ","),
		GitHub:     gh,
		Incus:      &incus.Client{Remote: *remote, Project: *project, Run: incus.CLI(*incusBin)},
		Clock:      supervisor.RealClock{},
		Timing:     timing(),
		MetricsDir: *metricsDir,
		Log:        log,
	}
	if err := v.Cleanup(ctx); err != nil {
		log.Warn("cleanup of earlier runners failed", "vm", *vm, "err", err)
	}
	v.GroupID, err = defaultGroup(ctx, gh, v.Clock, v.Timing, log)
	if err != nil {
		return 0
	}
	for {
		if err := v.Cycle(ctx); err != nil {
			return exitCode(err, log.With("vm", *vm))
		}
	}
}

// exitCode maps the error that ended the loop to the process's exit code.
func exitCode(err error, log *slog.Logger) int {
	if errors.Is(err, supervisor.ErrFailed) {
		log.Error("VM left stopped", "err", err)
		return exitFailed
	}
	log.Info("drained", "err", err)
	return 0
}

// defaultGroup looks up the Default runner group, retrying until ctx ends.
func defaultGroup(ctx context.Context, gh *runners.Client, clock supervisor.Clock, t supervisor.Timing, log *slog.Logger) (int64, error) {
	backoff := t.MintBackoffStart
	for {
		id, err := gh.DefaultGroupID(ctx)
		if err == nil {
			return id, nil
		}
		log.Warn("runner group lookup failed", "err", err)
		if err := clock.Sleep(ctx, backoff); err != nil {
			return 0, err
		}
		backoff = min(2*backoff, t.MintBackoffMax)
	}
}
