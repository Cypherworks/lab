// Package supervisor runs one Docker runner VM: it restores the VM to its
// golden snapshot, registers it with a single-job JIT config, waits for the
// job to end by watching GitHub and Incus, and starts again. The signal that
// a job ended never comes from the VM, since the job controls the VM.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Cypherworks/lab/automation/internal/runners"
)

// startJITRunner is the golden image's hand-off script; it reads the config.
const startJITRunner = "/usr/local/sbin/start-jit-runner"

// nameTime stamps runner and golden snapshot names in UTC.
const nameTime = "20060102T150405Z"

// goldenPrefix starts every golden snapshot's name.
const goldenPrefix = "golden-"

// GitHub is the part of the runners client the loop uses.
type GitHub interface {
	MintJIT(ctx context.Context, name string, groupID int64, labels []string) (runners.Runner, string, error)
	Get(ctx context.Context, id int64) (runners.Runner, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]runners.Runner, error)
}

// Incus is the part of the incus client the loop uses.
type Incus interface {
	Stop(ctx context.Context, vm string) error
	Start(ctx context.Context, vm string) error
	Restore(ctx context.Context, vm, snapshot string) error
	NewestGolden(ctx context.Context, vm string) (string, error)
	Running(ctx context.Context, vm string) (bool, error)
	Exec(ctx context.Context, vm string, stdin io.Reader, command ...string) error
}

// Clock tells the time and waits, so tests can run hours in microseconds.
type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

// RealClock is the wall clock.
type RealClock struct{}

// Now is time.Now.
func (RealClock) Now() time.Time { return time.Now() }

// Sleep waits d, or returns early with ctx's error.
func (RealClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// State is where a VM is in its cycle.
type State int

const (
	Resetting State = iota
	Registering
	Idle
	Busy
	Failed
)

var stateNames = [...]string{"resetting", "registering", "idle", "busy", "failed"}

func (s State) String() string { return stateNames[s] }

// ErrFailed means the VM's reset kept failing and the VM is stopped.
var ErrFailed = errors.New("reset kept failing")

// ErrDrained means the loop was told to stop and has stopped the VM.
var ErrDrained = errors.New("drained")

// Timing holds the loop's intervals, deadlines and backoffs.
type Timing struct {
	Poll             time.Duration
	Online           time.Duration
	Lifetime         time.Duration
	Ready            time.Duration
	ReadyPoll        time.Duration
	Exec             time.Duration
	ResetBackoff     []time.Duration
	MintBackoffStart time.Duration
	MintBackoffMax   time.Duration
}

// DefaultTiming is lab-deploy docs/docker-runner-reset.md's timing.
func DefaultTiming() Timing {
	return Timing{
		Poll:             15 * time.Second,
		Online:           5 * time.Minute,
		Lifetime:         time.Hour,
		Ready:            5 * time.Minute,
		ReadyPoll:        2 * time.Second,
		Exec:             time.Minute,
		ResetBackoff:     []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute},
		MintBackoffStart: 15 * time.Second,
		MintBackoffMax:   5 * time.Minute,
	}
}

// VM supervises one Docker runner VM. Use it from one goroutine.
type VM struct {
	Name       string
	GroupID    int64
	Labels     []string
	GitHub     GitHub
	Incus      Incus
	Clock      Clock
	Timing     Timing
	MetricsDir string
	Log        *slog.Logger

	state        State
	since        time.Time
	golden       string
	jobs         int
	mintFailures int
	resetsOK     int
	resetsFailed int
	undeleted    []int64
}

// Cycle runs one reset, registration and job, and leaves the VM stopped.
func (v *VM) Cycle(ctx context.Context) error {
	v.retryDeletes(ctx)
	started, err := v.reset(ctx)
	if err != nil {
		return err
	}
	runner, config, err := v.mint(ctx)
	if err != nil {
		v.stop(ctx)
		return ErrDrained
	}
	v.setState(Registering)
	err = v.exec(ctx, strings.NewReader(config), startJITRunner)
	if err != nil {
		v.log().Warn("hand-off failed", "runner", runner.ID, "err", err)
		v.finish(ctx, runner.ID)
		if ctx.Err() != nil {
			return ErrDrained
		}
		return nil
	}
	return v.watch(ctx, runner.ID, started)
}

// Cleanup deletes every runner this VM's loop minted and may have left.
func (v *VM) Cleanup(ctx context.Context) error {
	all, err := v.GitHub.List(ctx)
	if err != nil {
		return err
	}
	mine := regexp.MustCompile(`^` + regexp.QuoteMeta(v.Name) + `-\d{8}T\d{6}Z$`)
	var errs []error
	for _, r := range all {
		if mine.MatchString(r.Name) {
			v.log().Info("deleting a runner left by an earlier run", "runner", r.ID, "name", r.Name)
			errs = append(errs, v.GitHub.Delete(ctx, r.ID))
		}
	}
	return errors.Join(errs...)
}

