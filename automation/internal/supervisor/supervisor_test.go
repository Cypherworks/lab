package supervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Cypherworks/lab/automation/internal/runners"
)

var start = time.Date(2026, 11, 22, 15, 0, 0, 0, time.UTC)

// fakeClock advances instantly on Sleep and records each wait. After fires
// only when ticking is set, as a test sets it while a call in the VM hangs:
// otherwise the fake time would race the call's goroutine.
type fakeClock struct {
	now     time.Time
	sleeps  []time.Duration
	onTick  func()
	ticking bool
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	if !c.ticking {
		return nil
	}
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	if c.onTick != nil {
		c.onTick()
	}
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	if c.onTick != nil {
		c.onTick()
	}
	return ctx.Err()
}

// poll is one scripted answer to a runner lookup.
type poll struct {
	runner runners.Runner
	err    error
}

// fakeGitHub mints runners and answers lookups from a script.
type fakeGitHub struct {
	minted    []string
	labels    [][]string
	groups    []int64
	mintFails int
	polls     []poll
	deleted   []int64
	delFails  int
	listed    []runners.Runner
	listErr   error
	nextID    int64
}

func (g *fakeGitHub) MintJIT(_ context.Context, name string, group int64, labels []string) (runners.Runner, string, error) {
	g.minted = append(g.minted, name)
	if g.mintFails > 0 {
		g.mintFails--
		return runners.Runner{}, "", errors.New("POST /runners/generate-jitconfig: HTTP 503")
	}
	g.nextID++
	g.labels = append(g.labels, labels)
	g.groups = append(g.groups, group)
	return runners.Runner{ID: g.nextID, Name: name, Status: "offline"}, "config-" + name, nil
}

func (g *fakeGitHub) Get(_ context.Context, id int64) (runners.Runner, error) {
	if len(g.polls) == 0 {
		return runners.Runner{}, runners.ErrNotFound
	}
	p := g.polls[0]
	if len(g.polls) > 1 {
		g.polls = g.polls[1:]
	}
	p.runner.ID = id
	return p.runner, p.err
}

func (g *fakeGitHub) Delete(_ context.Context, id int64) error {
	if g.delFails > 0 {
		g.delFails--
		return errors.New("DELETE: HTTP 502")
	}
	g.deleted = append(g.deleted, id)
	return nil
}

func (g *fakeGitHub) List(context.Context) ([]runners.Runner, error) {
	return g.listed, g.listErr
}

// fakeIncus records each operation and fails named ones a set number of times.
type fakeIncus struct {
	ops      []string
	fails    map[string]int
	running  bool
	golden   string
	older    []string
	stdin    []string
	release  chan struct{}
	notReady int
	before   func(op string)
	hang     map[string]bool
}

func (f *fakeIncus) fail(op string) error {
	if f.before != nil {
		f.before(op)
	}
	if f.fails[op] > 0 {
		f.fails[op]--
		return errors.New(op + " failed")
	}
	return nil
}

func (f *fakeIncus) Stop(_ context.Context, vm string) error {
	f.ops = append(f.ops, "stop "+vm)
	if err := f.fail("stop"); err != nil {
		return err
	}
	f.running = false
	return nil
}

func (f *fakeIncus) Start(_ context.Context, vm string) error {
	f.ops = append(f.ops, "start "+vm)
	if err := f.fail("start"); err != nil {
		return err
	}
	f.running = true
	return nil
}

func (f *fakeIncus) Restore(_ context.Context, vm, snap string) error {
	f.ops = append(f.ops, "restore "+vm+" "+snap)
	return f.fail("restore")
}

func (f *fakeIncus) NewestGolden(_ context.Context, vm string) (string, error) {
	if err := f.fail("golden"); err != nil {
		return "", err
	}
	return f.golden, nil
}

func (f *fakeIncus) Goldens(context.Context, string) ([]string, error) {
	if err := f.fail("goldens"); err != nil {
		return nil, err
	}
	return append(slices.Clone(f.older), f.golden), nil
}

func (f *fakeIncus) Shutdown(_ context.Context, vm string, _ time.Duration) error {
	f.ops = append(f.ops, "shutdown "+vm)
	if err := f.fail("shutdown"); err != nil {
		return err
	}
	f.running = false
	return nil
}

func (f *fakeIncus) Snapshot(_ context.Context, vm, name string) error {
	f.ops = append(f.ops, "snapshot "+vm+" "+name)
	if err := f.fail("snapshot"); err != nil {
		return err
	}
	f.older = append(f.older, f.golden)
	f.golden = name
	return nil
}

func (f *fakeIncus) DeleteSnapshot(_ context.Context, vm, name string) error {
	f.ops = append(f.ops, "delete-snapshot "+vm+" "+name)
	if err := f.fail("delete-snapshot"); err != nil {
		return err
	}
	f.older = slices.DeleteFunc(f.older, func(g string) bool { return g == name })
	return nil
}

