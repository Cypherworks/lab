package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Cypherworks/lab/automation/internal/supervisor"
)

// fakeGitHub serves the org endpoints startup needs and records deletes.
type fakeGitHub struct {
	mu         sync.Mutex
	deleted    []string
	groupFails int
	onGroups   func()
}

func (g *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer github_pat_example" {
		http.Error(w, "bad credentials", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/orgs/example/actions/runners":
		_ = json.NewEncoder(w).Encode(map[string]any{"runners": []map[string]any{
			{"id": 7, "name": "docker-runner-1-20261122T140000Z", "status": "offline"},
			{"id": 8, "name": "ci-runner-1", "status": "online"},
		}})
	case r.Method == http.MethodGet && r.URL.Path == "/orgs/example/actions/runner-groups":
		if g.groupFails > 0 {
			g.groupFails--
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"runner_groups": []map[string]any{
			{"id": 1, "default": true},
		}})
	case r.Method == http.MethodDelete:
		g.deleted = append(g.deleted, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// fakeIncus writes an incus stand-in whose every command exits with code.
func fakeIncus(t *testing.T, code string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "incus")
	script := "#!/bin/sh\ncase \"$1\" in query) echo '{\"status\":\"Stopped\"}';; esac\nexit " + code + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func tokenFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func args(srv *httptest.Server, incus, token string, extra ...string) []string {
	return append([]string{
		"-vm", "docker-runner-1", "-org", "example", "-remote", "lab",
		"-project", "docker-runners", "-incus", incus, "-token-file", token,
		"-github-url", srv.URL,
	}, extra...)
}

func TestMissingRequiredFlagsAreAUsageError(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"-org", "example"}, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "-vm") {
		t.Fatalf("stderr = %q, want the missing flag named", stderr.String())
	}
}

func TestUnknownFlagsAreAUsageError(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"-nope"}, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestAnUnreadableOrEmptyTokenFileIsAUsageError(t *testing.T) {
	srv := httptest.NewServer(&fakeGitHub{})
	defer srv.Close()
	for name, path := range map[string]string{
		"missing": filepath.Join(t.TempDir(), "absent"),
		"empty":   tokenFile(t, " \n"),
	} {
		var stderr bytes.Buffer
		code := run(context.Background(), args(srv, fakeIncus(t, "0"), path), &stderr)
		if code != 2 {
			t.Fatalf("%s token: exit = %d, want 2", name, code)
		}
		if !strings.Contains(stderr.String(), "token") {
			t.Fatalf("%s token: stderr = %q", name, stderr.String())
		}
	}
}

// instantTiming drops the loop's waits for the test's duration.
func instantTiming(t *testing.T) {
	t.Helper()
	instant := supervisor.DefaultTiming()
	instant.ResetBackoff = []time.Duration{0, 0, 0}
	instant.MintBackoffStart = 0
	timing = func() supervisor.Timing { return instant }
	t.Cleanup(func() { timing = supervisor.DefaultTiming })
}

func TestAVMWhoseResetKeepsFailingExitsWithTheFailedCode(t *testing.T) {
	g := &fakeGitHub{groupFails: 1}
	srv := httptest.NewServer(g)
	defer srv.Close()
	instantTiming(t)
	var stderr bytes.Buffer
	a := args(srv, fakeIncus(t, "1"), tokenFile(t, "github_pat_example\n"))
	if code := run(context.Background(), a, &stderr); code != exitFailed {
		t.Fatalf("exit = %d, want %d; stderr:\n%s", code, exitFailed, stderr.String())
	}
	for _, want := range []string{"runner group lookup failed", "reset kept failing"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if len(g.deleted) != 1 || g.deleted[0] != "/orgs/example/actions/runners/7" {
		t.Fatalf("deleted = %v, want only the VM's leftover runner", g.deleted)
	}
}

func TestTheLoopsEndMapsToItsExitCode(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	failed := fmt.Errorf("docker-runner-1: %w: start failed", supervisor.ErrFailed)
	if code := exitCode(failed, log); code != exitFailed {
		t.Fatalf("failed: exit = %d, want %d", code, exitFailed)
	}
	if code := exitCode(supervisor.ErrDrained, log); code != 0 {
		t.Fatalf("drained: exit = %d, want 0", code)
	}
	for _, want := range []string{"VM left stopped", "drained"} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("logs lack %q:\n%s", want, logs.String())
		}
	}
}

func TestADrainBeforeTheGroupIsKnownExitsCleanly(t *testing.T) {
	srv := httptest.NewServer(&fakeGitHub{groupFails: 1 << 30})
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stderr bytes.Buffer
	a := args(srv, fakeIncus(t, "0"), tokenFile(t, "github_pat_example"))
	if code := run(ctx, a, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, stderr.String())
	}
}