// State reports where the VM is in its cycle.
func (v *VM) State() State { return v.state }

// Jobs counts the jobs the VM has run.
func (v *VM) Jobs() int { return v.jobs }

// MintFailures counts failed JIT mints.
func (v *VM) MintFailures() int { return v.mintFailures }

// reset restores the newest golden snapshot and boots it, with backoff.
func (v *VM) reset(ctx context.Context) (time.Time, error) {
	for attempt := 0; ; attempt++ {
		v.setState(Resetting)
		started, err := v.resetOnce(ctx)
		if err == nil {
			v.resetsOK++
			v.publish()
			return started, nil
		}
		if ctx.Err() != nil {
			v.stop(ctx)
			return time.Time{}, ErrDrained
		}
		v.resetsFailed++
		v.log().Warn("reset failed", "attempt", attempt+1, "err", err)
		if attempt == len(v.Timing.ResetBackoff) {
			v.stop(ctx)
			v.setState(Failed)
			return time.Time{}, fmt.Errorf("%s: %w: %v", v.Name, ErrFailed, err)
		}
		if v.Clock.Sleep(ctx, v.Timing.ResetBackoff[attempt]) != nil {
			v.stop(ctx)
			return time.Time{}, ErrDrained
		}
	}
}

func (v *VM) resetOnce(ctx context.Context) (time.Time, error) {
	if err := v.Incus.Stop(ctx, v.Name); err != nil {
		return time.Time{}, err
	}
	golden, err := v.Incus.NewestGolden(ctx, v.Name)
	if err != nil {
		return time.Time{}, err
	}
	if err := v.Incus.Restore(ctx, v.Name, golden); err != nil {
		return time.Time{}, err
	}
	if err := v.Incus.Start(ctx, v.Name); err != nil {
		return time.Time{}, err
	}
	started := v.Clock.Now()
	v.golden = golden
	return started, v.waitReady(ctx)
}

// waitReady waits for the incus agent and Docker to answer inside the VM.
func (v *VM) waitReady(ctx context.Context) error {
	deadline := v.Clock.Now().Add(v.Timing.Ready)
	for {
		err := v.exec(ctx, nil, "docker", "info")
		if err == nil {
			return nil
		}
		if !v.Clock.Now().Before(deadline) {
			return fmt.Errorf("docker not ready within %v: %w", v.Timing.Ready, err)
		}
		if err := v.Clock.Sleep(ctx, v.Timing.ReadyPoll); err != nil {
			return err
		}
	}
}

// mint registers a JIT runner, retrying with backoff until ctx ends.
func (v *VM) mint(ctx context.Context) (runners.Runner, string, error) {
	backoff := v.Timing.MintBackoffStart
	for {
		name := v.Name + "-" + v.Clock.Now().UTC().Format(nameTime)
		r, config, err := v.GitHub.MintJIT(ctx, name, v.GroupID, v.Labels)
		if err == nil {
			return r, config, nil
		}
		if ctx.Err() != nil {
			return runners.Runner{}, "", ctx.Err()
		}
		v.mintFailures++
		v.publish()
		v.log().Warn("mint failed", "name", name, "err", err)
		if err := v.Clock.Sleep(ctx, backoff); err != nil {
			return runners.Runner{}, "", err
		}
		backoff = min(2*backoff, v.Timing.MintBackoffMax)
	}
}

// watch polls GitHub and Incus until the runner's job has ended.
func (v *VM) watch(ctx context.Context, id int64, started time.Time) error {
	handedOff := v.Clock.Now()
	online, busy, ran, draining := false, false, false, false
	for {
		if err := v.Clock.Sleep(ctx, v.Timing.Poll); err != nil && !draining {
			if !busy {
				v.finish(ctx, id)
				return ErrDrained
			}
			v.log().Info("draining after the running job")
			draining = true
			ctx = context.WithoutCancel(ctx)
		}
		v.publish()
		now := v.Clock.Now()
		if now.Sub(started) >= v.Timing.Lifetime {
			v.log().Warn("lifetime cap reached, killing the job", "runner", id)
			break
		}
		if !online && now.Sub(handedOff) >= v.Timing.Online {
			v.log().Warn("runner never came online", "runner", id)
			break
		}
		up, err := v.Incus.Running(ctx, v.Name)
		if err != nil {
			v.log().Warn("VM state lookup failed", "err", err)
			continue
		}
		if !up {
			v.log().Info("VM is not running", "runner", id)
			break
		}
		r, err := v.GitHub.Get(ctx, id)
		if errors.Is(err, runners.ErrNotFound) {
			ran = true
			break
		}
		if err != nil {
			v.log().Warn("runner lookup failed", "runner", id, "err", err)
			continue
		}
		if r.Busy {
			online, busy, ran = true, true, true
			v.setState(Busy)
			continue
		}
		if busy {
			break
		}
		if r.Status != "online" {
			if online {
				break
			}
			continue
		}
		online = true
		v.setState(Idle)
	}
	if ran {
		v.jobs++
	}
	v.finish(ctx, id)
	if draining {
		return ErrDrained
	}
	return nil
}

