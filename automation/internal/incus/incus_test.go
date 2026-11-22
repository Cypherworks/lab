package incus

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// call is one recorded run of the incus CLI.
type call struct {
	args  []string
	stdin string
}

// recorder stands in for the incus binary, answering by subcommand.
type recorder struct {
	calls []call
	out   map[string]string
	fail  map[string]error
}

func (r *recorder) run(_ context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	c := call{args: args}
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		c.stdin = string(b)
	}
	r.calls = append(r.calls, c)
	key := args[0]
	if key == "snapshot" {
		key += " " + args[1]
	}
	return []byte(r.out[key]), r.fail[key]
}

func newClient() (*recorder, *Client) {
	r := &recorder{out: map[string]string{}, fail: map[string]error{}}
	return r, &Client{Remote: "lab", Project: "docker-runners", Run: r.run}
}

func TestResetStepsAreScopedToTheRemoteAndProject(t *testing.T) {
	r, c := newClient()
	r.out["query"] = `{"status": "Running"}`
	ctx := context.Background()
	if err := c.Stop(ctx, "docker-runner-1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Restore(ctx, "docker-runner-1", "golden-20261122T150000Z"); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx, "docker-runner-1"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"query", "lab:/1.0/instances/docker-runner-1/state?project=docker-runners"},
		{"stop", "lab:docker-runner-1", "--force", "--project", "docker-runners"},
		{"snapshot", "restore", "lab:docker-runner-1", "golden-20261122T150000Z", "--project", "docker-runners"},
		{"start", "lab:docker-runner-1", "--project", "docker-runners"},
	}
	if len(r.calls) != len(want) {
		t.Fatalf("%d calls, want %d", len(r.calls), len(want))
	}
	for i, w := range want {
		if !reflect.DeepEqual(r.calls[i].args, w) {
			t.Fatalf("call %d = %q, want %q", i, r.calls[i].args, w)
		}
	}
}

func TestStopOfAStoppedVMSendsNoStop(t *testing.T) {
	r, c := newClient()
	r.out["query"] = `{"status": "Stopped"}`
	if err := c.Stop(context.Background(), "docker-runner-1"); err != nil {
		t.Fatalf("stop of a stopped VM: %v", err)
	}
	if len(r.calls) != 1 || r.calls[0].args[0] != "query" {
		t.Fatalf("calls = %v, want only the state query", r.calls)
	}
}

func TestStopFailuresSurface(t *testing.T) {
	r, c := newClient()
	r.out["query"] = `{"status": "Running"}`
	r.fail["stop"] = errors.New("exit status 1")
	if err := c.Stop(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("stop failure swallowed")
	}
	r.fail["query"] = errors.New("exit status 1")
	if err := c.Stop(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("state failure swallowed")
	}
}

func TestStartAndRestoreFailuresSurface(t *testing.T) {
	r, c := newClient()
	r.fail["start"] = errors.New("exit status 1")
	r.fail["snapshot restore"] = errors.New("exit status 1")
	if err := c.Start(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("start failure swallowed")
	}
	if err := c.Restore(context.Background(), "docker-runner-1", "golden-x"); err == nil {
		t.Fatal("restore failure swallowed")
	}
}

func TestNewestGoldenIsTheLatestGoldenSnapshotByName(t *testing.T) {
	r, c := newClient()
	r.out["snapshot list"] = `[
		{"name": "golden-20261115T020000Z", "created_at": "2026-11-15T02:00:00Z"},
		{"name": "before-cis", "created_at": "2026-11-23T00:00:00Z"},
		{"name": "golden-20261122T020000Z", "created_at": "2026-11-22T02:00:00Z"}
	]`
	got, err := c.NewestGolden(context.Background(), "docker-runner-2")
	if err != nil {
		t.Fatal(err)
	}
	if got != "golden-20261122T020000Z" {
		t.Fatalf("newest golden = %q", got)
	}
	want := []string{"snapshot", "list", "lab:docker-runner-2", "--format", "json", "--project", "docker-runners"}
	if !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("args = %q, want %q", r.calls[0].args, want)
	}
}