func (f *fakeIncus) Running(context.Context, string) (bool, error) {
	if err := f.fail("running"); err != nil {
		return false, err
	}
	return f.running, nil
}

func (f *fakeIncus) Exec(ctx context.Context, vm string, stdin io.Reader, command ...string) error {
	op := "exec " + strings.Join(command, " ")
	f.ops = append(f.ops, op)
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		f.stdin = append(f.stdin, string(b))
	}
	if f.hang[command[0]] {
		<-ctx.Done()
		return ctx.Err()
	}
	if command[0] == "sh" && f.release != nil {
		<-f.release
	}
	if command[0] == "docker" && f.notReady > 0 {
		f.notReady--
		return errors.New("docker not up")
	}
	return f.fail(command[0])
}

func newVM(t *testing.T) (*VM, *fakeGitHub, *fakeIncus, *fakeClock) {
	t.Helper()
	gh := &fakeGitHub{nextID: 100}
	inc := &fakeIncus{fails: map[string]int{}, running: true, golden: "golden-20261122T020000Z"}
	clk := &fakeClock{now: start}
	v := &VM{
		Name:    "docker-runner-1",
		GroupID: 1,
		Labels:  []string{"self-hosted", "docker"},
		GitHub:  gh,
		Incus:   inc,
		Clock:   clk,
		Timing:  DefaultTiming(),
	}
	return v, gh, inc, clk
}

func online(busy bool) poll {
	return poll{runner: runners.Runner{Status: "online", Busy: busy}}
}

func TestACycleResetsMintsHandsOffAndWaitsForTheJob(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	gh.polls = []poll{online(false), online(true), {err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantOps := []string{
		"stop docker-runner-1",
		"restore docker-runner-1 golden-20261122T020000Z",
		"start docker-runner-1",
		"exec docker info",
		"exec /usr/local/sbin/start-jit-runner",
		"stop docker-runner-1",
	}
	if !reflect.DeepEqual(inc.ops, wantOps) {
		t.Fatalf("incus ops = %q, want %q", inc.ops, wantOps)
	}
	if !reflect.DeepEqual(gh.minted, []string{"docker-runner-1-20261122T150000Z"}) {
		t.Fatalf("minted = %q", gh.minted)
	}
	if !reflect.DeepEqual(gh.labels[0], []string{"self-hosted", "docker"}) || gh.groups[0] != 1 {
		t.Fatalf("labels %q group %d", gh.labels[0], gh.groups[0])
	}
	if !reflect.DeepEqual(inc.stdin, []string{"config-docker-runner-1-20261122T150000Z"}) {
		t.Fatalf("config handed in = %q", inc.stdin)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v, want the job's runner removed", gh.deleted)
	}
	if v.Jobs() != 1 {
		t.Fatalf("jobs = %d", v.Jobs())
	}
}

func TestAJobEndsWhenBusyDrops(t *testing.T) {
	v, gh, _, clk := newVM(t)
	gh.polls = []poll{online(true), online(true), online(false), online(false)}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countSleeps(clk, v.Timing.Poll); got != 3 {
		t.Fatalf("%d polls, want 3", got)
	}
}

func TestAJobEndsWhenTheRunnerGoesOffline(t *testing.T) {
	v, gh, _, clk := newVM(t)
	gh.polls = []poll{online(false), {runner: runners.Runner{Status: "offline"}}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countSleeps(clk, v.Timing.Poll); got != 2 {
		t.Fatalf("%d polls, want 2", got)
	}
}

func TestAJobEndsWhenTheVMStops(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	gh.polls = []poll{online(true)}
	clk.onTick = func() {
		if countSleeps(clk, v.Timing.Poll) == 2 {
			inc.running = false
		}
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countSleeps(clk, v.Timing.Poll); got != 2 {
		t.Fatalf("%d polls, want 2", got)
	}
}

func TestARunnerThatNeverComesOnlineIsDeletedAfterTheDeadline(t *testing.T) {
	v, gh, _, clk := newVM(t)
	gh.polls = []poll{{runner: runners.Runner{Status: "offline"}}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v", gh.deleted)
	}
	if waited := clk.now.Sub(start); waited < v.Timing.Online {
		t.Fatalf("gave up after %v, want at least %v", waited, v.Timing.Online)
	}
	if v.Jobs() != 0 {
		t.Fatalf("jobs = %d, want 0 for a runner that never came online", v.Jobs())
	}
}

func TestAJobPastTheLifetimeCapIsKilled(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	gh.polls = []poll{online(true)}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if up := clk.now.Sub(start); up < v.Timing.Lifetime || up > v.Timing.Lifetime+v.Timing.Poll {
		t.Fatalf("killed after %v, want the %v cap", up, v.Timing.Lifetime)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v", gh.deleted)
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped at the cap", last)
	}
}

func TestLookupErrorsKeepPolling(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.fails["running"] = 1
	gh.polls = []poll{online(true), {err: errors.New("GET: HTTP 502")}, online(false)}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if v.Jobs() != 1 {
		t.Fatalf("jobs = %d, want the job seen through the errors", v.Jobs())
	}
}

func TestAFailedResetIsRetriedWithBackoff(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	inc.fails["restore"] = 2
	at := opTimes(inc, clk, "restore")
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{30 * time.Second, 2 * time.Minute}
	if got := gaps(*at); !reflect.DeepEqual(got, want) {
		t.Fatalf("restores %v apart, want %v", got, want)
	}
	if len(gh.minted) != 1 {
		t.Fatalf("minted %d, want 1 after the reset recovered", len(gh.minted))
	}
}

func TestAResetThatKeepsFailingLeavesTheVMStoppedAndFailed(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	inc.fails["start"] = 4
	at := opTimes(inc, clk, "start")
	err := v.Cycle(context.Background())
	if !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v, want ErrFailed", err)
	}
	want := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}
	if got := gaps(*at); !reflect.DeepEqual(got, want) {
		t.Fatalf("starts %v apart, want %v", got, want)
	}
	if got, total := clk.now.Sub(start), 12*time.Minute+30*time.Second; got != total {
		t.Fatalf("slept %v in all, want only the backoff, %v", got, total)
	}
	if len(gh.minted) != 0 {
		t.Fatal("minted a runner for a VM whose reset failed")
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the failed VM stopped", last)
	}
	if v.State() != Failed {
		t.Fatalf("state = %v", v.State())
	}
}

func TestNoGoldenSnapshotIsAResetFailure(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.fails["golden"] = 4
	if err := v.Cycle(context.Background()); !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v, want ErrFailed", err)
	}
	for _, op := range inc.ops {
		if strings.HasPrefix(op, "start") {
			t.Fatalf("started a VM with no golden snapshot: %q", inc.ops)
		}
	}
	if len(gh.minted) != 0 {
		t.Fatal("minted without a golden snapshot")
	}
}

func TestDockerNotReadyInTimeIsAResetFailure(t *testing.T) {
	v, _, inc, _ := newVM(t)
	inc.notReady = 1 << 30
	if err := v.Cycle(context.Background()); !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v, want ErrFailed", err)
	}
}

