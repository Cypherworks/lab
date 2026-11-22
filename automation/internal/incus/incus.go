// Package incus drives instances through the incus CLI, scoped to one remote
// and one project, so a restricted client certificate is all it needs.
package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// goldenPrefix marks restorable snapshots; a UTC timestamp sorts the rest.
const goldenPrefix = "golden-"

// Runner runs the incus CLI with args and stdin and returns its stdout.
type Runner func(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error)

// Client acts on instances in Project on Remote.
type Client struct {
	Remote  string
	Project string
	Run     Runner
}

// CLI runs the incus binary at path; a failure carries the command and stderr.
func CLI(path string) Runner {
	return func(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, path, args...)
		cmd.Stdin = stdin
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("incus %s: %w: %s", strings.Join(args, " "), err,
				strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}
}

// Stop force-stops vm if it's running.
func (c *Client) Stop(ctx context.Context, vm string) error {
	up, err := c.Running(ctx, vm)
	if err != nil || !up {
		return err
	}
	return c.run(ctx, "stop", c.target(vm), "--force")
}

// Start boots vm.
func (c *Client) Start(ctx context.Context, vm string) error {
	return c.run(ctx, "start", c.target(vm))
}

// Restore rolls vm's disk back to snapshot. The instance config stays as
// Terraform last set it, so a restore never reverts a later apply.
func (c *Client) Restore(ctx context.Context, vm, snapshot string) error {
	return c.run(ctx, "snapshot", "restore", c.target(vm), snapshot, "--diskonly")
}

// NewestGolden names vm's latest golden-<UTC timestamp> snapshot.
func (c *Client) NewestGolden(ctx context.Context, vm string) (string, error) {
	out, err := c.Run(ctx, nil, "snapshot", "list", c.target(vm), "--format", "json",
		"--project", c.Project)
	if err != nil {
		return "", err
	}
	var snaps []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &snaps); err != nil {
		return "", fmt.Errorf("snapshots of %s: %w", vm, err)
	}
	newest := ""
	for _, s := range snaps {
		if strings.HasPrefix(s.Name, goldenPrefix) && s.Name > newest {
			newest = s.Name
		}
	}
	if newest == "" {
		return "", fmt.Errorf("%s has no %s* snapshot", vm, goldenPrefix)
	}
	return newest, nil
}

// Running reports whether Incus has vm running.
func (c *Client) Running(ctx context.Context, vm string) (bool, error) {
	path := c.Remote + ":/1.0/instances/" + vm + "/state?project=" + c.Project
	out, err := c.Run(ctx, nil, "query", path)
	if err != nil {
		return false, err
	}
	var state struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &state); err != nil {
		return false, fmt.Errorf("state of %s: %w", vm, err)
	}
	if state.Status == "" {
		return false, errors.New("state of " + vm + " has no status")
	}
	return state.Status == "Running", nil
}

// Exec runs command in vm as root, with stdin if given.
func (c *Client) Exec(ctx context.Context, vm string, stdin io.Reader, command ...string) error {
	args := []string{"exec", c.target(vm), "--project", c.Project}
	if stdin == nil {
		args = append(args, "--disable-stdin")
	}
	args = append(append(args, "--"), command...)
	_, err := c.Run(ctx, stdin, args...)
	return err
}

func (c *Client) target(vm string) string {
	return c.Remote + ":" + vm
}

// run is Run for stdin-less subcommands, with the project flag last.
func (c *Client) run(ctx context.Context, args ...string) error {
	_, err := c.Run(ctx, nil, append(args, "--project", c.Project)...)
	return err
}