func TestNewestGoldenWithoutAGoldenSnapshotIsAnError(t *testing.T) {
	r, c := newClient()
	r.out["snapshot list"] = `[{"name": "before-cis"}]`
	if _, err := c.NewestGolden(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("no error with no golden snapshot")
	}
	r.out["snapshot list"] = `not json`
	if _, err := c.NewestGolden(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("no error on unparseable output")
	}
	r.fail["snapshot list"] = errors.New("exit status 1")
	if _, err := c.NewestGolden(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("no error when the list fails")
	}
}

func TestRunningReadsTheInstanceStateInTheProject(t *testing.T) {
	r, c := newClient()
	r.out["query"] = `{"status": "Running", "status_code": 103}`
	up, err := c.Running(context.Background(), "docker-runner-1")
	if err != nil || !up {
		t.Fatalf("running = %v, %v", up, err)
	}
	want := []string{"query", "lab:/1.0/instances/docker-runner-1/state?project=docker-runners"}
	if !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("args = %q, want %q", r.calls[0].args, want)
	}
	r.out["query"] = `{"status": "Stopped", "status_code": 102}`
	if up, err := c.Running(context.Background(), "docker-runner-1"); err != nil || up {
		t.Fatalf("stopped VM: running = %v, %v", up, err)
	}
	r.out["query"] = `garbage`
	if _, err := c.Running(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("no error on unparseable state")
	}
	r.fail["query"] = errors.New("exit status 1")
	if _, err := c.Running(context.Background(), "docker-runner-1"); err == nil {
		t.Fatal("no error when the query fails")
	}
}

func TestExecPassesStdinAndKeepsTheCommandAfterTheSeparator(t *testing.T) {
	r, c := newClient()
	err := c.Exec(context.Background(), "docker-runner-1", strings.NewReader("ZW5jb2RlZA=="),
		"/usr/local/sbin/start-jit-runner")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "lab:docker-runner-1", "--project", "docker-runners", "--",
		"/usr/local/sbin/start-jit-runner"}
	if !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("args = %q, want %q", r.calls[0].args, want)
	}
	if r.calls[0].stdin != "ZW5jb2RlZA==" {
		t.Fatalf("stdin = %q", r.calls[0].stdin)
	}
}

func TestExecWithoutStdinDisablesIt(t *testing.T) {
	r, c := newClient()
	if err := c.Exec(context.Background(), "docker-runner-1", nil, "docker", "info"); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "lab:docker-runner-1", "--project", "docker-runners",
		"--disable-stdin", "--", "docker", "info"}
	if !reflect.DeepEqual(r.calls[0].args, want) {
		t.Fatalf("args = %q, want %q", r.calls[0].args, want)
	}
	r.fail["exec"] = errors.New("exit status 1")
	if err := c.Exec(context.Background(), "docker-runner-1", nil, "docker", "info"); err == nil {
		t.Fatal("exec failure swallowed")
	}
}

// fakeIncus writes an incus stand-in that logs argv and stdin, then exits.
func fakeIncus(t *testing.T, code int) (bin, log string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "incus")
	log = filepath.Join(dir, "log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + log + "\ncat >> " + log +
		"\necho out\necho 'Error: boom' >&2\nexit " + strconv.Itoa(code) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

func TestCLIRunsTheBinaryWithArgsAndStdin(t *testing.T) {
	bin, log := fakeIncus(t, 0)
	out, err := CLI(bin)(context.Background(), strings.NewReader("in"), "list", "lab:")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "out\n" {
		t.Fatalf("stdout = %q", out)
	}
	got, _ := os.ReadFile(log)
	if string(got) != "list\nlab:\nin" {
		t.Fatalf("argv and stdin = %q", got)
	}
}

func TestCLIFailureCarriesTheCommandAndStderr(t *testing.T) {
	bin, _ := fakeIncus(t, 1)
	_, err := CLI(bin)(context.Background(), nil, "start", "lab:x")
	if err == nil || !strings.Contains(err.Error(), "Error: boom") ||
		!strings.Contains(err.Error(), "incus start lab:x") {
		t.Fatalf("err = %v, want the command and its stderr", err)
	}
}