func TestDockerReadyAfterAFewTriesProceeds(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.notReady = 3
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := countOps(inc, "exec docker info"); n != 4 {
		t.Fatalf("docker info ran %d times, want 4", n)
	}
}

func TestMintFailuresRetryWithAFreshNameAndCappedBackoff(t *testing.T) {
	v, gh, _, _ := newVM(t)
	gh.mintFails = 6
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(gh.minted) != 7 {
		t.Fatalf("mint attempts = %d, want 7", len(gh.minted))
	}
	seen := map[string]bool{}
	for _, n := range gh.minted {
		if seen[n] {
			t.Fatalf("name %q reused", n)
		}
		seen[n] = true
	}
	var at []time.Time
	for _, n := range gh.minted {
		ts, err := time.Parse(nameTime, strings.TrimPrefix(n, v.Name+"-"))
		if err != nil {
			t.Fatal(err)
		}
		at = append(at, ts)
	}
	want := []time.Duration{15 * time.Second, 30 * time.Second, time.Minute,
		2 * time.Minute, 4 * time.Minute, 5 * time.Minute}
	if got := gaps(at); !reflect.DeepEqual(got, want) {
		t.Fatalf("mints %v apart, want %v", got, want)
	}
	if v.MintFailures() != 6 {
		t.Fatalf("mint failures = %d", v.MintFailures())
	}
}

func TestAFailedHandOffDeletesTheRunnerAndEndsTheCycle(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	inc.fails["/usr/local/sbin/start-jit-runner"] = 1
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v", gh.deleted)
	}
	if countSleeps(clk, v.Timing.Poll) != 0 {
		t.Fatal("watched a runner whose hand-off failed")
	}
}

func TestAFailedDeleteIsRetriedOnTheNextCycle(t *testing.T) {
	v, gh, _, _ := newVM(t)
	gh.delFails = 1
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(gh.deleted) != 0 {
		t.Fatalf("deleted = %v", gh.deleted)
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101, 102}) {
		t.Fatalf("deleted = %v, want the earlier runner retried", gh.deleted)
	}
}