// exec runs command in the VM, bounded so a hung guest can't hold the loop.
func (v *VM) exec(ctx context.Context, stdin io.Reader, command ...string) error {
	ctx, cancel := context.WithTimeout(ctx, v.Timing.Exec)
	defer cancel()
	return v.Incus.Exec(ctx, v.Name, stdin, command...)
}

// finish deregisters the runner and stops the VM, even while draining.
func (v *VM) finish(ctx context.Context, id int64) {
	ctx = context.WithoutCancel(ctx)
	v.setState(Resetting)
	if err := v.GitHub.Delete(ctx, id); err != nil {
		v.log().Warn("delete failed, retrying next cycle", "runner", id, "err", err)
		v.undeleted = append(v.undeleted, id)
	}
	v.stop(ctx)
}

func (v *VM) retryDeletes(ctx context.Context) {
	var left []int64
	for _, id := range v.undeleted {
		if err := v.GitHub.Delete(ctx, id); err != nil {
			v.log().Warn("delete failed again", "runner", id, "err", err)
			left = append(left, id)
		}
	}
	v.undeleted = left
}

// stop stops the VM, ignoring ctx's cancellation; failures are logged.
func (v *VM) stop(ctx context.Context) {
	if err := v.Incus.Stop(context.WithoutCancel(ctx), v.Name); err != nil {
		v.log().Error("stop failed", "err", err)
	}
}

func (v *VM) setState(s State) {
	if s != v.state || v.since.IsZero() {
		v.state = s
		v.since = v.Clock.Now()
	}
	v.publish()
}

func (v *VM) log() *slog.Logger {
	if v.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return v.Log.With("vm", v.Name)
}

// publish writes the VM's metrics for node_exporter's textfile collector.
func (v *VM) publish() {
	if v.MetricsDir == "" {
		return
	}
	path := filepath.Join(v.MetricsDir, "docker_runner_"+v.Name+".prom")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(v.metrics()), 0o644); err != nil {
		v.log().Error("writing metrics failed", "err", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		v.log().Error("writing metrics failed", "err", err)
	}
}

func (v *VM) metrics() string {
	var b strings.Builder
	vm := fmt.Sprintf(`vm=%q`, v.Name)
	b.WriteString("# TYPE docker_runner_state gauge\n")
	for s, name := range stateNames {
		on := 0
		if State(s) == v.state {
			on = 1
		}
		fmt.Fprintf(&b, "docker_runner_state{%s,state=%q} %d\n", vm, name, on)
	}
	fmt.Fprintf(&b, "# TYPE docker_runner_state_since_seconds gauge\n")
	fmt.Fprintf(&b, "docker_runner_state_since_seconds{%s} %d\n", vm, v.since.Unix())
	fmt.Fprintf(&b, "# TYPE docker_runner_resets_total counter\n")
	fmt.Fprintf(&b, "docker_runner_resets_total{%s,result=\"ok\"} %d\n", vm, v.resetsOK)
	fmt.Fprintf(&b, "docker_runner_resets_total{%s,result=\"failed\"} %d\n", vm, v.resetsFailed)
	fmt.Fprintf(&b, "# TYPE docker_runner_jobs_total counter\n")
	fmt.Fprintf(&b, "docker_runner_jobs_total{%s} %d\n", vm, v.jobs)
	fmt.Fprintf(&b, "# TYPE docker_runner_mint_failures_total counter\n")
	fmt.Fprintf(&b, "docker_runner_mint_failures_total{%s} %d\n", vm, v.mintFailures)
	if t, err := time.Parse(nameTime, strings.TrimPrefix(v.golden, goldenPrefix)); err == nil {
		fmt.Fprintf(&b, "# TYPE docker_runner_golden_timestamp_seconds gauge\n")
		fmt.Fprintf(&b, "docker_runner_golden_timestamp_seconds{%s} %d\n", vm, t.Unix())
	}
	fmt.Fprintf(&b, "# TYPE docker_runner_heartbeat_timestamp_seconds gauge\n")
	fmt.Fprintf(&b, "docker_runner_heartbeat_timestamp_seconds{%s} %d\n", vm, v.Clock.Now().Unix())
	return b.String()
}
