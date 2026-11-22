// Package runners manages an org's self-hosted runners over the GitHub REST
// API: it mints single-job JIT configs, reads a runner's state, lists the
// org's runners and deletes one. Errors carry the method, path and HTTP
// status, never a response body, since a mint's body is a credential.
package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// apiVersion is the GitHub REST API version the client speaks.
const apiVersion = "2022-11-28"

// maxBody bounds every GitHub response read.
const maxBody = 1 << 20

// perPage is GitHub's largest page size for the runner listings.
const perPage = 100

// ErrNotFound means GitHub has no runner with that id.
var ErrNotFound = errors.New("runner not found")

// Client talks to the runner endpoints of one org with a bearer token.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Token   string
	Org     string
}

// Runner is the part of GitHub's runner record the supervisor acts on.
type Runner struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Busy   bool   `json:"busy"`
}

// MintJIT registers a single-job runner and returns its encoded JIT config.
func (c *Client) MintJIT(ctx context.Context, name string, groupID int64, labels []string) (Runner, string, error) {
	body := map[string]any{
		"name":            name,
		"runner_group_id": groupID,
		"labels":          labels,
	}
	var resp struct {
		Runner Runner `json:"runner"`
		Config string `json:"encoded_jit_config"`
	}
	path := "/runners/generate-jitconfig"
	if err := c.expect(ctx, http.MethodPost, path, body, http.StatusCreated, &resp); err != nil {
		return Runner{}, "", err
	}
	if resp.Config == "" {
		return Runner{}, "", fmt.Errorf("POST %s: no encoded_jit_config", path)
	}
	return resp.Runner, resp.Config, nil
}

// Get reads one runner; ErrNotFound once GitHub has removed it.
func (c *Client) Get(ctx context.Context, id int64) (Runner, error) {
	var r Runner
	path := fmt.Sprintf("/runners/%d", id)
	status, err := c.do(ctx, http.MethodGet, path, nil, &r)
	switch {
	case err != nil:
		return Runner{}, err
	case status == http.StatusNotFound:
		return Runner{}, ErrNotFound
	case status != http.StatusOK:
		return Runner{}, fmt.Errorf("GET %s: HTTP %d", path, status)
	}
	return r, nil
}

// Delete removes a runner. One that's already gone counts as removed.
func (c *Client) Delete(ctx context.Context, id int64) error {
	path := fmt.Sprintf("/runners/%d", id)
	status, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("DELETE %s: HTTP %d", path, status)
	}
	return nil
}

// DefaultGroupID finds the runner group GitHub marks as the default.
func (c *Client) DefaultGroupID(ctx context.Context) (int64, error) {
	var resp struct {
		Groups []struct {
			ID      int64 `json:"id"`
			Default bool  `json:"default"`
		} `json:"runner_groups"`
	}
	path := fmt.Sprintf("/runner-groups?per_page=%d", perPage)
	if err := c.expect(ctx, http.MethodGet, path, nil, http.StatusOK, &resp); err != nil {
		return 0, err
	}
	for _, g := range resp.Groups {
		if g.Default {
			return g.ID, nil
		}
	}
	return 0, errors.New("no runner group is marked default")
}

// List reads every runner registered in the org, a page at a time.
func (c *Client) List(ctx context.Context) ([]Runner, error) {
	var all []Runner
	for page := 1; ; page++ {
		var resp struct {
			Runners []Runner `json:"runners"`
		}
		path := fmt.Sprintf("/runners?per_page=%d&page=%d", perPage, page)
		if err := c.expect(ctx, http.MethodGet, path, nil, http.StatusOK, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Runners...)
		if len(resp.Runners) < perPage {
			return all, nil
		}
	}
}

// expect is do for calls with a single successful status.
func (c *Client) expect(ctx context.Context, method, path string, in any, want int, out any) error {
	status, err := c.do(ctx, method, path, in, out)
	if err != nil {
		return err
	}
	if status != want {
		return fmt.Errorf("%s %s: HTTP %d", method, path, status)
	}
	return nil
}

// do sends one request and decodes a 2xx body into out, if out is set.
func (c *Client) do(ctx context.Context, method, path string, in any, out any) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}
	url := c.BaseURL + "/orgs/" + c.Org + "/actions" + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, fmt.Errorf("build %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if out == nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, nil
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out)
	if err != nil {
		return 0, fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return resp.StatusCode, nil
}