func TestDrainWhileIdleDeletesTheRunnerAndStopsTheVM(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	gh.polls = []poll{online(false)}
	ctx, cancel := context.WithCancel(context.Background())
	clk.onTick = func() {
		if countSleeps(clk, v.Timing.Poll) == 2 {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v", gh.deleted)
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped", last)
	}
}

func TestDrainWhileBusyWaitsForTheJobThenStops(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	gh.polls = []poll{online(true), online(true), online(true), online(true), online(false)}
	ctx, cancel := context.WithCancel(context.Background())
	clk.onTick = func() {
		if countSleeps(clk, v.Timing.Poll) == 2 {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if v.Jobs() != 1 {
		t.Fatalf("jobs = %d, want the job allowed to finish", v.Jobs())
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped", last)
	}
}

func TestDrainDuringAResetStopsTheVMWithoutMinting(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	inc.fails["restore"] = 1
	ctx, cancel := context.WithCancel(context.Background())
	clk.onTick = cancel
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if len(gh.minted) != 0 {
		t.Fatal("minted while draining")
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped", last)
	}
}

func TestCleanupDeletesOnlyThisVMsRunners(t *testing.T) {
	v, gh, _, _ := newVM(t)
	gh.listed = []runners.Runner{
		{ID: 1, Name: "docker-runner-1-20261122T140000Z"},
		{ID: 2, Name: "docker-runner-10-20261122T140000Z"},
		{ID: 3, Name: "docker-runner-2-20261122T140000Z"},
		{ID: 4, Name: "ci-runner-1"},
		{ID: 5, Name: "docker-runner-1-manual"},
		{ID: 6, Name: "docker-runner-1-20261122T150000Z"},
	}
	if err := v.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{1, 6}) {
		t.Fatalf("deleted = %v, want only docker-runner-1's minted runners", gh.deleted)
	}
}

func TestStateFollowsTheCycle(t *testing.T) {
	v, gh, _, clk := newVM(t)
	gh.polls = []poll{online(false), online(true), {err: runners.ErrNotFound}}
	var seen []State
	clk.onTick = func() {
		if s := v.State(); len(seen) == 0 || seen[len(seen)-1] != s {
			seen = append(seen, s)
		}
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []State{Registering, Idle, Busy}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("states seen at each poll = %v, want %v", seen, want)
	}
}

func TestMetricsFileDescribesTheVM(t *testing.T) {
	v, gh, _, _ := newVM(t)
	v.MetricsDir = t.TempDir()
	gh.polls = []poll{online(true), {err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(v.MetricsDir, "docker_runner_docker-runner-1.prom"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`docker_runner_state{vm="docker-runner-1",state="resetting"} 1`,
		`docker_runner_state{vm="docker-runner-1",state="busy"} 0`,
		`docker_runner_state{vm="docker-runner-1",state="failed"} 0`,
		`docker_runner_jobs_total{vm="docker-runner-1"} 1`,
		`docker_runner_resets_total{vm="docker-runner-1",result="ok"} 1`,
		`docker_runner_mint_failures_total{vm="docker-runner-1"} 0`,
		`docker_runner_golden_timestamp_seconds{vm="docker-runner-1"} 1795312800`,
	} {
		if !strings.Contains(string(b), line+"\n") {
			t.Fatalf("metrics lack %q:\n%s", line, b)
		}
	}
	if !strings.Contains(string(b), `docker_runner_heartbeat_timestamp_seconds{vm="docker-runner-1"} `) {
		t.Fatalf("metrics lack the heartbeat:\n%s", b)
	}
}

// The heartbeat means the process is alive, so no wait may outlast a poll.
func TestTheHeartbeatStaysFreshThroughEveryWait(t *testing.T) {
	cases := map[string]func(*fakeGitHub, *fakeIncus){
		"reset backoff":    func(_ *fakeGitHub, inc *fakeIncus) { inc.fails["start"] = 4 },
		"mint backoff":     func(gh *fakeGitHub, _ *fakeIncus) { gh.mintFails = 6 },
		"docker not ready": func(_ *fakeGitHub, inc *fakeIncus) { inc.notReady = 120 },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			v, gh, inc, clk := newVM(t)
			v.MetricsDir = t.TempDir()
			gh.polls = []poll{{err: runners.ErrNotFound}}
			setup(gh, inc)
			var stalest time.Duration
			clk.onTick = func() {
				if age := clk.now.Sub(heartbeat(t, v)); age > stalest {
					stalest = age
				}
			}
			_ = v.Cycle(context.Background())
			if stalest > v.Timing.Poll {
				t.Fatalf("the heartbeat went %v stale, want at most a poll (%v)", stalest, v.Timing.Poll)
			}
		})
	}
}

func heartbeat(t *testing.T, v *VM) time.Time {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(v.MetricsDir, "docker_runner_"+v.Name+".prom"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`docker_runner_heartbeat_timestamp_seconds\{[^}]*\} (\d+)`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("no heartbeat in:\n%s", b)
	}
	s, _ := strconv.ParseInt(string(m[1]), 10, 64)
	return time.Unix(s, 0)
}

// A drain is maintenance, so the metrics must tell it from a dead process.
func TestADrainedVMSaysSoInItsMetrics(t *testing.T) {
	cases := map[string]func(*VM, *fakeGitHub, *fakeIncus, *fakeClock, context.CancelFunc){
		"while idle": func(v *VM, gh *fakeGitHub, _ *fakeIncus, clk *fakeClock, cancel context.CancelFunc) {
			gh.polls = []poll{online(false)}
			clk.onTick = func() {
				if countSleeps(clk, v.Timing.Poll) == 2 {
					cancel()
				}
			}
		},
		"during a reset's backoff": func(_ *VM, _ *fakeGitHub, inc *fakeIncus, clk *fakeClock, cancel context.CancelFunc) {
			inc.fails["start"] = 1
			clk.onTick = cancel
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			v, gh, inc, clk := newVM(t)
			v.MetricsDir = t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			setup(v, gh, inc, clk, cancel)
			if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
				t.Fatalf("err = %v, want ErrDrained", err)
			}
			if v.State() != Drained {
				t.Fatalf("state = %v, want drained", v.State())
			}
			b, err := os.ReadFile(filepath.Join(v.MetricsDir, "docker_runner_docker-runner-1.prom"))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range []string{
				`docker_runner_state{vm="docker-runner-1",state="drained"} 1`,
				`docker_runner_state{vm="docker-runner-1",state="resetting"} 0`,
				fmt.Sprintf(`docker_runner_state_since_seconds{vm="docker-runner-1"} %d`, clk.now.Unix()),
			} {
				if !strings.Contains(string(b), line+"\n") {
					t.Fatalf("metrics lack %q:\n%s", line, b)
				}
			}
		})
	}
}

func countSleeps(c *fakeClock, d time.Duration) int {
	n := 0
	for _, s := range c.sleeps {
		if s == d {
			n++
		}
	}
	return n
}

// opTimes records the fake clock's time at each Incus call of kind op.
func opTimes(f *fakeIncus, c *fakeClock, op string) *[]time.Time {
	var at []time.Time
	f.before = func(o string) {
		if o == op {
			at = append(at, c.Now())
		}
	}
	return &at
}

// gaps is the time between each pair of consecutive instants.
func gaps(at []time.Time) []time.Duration {
	var d []time.Duration
	for i := 1; i < len(at); i++ {
		d = append(d, at[i].Sub(at[i-1]))
	}
	return d
}

func countOps(f *fakeIncus, op string) int {
	n := 0
	for _, o := range f.ops {
		if o == op {
			n++
		}
	}
	return n
}

func TestDrainDuringMintBackoffStopsTheVM(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	gh.mintFails = 1
	ctx, cancel := context.WithCancel(context.Background())
	clk.onTick = cancel
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped", last)
	}
}

func TestDrainDuringAMintCallStopsTheVM(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	gh.mintFails = 1
	ctx, cancel := context.WithCancel(context.Background())
	inc.before = func(op string) {
		if op == "docker" {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if v.MintFailures() != 0 {
		t.Fatalf("mint failures = %d, want a drain not counted as a failure", v.MintFailures())
	}
}

func TestDrainDuringAFailedHandOffDeletesAndStops(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.fails[startJITRunner] = 1
	ctx, cancel := context.WithCancel(context.Background())
	inc.before = func(op string) {
		if op == startJITRunner {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v", gh.deleted)
	}
}

func TestDrainDuringAFailingResetStepStopsTheVM(t *testing.T) {
	v, _, inc, clk := newVM(t)
	inc.fails["restore"] = 1
	ctx, cancel := context.WithCancel(context.Background())
	inc.before = func(op string) {
		if op == "restore" {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if len(clk.sleeps) != 0 {
		t.Fatalf("backed off %v while draining", clk.sleeps)
	}
	if last := inc.ops[len(inc.ops)-1]; last != "stop docker-runner-1" {
		t.Fatalf("last op = %q, want the VM stopped", last)
	}
}

func TestDrainWhileWaitingForDockerStopsTheVM(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	inc.notReady = 1 << 30
	ctx, cancel := context.WithCancel(context.Background())
	clk.onTick = cancel
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if len(gh.minted) != 0 {
		t.Fatal("minted while draining")
	}
}

func TestAFailedStopIsRetriedAsPartOfTheReset(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.fails["stop"] = 1
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(gh.minted) != 1 {
		t.Fatalf("minted %d, want 1 after the stop recovered", len(gh.minted))
	}
}

func TestAStopThatFailsAtTheEndIsLogged(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	var logs bytes.Buffer
	v.Log = slog.New(slog.NewTextHandler(&logs, nil))
	gh.polls = []poll{{err: runners.ErrNotFound}}
	inc.before = func(op string) {
		if op == "stop" && countOps(inc, "stop docker-runner-1") == 2 {
			inc.fails["stop"] = 1
		}
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "stop failed") {
		t.Fatalf("logs lack the failed stop:\n%s", logs.String())
	}
}

func TestADeleteThatFailsTwiceIsKeptForTheCycleAfter(t *testing.T) {
	v, gh, _, _ := newVM(t)
	gh.delFails = 2
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.deleted, []int64{102, 101, 103}) {
		t.Fatalf("deleted = %v, want 101 retried until it went", gh.deleted)
	}
}

func TestCleanupFailsWhenTheListFails(t *testing.T) {
	v, gh, _, _ := newVM(t)
	gh.listErr = errors.New("GET /runners: HTTP 503")
	if err := v.Cleanup(context.Background()); err == nil {
		t.Fatal("cleanup succeeded without the list")
	}
}

func TestLogsNeverCarryTheJITConfig(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	var logs bytes.Buffer
	v.Log = slog.New(slog.NewTextHandler(&logs, nil))
	inc.fails[startJITRunner] = 1
	gh.delFails = 1
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	gh.polls = []poll{online(true), {err: errors.New("GET: HTTP 502")}, online(false)}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "vm=docker-runner-1") {
		t.Fatalf("logs lack the VM:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "config-") {
		t.Fatalf("logs carry a JIT config:\n%s", logs.String())
	}
}

func TestMetricsWriteFailuresAreLogged(t *testing.T) {
	v, gh, _, _ := newVM(t)
	var logs bytes.Buffer
	v.Log = slog.New(slog.NewTextHandler(&logs, nil))
	v.MetricsDir = filepath.Join(t.TempDir(), "missing")
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "writing metrics failed") {
		t.Fatalf("logs lack the metrics failure:\n%s", logs.String())
	}
	logs.Reset()
	v.MetricsDir = t.TempDir()
	if err := os.Mkdir(filepath.Join(v.MetricsDir, "docker_runner_docker-runner-1.prom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "writing metrics failed") {
		t.Fatalf("logs lack the rename failure:\n%s", logs.String())
	}
}

func TestStatesHaveTheirMetricNames(t *testing.T) {
	got := []string{Resetting.String(), Registering.String(), Idle.String(), Busy.String(), Failed.String(),
		Drained.String(), Refreshing.String()}
	want := []string{"resetting", "registering", "idle", "busy", "failed", "drained", "refreshing"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %q", got)
	}
}

func TestRealClockAfterFiresOnceTheTimeHasPassed(t *testing.T) {
	c := RealClock{}
	before := c.Now()
	<-c.After(20 * time.Millisecond)
	if took := c.Now().Sub(before); took < 20*time.Millisecond {
		t.Fatalf("fired after %v, want 20ms", took)
	}
}

func TestRealClockSleepsAndStopsEarlyOnCancel(t *testing.T) {
	c := RealClock{}
	before := c.Now()
	if err := c.Sleep(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if c.Now().Sub(before) < 20*time.Millisecond {
		t.Fatal("woke early")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestDrainDuringTheLastResetAttemptIsNotAFailure(t *testing.T) {
	v, _, inc, _ := newVM(t)
	inc.fails["restore"] = 4
	ctx, cancel := context.WithCancel(context.Background())
	inc.before = func(op string) {
		if op == "restore" && countOps(inc, "restore docker-runner-1 golden-20261122T020000Z") == 4 {
			cancel()
		}
	}
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if v.State() == Failed {
		t.Fatal("a drained VM was marked failed")
	}
}

func TestAHungHandOffTimesOutAndIsTreatedAsFailed(t *testing.T) {
	v, gh, inc, clk := newVM(t)
	v.Timing.Exec = 20 * time.Millisecond
	inc.hang = map[string]bool{startJITRunner: true}
	done := make(chan error, 1)
	go func() { done <- v.Cycle(context.Background()) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a hung hand-off blocked the loop")
	}
	if !reflect.DeepEqual(gh.deleted, []int64{101}) {
		t.Fatalf("deleted = %v, want the runner removed", gh.deleted)
	}
	if countSleeps(clk, v.Timing.Poll) != 0 {
		t.Fatal("watched a runner whose hand-off hung")
	}
}

func TestAHungDockerCheckTimesOutAndCountsAsNotReady(t *testing.T) {
	v, _, inc, _ := newVM(t)
	v.Timing.Exec = 20 * time.Millisecond
	v.Timing.Ready = 0
	inc.hang = map[string]bool{"docker": true}
	done := make(chan error, 1)
	go func() { done <- v.Cycle(context.Background()) }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrFailed) {
			t.Fatalf("err = %v, want ErrFailed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a hung docker check blocked the loop")
	}
}

// weekOld is a VM whose golden turns a week old during its first cycle,
// which has run, so its next cycle refreshes the golden.
func weekOld(t *testing.T) (*VM, *fakeGitHub, *fakeIncus, *fakeClock) {
	t.Helper()
	v, gh, inc, clk := newVM(t)
	inc.golden = "golden-20261115T150000Z"
	gh.polls = []poll{{err: runners.ErrNotFound}}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	inc.ops = nil
	return v, gh, inc, clk
}

var goldenName = regexp.MustCompile(`^golden-\d{8}T\d{6}Z$`)

// snapshots lists the snapshots the cycle took, by name.
func snapshots(f *fakeIncus) []string {
	var names []string
	for _, o := range f.ops {
		if name, ok := strings.CutPrefix(o, "snapshot docker-runner-1 "); ok {
			names = append(names, name)
		}
	}
	return names
}

func TestAWeekOldGoldenIsRefreshedBeforeTheNextReset(t *testing.T) {
	v, gh, inc, _ := weekOld(t)
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	taken := snapshots(inc)
	if len(taken) != 1 || !goldenName.MatchString(taken[0]) || taken[0] <= "golden-20261122T150000Z" {
		t.Fatalf("snapshots taken = %q, want one golden-<now>", taken)
	}
	wantOps := []string{
		"stop docker-runner-1",
		"restore docker-runner-1 golden-20261115T150000Z",
		"start docker-runner-1",
		"exec docker info",
		"exec " + strings.Join(upgrade, " "),
		"shutdown docker-runner-1",
		"start docker-runner-1",
		"exec docker info",
		"exec " + checkGolden,
		"shutdown docker-runner-1",
		"snapshot docker-runner-1 " + taken[0],
		"stop docker-runner-1",
		"restore docker-runner-1 " + taken[0],
		"start docker-runner-1",
		"exec docker info",
		"exec /usr/local/sbin/start-jit-runner",
		"stop docker-runner-1",
	}
	if !reflect.DeepEqual(inc.ops, wantOps) {
		t.Fatalf("incus ops = %q\nwant %q", inc.ops, wantOps)
	}
	if len(gh.minted) != 2 {
		t.Fatalf("minted = %q, want the refresh to mint nothing", gh.minted)
	}
}

func TestAGoldenUnderAWeekOldIsNotRefreshed(t *testing.T) {
	v, gh, inc, _ := newVM(t)
	inc.golden = "golden-20261115T150100Z"
	gh.polls = []poll{{err: runners.ErrNotFound}}
	for range 2 {
		if err := v.Cycle(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if taken := snapshots(inc); len(taken) != 0 {
		t.Fatalf("snapshots taken = %q, want none", taken)
	}
}

func TestARefreshKeepsTheNewestTwoGoldens(t *testing.T) {
	v, _, inc, _ := weekOld(t)
	inc.older = []string{"golden-20261108T150000Z"}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := countOps(inc, "delete-snapshot docker-runner-1 golden-20261108T150000Z"); got != 1 {
		t.Fatalf("ops = %q, want the oldest golden deleted", inc.ops)
	}
	if want := []string{"golden-20261115T150000Z"}; !reflect.DeepEqual(inc.older, want) {
		t.Fatalf("goldens left = %q + %q, want %q + the new one", inc.older, inc.golden, want)
	}
}

func TestAFailedPruneIsLoggedAndTheNewGoldenStands(t *testing.T) {
	cases := map[string]string{"listing": "goldens", "deleting": "delete-snapshot"}
	for name, op := range cases {
		t.Run(name, func(t *testing.T) {
			v, _, inc, _ := weekOld(t)
			var logs bytes.Buffer
			v.Log = slog.New(slog.NewTextHandler(&logs, nil))
			inc.older = []string{"golden-20261108T150000Z"}
			inc.fails[op] = 1
			if err := v.Cycle(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(logs.String(), "pruning old goldens failed") {
				t.Fatalf("logs lack the prune failure:\n%s", logs.String())
			}
			if taken := snapshots(inc); len(taken) != 1 || countOps(inc, "restore docker-runner-1 "+taken[0]) != 1 {
				t.Fatalf("ops = %q, want the new golden restored", inc.ops)
			}
		})
	}
}

// arm runs set the first time the fake sees op, to fail a later step.
func arm(f *fakeIncus, op string, set func()) {
	armed := false
	f.before = func(o string) {
		if o == op && !armed {
			armed = true
			set()
		}
	}
}

func TestAFailedRefreshKeepsTheOldGoldenAndRetriesADayLater(t *testing.T) {
	cases := map[string]func(*VM, *fakeIncus){
		"restore":  func(_ *VM, inc *fakeIncus) { inc.fails["restore"] = 1 },
		"upgrade":  func(_ *VM, inc *fakeIncus) { inc.fails["sh"] = 1 },
		"reboot":   func(_ *VM, inc *fakeIncus) { inc.fails["shutdown"] = 1 },
		"boot":     func(_ *VM, inc *fakeIncus) { arm(inc, "shutdown", func() { inc.fails["start"] = 1 }) },
		"checks":   func(_ *VM, inc *fakeIncus) { inc.fails[checkGolden] = 1 },
		"snapshot": func(_ *VM, inc *fakeIncus) { inc.fails["snapshot"] = 1 },
		"docker after the reboot": func(v *VM, inc *fakeIncus) {
			v.Timing.Ready = 0
			arm(inc, "shutdown", func() { inc.notReady = 1 })
		},
		"clean stop": func(_ *VM, inc *fakeIncus) {
			arm(inc, checkGolden, func() { inc.fails["shutdown"] = 1 })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			v, gh, inc, clk := weekOld(t)
			var logs bytes.Buffer
			v.Log = slog.New(slog.NewTextHandler(&logs, nil))
			setup(v, inc)
			if err := v.Cycle(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(logs.String(), "golden refresh failed") {
				t.Fatalf("logs lack the failure:\n%s", logs.String())
			}
			if inc.golden != "golden-20261115T150000Z" {
				t.Fatalf("golden = %q, want the old one kept", inc.golden)
			}
			if len(gh.minted) < 2 {
				t.Fatalf("minted = %q, want the cycle to carry on after the failure", gh.minted)
			}
			failedAt := clk.now
			inc.ops = nil
			if err := v.Cycle(context.Background()); err != nil {
				t.Fatal(err)
			}
			if countOps(inc, "exec "+strings.Join(upgrade, " ")) != 0 {
				t.Fatalf("refreshed again straight after a failure: %q", inc.ops)
			}
			clk.now = failedAt.Add(v.Timing.RefreshRetry)
			if err := v.Cycle(context.Background()); err != nil {
				t.Fatal(err)
			}
			if taken := snapshots(inc); len(taken) != 1 {
				t.Fatalf("snapshots = %q, want a retry a day after the failure", taken)
			}
		})
	}
}

func TestAFailedRefreshLeavesTheVMStopped(t *testing.T) {
	v, _, inc, _ := weekOld(t)
	inc.fails[checkGolden] = 1
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	i := slices.Index(inc.ops, "exec "+checkGolden)
	if i < 0 || i+1 >= len(inc.ops) || inc.ops[i+1] != "stop docker-runner-1" {
		t.Fatalf("ops = %q, want a stop straight after the failed checks", inc.ops)
	}
}

func TestAHungUpgradeTimesOutAndTakesNoSnapshot(t *testing.T) {
	v, _, inc, clk := weekOld(t)
	inc.hang = map[string]bool{"sh": true}
	clk.ticking = true
	at := opTimes(inc, clk, "shutdown")
	began := clk.now
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if taken := snapshots(inc); len(taken) != 0 {
		t.Fatalf("snapshots = %q, want none after a hung upgrade", taken)
	}
	if len(*at) != 0 {
		t.Fatalf("shut down after a hung upgrade at %v", *at)
	}
	if took := clk.now.Sub(began); took < v.Timing.Upgrade {
		t.Fatalf("gave up after %v, want the %v deadline", took, v.Timing.Upgrade)
	}
}

func TestTheHeartbeatStaysFreshThroughARefresh(t *testing.T) {
	v, _, inc, clk := weekOld(t)
	v.MetricsDir = t.TempDir()
	inc.release = make(chan struct{})
	clk.ticking = true
	var stalest time.Duration
	ticks := 0
	var refreshing bool
	clk.onTick = func() {
		if age := clk.now.Sub(heartbeat(t, v)); age > stalest {
			stalest = age
		}
		refreshing = refreshing || v.State() == Refreshing
		if ticks++; ticks == 40 {
			close(inc.release)
			clk.ticking = false
		}
	}
	if err := v.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ticks < 40 {
		t.Fatalf("%d ticks, want the upgrade to have run for 40 polls", ticks)
	}
	if stalest > v.Timing.Poll {
		t.Fatalf("the heartbeat went %v stale, want at most a poll (%v)", stalest, v.Timing.Poll)
	}
	if !refreshing {
		t.Fatal("never read refreshing during the refresh")
	}
}

func TestDrainDuringARefreshStopsTheVMWithoutASnapshot(t *testing.T) {
	v, gh, inc, clk := weekOld(t)
	ctx, cancel := context.WithCancel(context.Background())
	inc.hang = map[string]bool{"sh": true}
	clk.ticking = true
	clk.onTick = cancel
	if err := v.Cycle(ctx); !errors.Is(err, ErrDrained) {
		t.Fatalf("err = %v, want ErrDrained", err)
	}
	if taken := snapshots(inc); len(taken) != 0 {
		t.Fatalf("snapshots = %q, want none", taken)
	}
	if inc.running || inc.ops[len(inc.ops)-1] != "stop docker-runner-1" {
		t.Fatalf("ops = %q, want the VM stopped", inc.ops)
	}
	if len(gh.minted) != 1 || v.State() != Drained {
		t.Fatalf("minted %q, state %v; want no mint and drained", gh.minted, v.State())
	}
}
